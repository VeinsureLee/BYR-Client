package desktop

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/VeinsureLee/BYR-Client/apps/desktop/internal/bootstrap"
	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Explicit opt-in native backend regression using a local fixture only.
func captureNativeWindows(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := `{"code":"0000"}`
		switch r.URL.Path {
		case "/open/section.json":
			body = `{"section":[{"name":0,"description":"校园"}]}`
		case "/open/section/0.json":
			body = `{"name":"0","description":"校园","sub_section":["1"],"board":[{"name":"Go","description":"Go 讨论","manager":["fixture"]}]}`
		case "/open/section/1.json":
			body = `{"name":"1","description":"院系","parent":"0","board":[{"name":"CS","description":"计算机学院"}]}`
		case "/open/favorite/0.json":
			body = `{"section":[],"board":[{"name":"Go","description":"Go 讨论"}]}`
		case "/open/widget/topten.json":
			body = `{"article":[{"id":42,"board_name":"Go","title":"原生窗口测试主题","user":{"id":"fixture","level":"用户"}}]}`
		case "/open/favorpost.json":
			body = `{"article":[{"id":42,"group_id":42,"board_name":"Go","title":"原生时间线主题","user":{"id":"fixture","level":"用户"},"post_time":100}],"pagination":{"page_all_count":1,"page_current_count":1}}`
		case "/open/user/getinfo.json":
			body = `{"id":"native-test","user_name":"原生窗口测试","level":"用户","post_count":12}`
		case "/open/mail/inbox.json", "/open/mail/outbox.json":
			body = `{"mail":[{"index":1,"title":"测试私信","user":{"id":"bob"}}]}`
		}
		io.WriteString(w, body)
	}))
	defer server.Close()
	config := bootstrap.Config{BaseURL: server.URL, Credentials: &testCredentials{session: application.Credential{Username: "native-test", Token: "local-fixture"}}}
	pages := []string{"home", "top", "timeline", "favorites", "compose", "search", "contacts", "self", "settings"}
	services, err := bootstrap.New(config)
	if err != nil {
		return err
	}
	var captureErr error
	mygo.App.WhenReady(func() {
		var window *mygo.Window
		a := New(services.App, &testDevice{}, func(fn func()) { window.Update(fn) })
		var renderedPage string
		var renderedBusy bool
		window = mygo.NewWindow(mygo.WindowOptions{Title: "BYR Native Regression", Width: 1100, Height: 760, Hidden: true, Content: ui.View(func(c *ui.Context) { a.View(c); renderedPage = string(a.route.Page); renderedBusy = a.busy })})
		window.OnClosed(a.Close)
		window.ShowInactive()
		a.Start()
		go func() {
			defer mygo.App.Quit()
			defer a.close()
			for _, page := range pages {
				ready := false
				deadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					mygo.RunOnMain(func() { ready = !a.busy })
					if ready {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if !ready {
					captureErr = fmt.Errorf("native startup timeout")
					return
				}
				if os.Getenv("BYR_NATIVE_CLICK") == "1" {
					mygo.RunOnMain(func() { captureErr = nativeSidebarClick(window, page) })
					if captureErr != nil {
						return
					}
				} else {
					window.Update(func() { a.navigate(Route{Page: Page(page)}, false) })
				}
				// A main-thread barrier ensures the route change has been applied.
				mygo.RunOnMain(func() {})
				deadline = time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					mygo.RunOnMain(func() { ready = !a.busy })
					if ready {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if !ready {
					captureErr = fmt.Errorf("native %s timeout", page)
					return
				}
				deadline = time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					mygo.RunOnMain(func() { ready = renderedPage == page && !renderedBusy })
					if ready {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if !ready {
					captureErr = fmt.Errorf("native %s never rendered (last page %s busy %t)", page, renderedPage, renderedBusy)
					return
				}
				mygo.RunOnMain(func() {
					if a.failed || a.pageError != "" {
						captureErr = fmt.Errorf("native %s failed: %s", page, a.diagnostic)
						return
					}
					if page == "home" && (len(a.sections) == 0 || len(a.boards) == 0) {
						captureErr = fmt.Errorf("native home had no loaded content")
					}
					if page == "top" && len(a.articles) == 0 {
						captureErr = fmt.Errorf("native top-ten had no loaded content")
					}
				})
				if captureErr != nil {
					return
				}
				b, err := window.CapturePage()
				if err != nil {
					captureErr = err
					return
				}
				if err = os.WriteFile(filepath.Join(dir, page+".png"), b, 0600); err != nil {
					captureErr = err
					return
				}
				mygo.RunOnMain(func() {
					fmt.Printf("native page=%s route=%s sections=%d articles=%d status=%q\n", page, a.route.Page, len(a.sections), len(a.articles), a.message)
				})
			}
		}()
	})
	if err = mygo.App.Run(); err != nil {
		return err
	}
	return captureErr
}
