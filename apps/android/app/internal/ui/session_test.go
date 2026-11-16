package ui

import (
	"context"
	"errors"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/apps/android/app/internal/platform"
	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestCurrentRouteRepairsEmptyStack(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.stack = nil
	if got := a.current().Page; got != PageLogin {
		t.Fatalf("current page = %v, want login", got)
	}
	if len(a.stack) != 1 {
		t.Fatalf("route stack was not repaired: %+v", a.stack)
	}
}

func TestMissingCredentialsStartAtLogin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BYR_DATA_DIR", dir)
	a := startAppForTest(t)
	defer a.Close()
	if !a.ready || a.initErr != "" {
		t.Fatalf("app did not become ready: ready=%v initErr=%q", a.ready, a.initErr)
	}
	if got := a.current().Page; got != PageLogin {
		t.Fatalf("current page = %v, want login", got)
	}
	assertFrameRenders(t, a)
}

func TestExpiredCredentialsStartAtLogin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BYR_DATA_DIR", dir)
	credentials := platform.FileCredentials{Path: filepath.Join(dir, "session.json")}
	if err := credentials.Save(context.Background(), application.Credential{
		Username:  "alice",
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	a := startAppForTest(t)
	defer a.Close()
	if got := a.current().Page; got != PageLogin {
		t.Fatalf("current page = %v, want login", got)
	}
	if _, err := os.Stat(credentials.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired credential file still exists: %v", err)
	}
	assertFrameRenders(t, a)
}

func TestLogoutThenNewAppStartsAtLogin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BYR_DATA_DIR", dir)
	credentials := platform.FileCredentials{Path: filepath.Join(dir, "session.json")}
	if err := credentials.Save(context.Background(), application.Credential{
		Username:  "alice",
		Token:     "remembered-token",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	store, err := local.Open(context.Background(), platform.FileState{Path: filepath.Join(dir, "state.json")})
	if err != nil {
		t.Fatal(err)
	}
	core, err := application.New(application.Options{Local: store, Credentials: credentials})
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if core.Session().Token == "" {
		t.Fatal("fixture session was not restored")
	}

	first, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	first.started = true
	first.ready = true
	first.core = core
	first.dir = dir
	first.stack = []Route{{Page: PageSelf}}
	first.username.SetText("alice")
	first.logout()
	waitForIdle(t, first)
	if got := first.current().Page; got != PageLogin {
		t.Fatalf("logout page = %v, want login", got)
	}
	if core.Session().Token != "" {
		t.Fatal("logout retained the in-memory token")
	}
	if _, err := os.Stat(credentials.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("logout retained credential file: %v", err)
	}
	first.Close()

	second := startAppForTest(t)
	defer second.Close()
	if got := second.current().Page; got != PageLogin {
		t.Fatalf("restarted page = %v, want login", got)
	}
	if !second.ready || second.initErr != "" {
		t.Fatalf("restarted app is not usable: ready=%v initErr=%q", second.ready, second.initErr)
	}
}

func TestRestartWithRememberedSessionRendersSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/section.json":
			_, _ = w.Write([]byte(`{"section":[{"name":"0","description":"校园"}]}`))
		case "/open/section/0.json":
			_, _ = w.Write([]byte(`{"name":"0","description":"校园","board":[]}`))
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	t.Setenv("BYR_DATA_DIR", dir)
	t.Setenv("BYR_BASE_URL", server.URL)
	credentials := platform.FileCredentials{Path: filepath.Join(dir, "session.json")}
	if err := credentials.Save(context.Background(), application.Credential{
		Username: "alice", Token: "remembered-token", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	a := startAppForTest(t)
	defer a.Close()
	waitForIdle(t, a)
	if got := a.current().Page; got != PageSearch {
		t.Fatalf("restart page = %v, want search", got)
	}
	assertFrameRenders(t, a)
	a.setTab(PageHome)
	waitForIdle(t, a)
	if got := a.current().Page; got != PageHome || a.sectionTab != "favorites" {
		t.Fatalf("section tab default = %+v/%q, want favorite boards", a.current(), a.sectionTab)
	}
	a.showAllSections()
	waitForIdle(t, a)
	if got := a.current().Page; got != PageHome || a.sectionTab != "all" {
		t.Fatalf("all sections page = %+v/%q", a.current(), a.sectionTab)
	}
	if len(a.sections) != 1 {
		t.Fatalf("all sections did not load: %+v", a.sections)
	}
}

func TestLogoutThenLoginAgainRenders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth2/official":
			_, _ = w.Write([]byte(`{"access_token":"token","expires_in":3600}`))
		case "/open/section.json":
			_, _ = w.Write([]byte(`{"section":[{"name":"0","description":"校园"}]}`))
		case "/open/section/0.json":
			_, _ = w.Write([]byte(`{"name":"0","description":"校园","board":[{"name":"Go","description":"Go 讨论"}]}`))
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	defer server.Close()
	t.Setenv("BYR_DATA_DIR", t.TempDir())
	t.Setenv("BYR_BASE_URL", server.URL)

	a := startAppForTest(t)
	defer a.Close()
	a.username.SetText("alice")
	a.password.SetText("pw")
	a.login()
	waitForIdle(t, a)
	if got := a.current().Page; got != PageSearch {
		t.Fatalf("login page = %v, want search", got)
	}
	assertFrameRenders(t, a)

	prefs := a.core.Local.Preferences()
	prefs.Theme = 2
	if err := a.core.Local.SetPreferences(context.Background(), prefs); err != nil {
		t.Fatal(err)
	}
	defer func() {
		setPalette(false)
		a.dark, a.themed = false, false
		a.theme.Bg, a.theme.Fg = colorBg, colorFg
	}()

	a.logout()
	waitForIdle(t, a)
	if got := a.current().Page; got != PageLogin {
		t.Fatalf("logout page = %v, want login", got)
	}
	assertFrameRenders(t, a)

	a.username.SetText("alice")
	a.password.SetText("pw")
	a.login()
	waitForIdle(t, a)
	if got := a.current().Page; got != PageSearch {
		t.Fatalf("second login page = %v, want search", got)
	}
	assertFrameRenders(t, a)
}

func assertFrameRenders(t *testing.T, a *App) {
	t.Helper()
	var router input.Router
	var ops op.Ops
	dims := a.Layout(layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 760)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("frame rendered blank: %+v", dims.Size)
	}
	if a.renderPanic != "" {
		t.Fatalf("frame hit a render panic: %q", a.renderPanic)
	}
}

func startAppForTest(t *testing.T) *App {
	t.Helper()
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.start()
	a.drain()
	return a
}

func waitForIdle(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		a.drain()
		if !a.busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("app remained busy: message=%q", a.message)
}
