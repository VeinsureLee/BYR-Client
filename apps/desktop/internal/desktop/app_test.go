package desktop

import (
	"context"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/apps/desktop/internal/bootstrap"
	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
	"github.com/VeinsureLee/BYR-Client/core/resources/fonts"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if err := ui.RegisterFont(fonts.Regular, fonts.Family); err != nil {
		panic(err)
	}
	if os.Getenv("BYR_DIAGNOSE_FIXTURE") == "1" {
		if err := diagnoseFixture(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if dir := os.Getenv("BYR_NATIVE_CAPTURE_DIR"); dir != "" {
		if err := captureNativeWindows(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type testCredentials struct{ session application.Credential }

func (s *testCredentials) Load(context.Context) (application.Credential, error) {
	return s.session, nil
}
func (s *testCredentials) Save(_ context.Context, c application.Credential) error {
	s.session = c
	return nil
}
func (s *testCredentials) Clear(context.Context) error {
	s.session = application.Credential{}
	return nil
}

type testDevice struct{ copied, opened string }

func (d *testDevice) Copy(v string) error { d.copied = v; return nil }
func (d *testDevice) Open(v string) error { d.opened = v; return nil }
func (*testDevice) Pick(context.Context) (string, []byte, error) {
	return "test.txt", []byte("file content"), nil
}
func (*testDevice) Save(string, []byte) error { return nil }

type recordedForm struct {
	path   string
	values url.Values
}

type harness struct {
	app       *App
	view      *ui.Tester
	updates   chan func()
	mu        sync.Mutex
	requests  []string
	queries   []string
	forms     []recordedForm
	authForms []recordedForm
	t         *testing.T
	device    *testDevice
	overrides map[string]string
	gates     map[string]chan struct{}
	// searchBodies overrides /open/search/article.json per board, so tests can
	// make one board succeed while another fails.
	searchBodies map[string]string
}

func newHarness(t *testing.T) *harness { return newHarnessWithStorage(t, nil) }

func newHarnessWithStorage(t *testing.T, storage local.Backend) *harness {
	t.Helper()
	h := &harness{updates: make(chan func(), 256), t: t, device: &testDevice{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.requests = append(h.requests, r.Method+" "+r.URL.Path)
		h.queries = append(h.queries, r.URL.RawQuery)
		h.mu.Unlock()
		if r.Method == http.MethodPost && (r.URL.Path == "/open/mail/send.json" || r.URL.Path == "/oauth2/official") {
			if err := r.ParseForm(); err != nil {
				h.t.Errorf("parse form for %s: %v", r.URL.Path, err)
			}
			values := make(url.Values, len(r.PostForm))
			for key, value := range r.PostForm {
				values[key] = append([]string(nil), value...)
			}
			form := recordedForm{path: r.URL.Path, values: values}
			h.mu.Lock()
			if r.URL.Path == "/oauth2/official" {
				h.authForms = append(h.authForms, form)
			} else {
				h.forms = append(h.forms, form)
			}
			h.mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		body := `{"code":"0000"}`
		switch r.URL.Path {
		case "/oauth2/official":
			body = `{"access_token":"fixture-token","expires_in":3600}`
		case "/open/section.json":
			body = `{"section":[{"name":0,"description":"校园"}]}`
		case "/open/section/0.json":
			body = `{"name":"0","description":"校园","sub_section":[{"name":"1","description":"院系"}],"board":[{"name":"Go","description":"Go 讨论"}]}`
		case "/open/section/1.json":
			body = `{"name":"1","description":"院系","parent":"0","board":[{"name":"CS","description":"计算机学院"}]}`
		case "/open/board/Go.json":
			body = `{"name":"Go","description":"Go 讨论","allow_post":1,"allow_attachment":1,"allow_anonymous":0,"article":[{"id":42,"title":"测试主题","user":{"id":"alice"}}],"pagination":{"page_all_count":1,"page_current_count":1}}`
		case "/open/threads/Go/42.json", "/open/threads/Go/43.json":
			body = `{"id":42,"group_id":42,"title":"测试主题","board_name":"Go","board_description":"Go 讨论","user":{"id":"alice","level":"用户"},"reply_count":1,"like_sum":2,"is_liked":0,"votedown_sum":0,"collect":0,"article":[{"id":42,"position":1,"title":"测试主题","content":"你好 [b]北邮[/b]","user":{"id":"alice","level":"用户"},"like_sum":2,"is_liked":0,"votedown_sum":0,"is_votedown":0},{"id":44,"position":2,"reply_id":42,"content":"回复正文","user":{"id":"bob","level":"用户"},"like_sum":0,"is_liked":0,"votedown_sum":5,"is_votedown":0}],"pagination":{"page_all_count":1,"page_current_count":1}}`
		case "/open/article/Go/42.json":
			body = `{"id":42,"position":1,"title":"测试主题","content":"你好 [b]北邮[/b]","user":{"id":"alice","level":"用户"}}`
		case "/open/article/Go/44.json":
			body = `{"id":44,"position":2,"reply_id":42,"content":"回复正文","user":{"id":"bob","level":"用户"}}`
		case "/open/article/Go/like/42.json":
			body = `{"like_sum":3,"votedown_sum":0}`
		case "/open/article/Go/votedown/42.json":
			body = `{"like_sum":2,"votedown_sum":1}`
		case "/open/article/Go/like/44.json":
			body = `{"like_sum":1,"votedown_sum":5}`
		case "/open/article/Go/votedown/44.json":
			body = `{"like_sum":0,"votedown_sum":6}`
		case "/open/collection/add.json", "/open/collection/delete.json":
			body = `{}`
		case "/open/article/Go/post.json", "/open/article/Go/update/42.json":
			r.ParseForm()
			if !strings.Contains(r.PostForm.Get("content"), "正文") {
				t.Error("post body lost")
			}
			if r.PostForm.Has("reid") && r.PostForm.Get("reid") == "" {
				t.Error("empty reid")
			}
			body = `{"id":43,"group_id":42,"board_name":"Go"}`
		case "/open/attachment/Go/add/.json":
			body = `{"file":[{"name":"test.txt","url":"/api/attachment/Go/99"}]}`
		case "/open/widget/topten.json", "/open/search/article.json":
			body = `{"article":[{"id":42,"board_name":"Go","title":"测试主题","user":{"id":"alice","level":"用户"}}]}`
		case "/open/search/board.json":
			body = `{"section":[],"board":[{"name":"Go","description":"Go 讨论"}]}`
		case "/open/favorpost.json":
			body = `{"article":[{"id":42,"group_id":42,"board_name":"Go","title":"时间线旧帖","user":{"id":"alice","level":"用户"},"post_time":100,"reply_count":2},{"id":44,"group_id":44,"board_name":"Go","title":"时间线新帖","user":{"id":"bob","level":"用户"},"post_time":200,"reply_count":1}],"pagination":{"page_all_count":1,"page_current_count":1,"item_all_count":2}}`
		case "/open/favorite/0.json":
			body = `{"section":[],"board":[{"name":"Go","description":"Go 讨论"}]}`
		case "/open/collection.json":
			body = `{"article":[{"gid":42,"bname":"Go","title":"收藏主题","num":3}]}`
		case "/open/user/getinfo.json", "/open/user/query/bob.json":
			body = `{"id":"bob","user_name":"Bob","level":"用户","post_count":"12","score":100}`
		case "/open/mail/inbox.json":
			body = `{"mail":[{"index":1,"title":"来信标题","user":{"id":"bob"},"post_time":2}]}`
		case "/open/mail/outbox.json":
			body = `{"mail":[{"index":2,"title":"已发标题","user":{"id":"bob"},"post_time":1}]}`
		case "/open/mail/inbox/1.json":
			body = `{"index":1,"title":"来信标题","content":"收到的完整信件正文","user":{"id":"bob"},"post_time":2}`
		case "/open/mail/outbox/2.json":
			body = `{"index":2,"title":"已发标题","content":"发出的完整信件正文","user":{"id":"bob"},"post_time":1}`
		case "/open/refer/reply.json", "/open/refer/at.json":
			body = `{"article":[{"index":1,"id":44,"group_id":42,"board_name":"Go","title":"通知主题","user":{"id":"bob"}}]}`
		case "/open/blacklist/list.json":
			body = `{"user":[{"id":"bob","user_name":"Bob"}]}`
		case "/open/vote/42.json":
			body = `{"title":"测试投票","options":[{"viid":1,"label":"选项甲","num":2}]}`
		case "/open/bet/42.json":
			body = `{"title":"测试竞猜","bitems":[{"biid":1,"label":"竞猜甲","percent":"0.5"}],"score":100}`
		}
		h.mu.Lock()
		override, ok := h.overrides[r.URL.Path]
		gate := h.gates[r.URL.Path]
		boardBody, boardOK := h.searchBodies[r.URL.Query().Get("board")]
		h.mu.Unlock()
		if ok {
			body = override
		}
		if r.URL.Path == "/open/search/article.json" && boardOK {
			body = boardBody
		}
		if gate != nil {
			<-gate
		}
		io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	services, err := bootstrap.New(bootstrap.Config{BaseURL: server.URL, Credentials: &testCredentials{}, Storage: storage})
	if err != nil {
		t.Fatal(err)
	}
	h.app = New(services.App, h.device, func(fn func()) { h.updates <- fn })
	h.view = ui.NewTester(h.app.View, 1200, 900)
	t.Cleanup(h.app.Close)
	return h
}
func (h *harness) waitFor(cond func() bool) {
	h.t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for !cond() {
		select {
		case fn := <-h.updates:
			fn()
			h.view.Frame()
		case <-deadline.C:
			h.t.Fatalf("condition not reached: route=%s busy=%t pageLoading=%t extrasBusy=%t message=%q", h.app.route.Page, h.app.busy, h.app.pageLoading, h.app.threadResourcesBusy, h.app.message)
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func (h *harness) idle() {
	h.t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		if !h.app.busy && !h.app.threadResourcesBusy {
			for {
				select {
				case fn := <-h.updates:
					fn()
					h.view.Frame()
				default:
					return
				}
			}
		}
		select {
		case fn := <-h.updates:
			fn()
			h.view.Frame()
		case <-deadline.C:
			h.t.Fatalf("async timeout page=%s message=%s", h.app.route.Page, h.app.message)
		}
	}
}
func (h *harness) click(label string) {
	h.t.Helper()
	if err := h.view.Click(label); err != nil {
		h.t.Fatalf("click %q: %v; texts %q", label, err, h.view.Texts())
	}
	h.idle()
}
func (h *harness) typeIn(label, text string) {
	h.t.Helper()
	if err := h.view.Click(label); err != nil {
		h.t.Fatal(err)
	}
	h.view.Type(text)
	h.idle()
}
func (h *harness) login() {
	h.t.Helper()
	h.typeIn("用户名", "alice")
	h.typeIn("密码", "pw")
	h.click("登录")
	if h.app.route.Page != "home" {
		h.t.Fatalf("login failed: %s", h.app.message)
	}
}
func (h *harness) goTo(r Route) {
	h.t.Helper()
	h.app.navigate(r, true)
	h.idle()
	h.view.Frame()
	if h.app.failed {
		h.t.Fatal(h.app.message)
	}
}
func (h *harness) has(text string) {
	h.t.Helper()
	if !h.view.HasText(text) {
		h.t.Fatalf("missing %q: %q", text, h.view.Texts())
	}
}

func countText(texts []string, query string) int {
	n := 0
	for _, text := range texts {
		if strings.Contains(text, query) {
			n++
		}
	}
	// MyGo reports a control's accessible label and its rendered text.
	return (n + 1) / 2
}

func (h *harness) query(methodPath string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.requests) - 1; i >= 0; i-- {
		if h.requests[i] == methodPath {
			return h.queries[i]
		}
	}
	return ""
}

func (h *harness) count(methodPath string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, request := range h.requests {
		if request == methodPath {
			n++
		}
	}
	return n
}
func TestCaptureMigratedScreens(t *testing.T) {
	dir := os.Getenv("BYR_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set BYR_CAPTURE_DIR to inspect offline UI captures")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t)
	capture := func(name string) {
		t.Helper()
		h.view.Frame()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, h.view.Image()); err != nil {
			t.Fatal(err)
		}
	}
	capture("login")
	h.login()
	h.click("校园")
	capture("boards")
	h.click("Go 讨论 · Go")
	h.click("测试主题")
	capture("thread")
	h.goTo(Route{Page: "search"})
	h.typeIn("搜索关键词", "北邮")
	h.click("开始搜索")
	capture("search")
	h.goTo(Route{Page: "contacts"})
	h.app.contacts = nil
	h.app.message = ""
	h.app.failed = false
	capture("messages-empty")
	h.goTo(Route{Page: "thread", Name: "Go", ID: "42", Number: 1})
	h.mu.Lock()
	if h.overrides == nil {
		h.overrides = make(map[string]string)
	}
	h.overrides["/open/threads/Go/42.json"] = `{"invalid":`
	h.mu.Unlock()
	h.click("刷新")
	capture("thread-refresh-error")
	h.mu.Lock()
	delete(h.overrides, "/open/threads/Go/42.json")
	h.mu.Unlock()
	h.app.failed = false
	h.app.message = ""
	h.app.retryLoad = nil
	for _, size := range [][2]int{{720, 480}, {1100, 760}, {1440, 900}} {
		h.view.SetSize(size[0], size[1])
		capture(fmt.Sprintf("thread-%dx%d", size[0], size[1]))
	}
	theme, scale := 2, float32(1)
	if err := h.app.Core.UpdatePreferences(context.Background(), application.PreferencePatch{Theme: &theme, FontScale: &scale}); err != nil {
		t.Fatal(err)
	}
	h.view.SetSize(1100, 760)
	capture("thread-dark")
	theme, scale = 1, 1.3
	if err := h.app.Core.UpdatePreferences(context.Background(), application.PreferencePatch{Theme: &theme, FontScale: &scale}); err != nil {
		t.Fatal(err)
	}
	h.view.SetSize(720, 480)
	capture("thread-large-font")
	scale = 1.4
	if err := h.app.Core.UpdatePreferences(context.Background(), application.PreferencePatch{FontScale: &scale}); err != nil {
		t.Fatal(err)
	}
	h.app.thread.Title = strings.Repeat("测试", 80)
	h.view.Frame()
	capture("thread-long-title-720x480-large-font")
	h.app.desktopListState(h.app.threadListKey()).ScrollTo(2, ui.Start)
	h.view.Frame()
	capture("thread-long-title-720x480-large-font-scrolled")
	h.app.desktopListState(h.app.threadListKey()).ScrollTo(0, ui.Start)
	theme, scale = 1, 1
	if err := h.app.Core.UpdatePreferences(context.Background(), application.PreferencePatch{Theme: &theme, FontScale: &scale}); err != nil {
		t.Fatal(err)
	}
	h.view.SetSize(1200, 900)
	h.view.Frame()
	h.click("回复此楼")
	h.typeIn("帖子正文", "迁移后的发帖编辑器，支持草稿、附件与表情。")
	capture("compose")
	h.goTo(Route{Page: "chat", Name: "bob"})
	capture("chat")
	h.goTo(Route{Page: "favorites", Sub: "collection"})
	capture("favorites")
	h.goTo(Route{Page: "mailbox", Name: "inbox"})
	capture("mailbox")
	h.goTo(Route{Page: "self"})
	capture("self")
	h.goTo(Route{Page: "settings"})
	capture("settings")
	h.click("清除缓存")
	capture("settings-cache-cleared")
	if err := h.app.Core.RecordThreadVisit(context.Background(), h.app.Core.HistoryReadToken(), "Go", "42", "测试主题"); err != nil {
		t.Fatal(err)
	}
	h.goTo(Route{Page: "history"})
	h.click("清空阅读历史")
	capture("history-clear-confirm")
	h.click("确认")
	capture("history-empty")
}
func TestConstructingUIIsOffline(t *testing.T) {
	h := newHarness(t)
	h.has("登录北邮人论坛")
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) != 0 {
		t.Fatal(h.requests)
	}
}

func TestDesktopUserProfileShowsAllFieldsAndOpensHomepage(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "user", Name: "bob"}
	h.app.user = forum.User{
		ID: "bob", UserName: "Bob", Gender: "男", Astro: "双子座", Life: "学生",
		QQ: "12345", MSN: "bob@example", HomePage: "https://example.test/bob", Level: "用户",
		IsOnline: true, PostCount: 12, LastLoginTime: "1700000000", LastLoginIP: "127.0.0.1",
		IsRegister: true, Score: 88, FollowNum: 3, FansNum: 4, IsFollow: true,
	}
	h.view.Frame()
	for _, label := range []string{"在线状态", "注册状态", "生命力", "性别", "星座", "主页", "QQ", "MSN", "上次登录时间", "最后登录 IP", "关注数", "粉丝数", "是否已关注", "是否为粉丝", "发帖数", "积分", "等级"} {
		h.has(label)
	}
	for _, value := range []string{"学生", "12345", "bob@example", "https://example.test/bob"} {
		h.has(value)
	}
	h.click("个人主页")
	if h.device.opened != "https://example.test/bob" {
		t.Fatalf("homepage opened %q, want %q", h.device.opened, "https://example.test/bob")
	}

	h.app.user.IsHide = true
	h.view.Frame()
	if h.view.HasText("上次登录时间") || h.view.HasText("最后登录 IP") {
		t.Fatalf("private login fields leaked: %q", h.view.Texts())
	}

	h.app.user = forum.User{ID: "IWhisper#93", UserName: "匿名", HomePage: "https://example.test/anonymous"}
	h.view.Frame()
	if h.view.HasText("发私信") || h.view.HasText("加入黑名单") {
		t.Fatalf("anonymous user exposed private controls: %q", h.view.Texts())
	}
}

func TestDesktopThreadHasSingleBoardEntryAndNoDuplicateRootActions(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "thread", Name: "Go", ID: "42", Number: 1}
	h.app.thread = forum.Thread{ID: "42", Board: "Go", BoardDescription: "Go 讨论", Title: "主题", Articles: []forum.Article{
		{ID: "42", GroupID: "42", Board: "Go", Title: "主题", Author: forum.User{ID: "alice"}, LikeSum: 2},
		{ID: "44", GroupID: "42", Board: "Go", Author: forum.User{ID: "bob"}},
	}}
	h.view.Frame()
	texts := h.view.Texts()
	if countText(texts, "进入版面") != 0 {
		t.Fatalf("duplicate board action remains: %q", texts)
	}
	if countText(texts, "版面：") != 1 {
		t.Fatalf("board breadcrumb count = %d, texts=%q", countText(texts, "版面："), texts)
	}
	if countText(texts, "回复主题") != 0 {
		t.Fatalf("duplicate root reply action remains: %q", texts)
	}
	if countText(texts, "点赞") != 0 {
		t.Fatalf("duplicate root like action remains: %q", texts)
	}
	if countText(texts, "回复此楼") != 2 {
		t.Fatalf("floor reply actions = %d, texts=%q", countText(texts, "回复此楼"), texts)
	}
}

func TestDesktopThreadUsesCanonicalBoardForLinkAndOneBoardEntry(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "thread", Name: "悄悄话", ID: "42", Number: 1}
	h.app.thread = forum.Thread{ID: "42", Board: "IWhisper", BoardDescription: "悄悄话", Articles: []forum.Article{
		{ID: "42", Board: "IWhisper", Author: forum.User{ID: "alice"}},
	}}
	h.view.Frame()
	h.click("版面：悄悄话 · IWhisper")
	if h.app.route.Page != "board" || h.app.route.Name != "IWhisper" {
		t.Fatalf("board route = %+v", h.app.route)
	}

	h.app.route = Route{Page: "thread", Name: "悄悄话", ID: "42", Number: 1}
	h.app.thread = forum.Thread{ID: "42", Board: "IWhisper", BoardDescription: "悄悄话", Articles: []forum.Article{
		{ID: "42", Board: "IWhisper", Author: forum.User{ID: "alice"}},
	}}
	h.view.Frame()
	h.click("复制链接")
	if !strings.Contains(h.device.copied, "/article/IWhisper/42") || strings.Contains(h.device.copied, "/article/悄悄话/42") {
		t.Fatalf("copied link = %q", h.device.copied)
	}
}

func TestDesktopListStateIsolatedByRouteAndPage(t *testing.T) {
	a := newHarness(t).app
	first := a.desktopListState(listKey{Page: "board", Name: "Go", Number: 1})
	first.Key = func(int) any { return "first" }
	second := a.desktopListState(listKey{Page: "board", Name: "Go", Number: 2})
	if first == second {
		t.Fatal("different pages shared desktop list state")
	}
	if got := a.desktopListState(listKey{Page: "board", Name: "Go", Number: 1}); got != first {
		t.Fatal("returning to page 1 did not reuse desktop list state")
	}
}

func TestBoardReturnRestoresListState(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.view.SetSize(1200, 260)
	articles := []forum.Article{
		{ID: "1", Board: "Go", Title: "第一主题"},
		{ID: "2", Board: "Go", Title: "第二主题"},
		{ID: "42", Board: "Go", Title: "第三主题"},
	}
	h.mu.Lock()
	h.overrides = map[string]string{
		"/open/board/Go.json": `{"name":"Go","description":"Go 讨论","article":[{"id":1,"title":"第一主题","board_name":"Go"},{"id":2,"title":"第二主题","board_name":"Go"},{"id":42,"title":"第三主题","board_name":"Go"}],"pagination":{"page_all_count":1,"page_current_count":1,"item_all_count":3}}`,
	}
	h.mu.Unlock()
	h.app.board = forum.Board{
		Name: "Go", Description: "Go 讨论", Articles: articles,
		Pagination: forum.Pagination{TotalPages: 1, CurrentPage: 1, TotalItems: 3},
	}
	h.app.route = Route{Page: "board", Name: "Go", Number: 1}
	h.view.Frame()

	key := listKey{Page: "board", Name: "Go", Number: 1}
	state := h.app.desktopListState(key)
	state.ScrollTo(2, ui.Start)
	h.view.Frame()
	first, _ := state.Visible()
	if first != 2 {
		t.Fatalf("board list first row before navigation = %d, want 2", first)
	}

	h.app.openArticle(articles[2])
	h.idle()
	h.app.goBack()
	h.idle()
	h.view.Frame()
	if h.app.route.Page != "board" || h.app.route.Name != "Go" || h.app.route.Number != 1 {
		t.Fatalf("route after board back = %+v", h.app.route)
	}
	got := h.app.desktopListState(key)
	if got != state {
		t.Fatal("board list state was replaced")
	}
	first, _ = got.Visible()
	if first != 2 {
		t.Fatalf("board list first row after navigation = %d, want 2", first)
	}
}

func TestSearchReturnKeepsQueryAndListState(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.view.SetSize(1200, 260)
	results := []forum.Article{
		{ID: "10", Board: "Go", Title: "搜索第一主题"},
		{ID: "11", Board: "Go", Title: "搜索第二主题"},
		{ID: "42", Board: "Go", Title: "搜索第三主题"},
	}
	h.app.route = Route{Page: "search", Number: 1}
	h.app.searchQuery = "搜索"
	h.app.searchHasRun = true
	h.app.searchResults = results
	h.app.searchPagination = forum.Pagination{TotalPages: 1, CurrentPage: 1, TotalItems: 3}
	h.view.Frame()

	key := listKey{Page: "search", Query: "搜索", Number: 1}
	state := h.app.desktopListState(key)
	state.ScrollTo(2, ui.Start)
	h.view.Frame()
	first, _ := state.Visible()
	if first != 2 {
		t.Fatalf("search list first row before navigation = %d, want 2", first)
	}
	if h.app.desktopListState(listKey{Page: "search", Query: "搜索", Number: 2}) == state {
		t.Fatal("search pages shared list state")
	}
	if h.app.desktopListState(listKey{Page: "search", Query: "另一个", Number: 1}) == state {
		t.Fatal("search queries shared list state")
	}

	h.app.openArticle(results[2])
	h.idle()
	h.app.goBack()
	h.idle()
	h.view.Frame()
	if h.app.route.Page != "search" || h.app.route.Number != 1 || h.app.searchQuery != "搜索" || len(h.app.searchResults) != 3 {
		t.Fatalf("search state after back: route=%+v query=%q results=%d", h.app.route, h.app.searchQuery, len(h.app.searchResults))
	}
	got := h.app.desktopListState(key)
	if got != state {
		t.Fatal("search list state was replaced")
	}
	first, _ = got.Visible()
	if first != 2 {
		t.Fatalf("search list first row after navigation = %d, want 2", first)
	}
}

func TestDesktopUserSearchOpensExactProfile(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "search"})
	h.typeIn("搜索关键词", "bob")
	h.click("人物")
	h.click("开始搜索")

	if h.app.route.Page != "user" {
		t.Fatalf("user search page = %q, want user", h.app.route.Page)
	}
	if h.app.route.Name != "bob" {
		t.Fatalf("user search route name = %q, want bob", h.app.route.Name)
	}
	if h.app.user.ID != "bob" {
		t.Fatalf("user search profile ID = %q, want bob", h.app.user.ID)
	}
	if got := h.count("GET /open/user/query/bob.json"); got != 1 {
		t.Fatalf("user search requests = %d, want one", got)
	}
}

func TestDesktopEmptyUserSearchDoesNotRequest(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "search"})
	h.click("人物")
	h.click("开始搜索")

	if got := h.count("GET /open/user/query/bob.json"); got != 0 {
		t.Fatalf("empty user search requests = %d, want zero", got)
	}
	if h.app.route.Page != "search" {
		t.Fatalf("empty user search changed page to %q", h.app.route.Page)
	}
}

func TestDesktopClearSearchResetsStateWithoutRequest(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.view.SetSize(1200, 260)
	h.app.route = Route{Page: "search", Number: 3}
	h.app.searchQuery = "搜索"
	h.app.searchHasRun = true
	h.app.searchResults = []forum.Article{
		{ID: "40", Board: "Go", Title: "搜索主题一"},
		{ID: "41", Board: "Go", Title: "搜索主题二"},
		{ID: "42", Board: "Go", Title: "搜索主题三"},
	}
	h.app.searchPagination = forum.Pagination{TotalPages: 3, CurrentPage: 3, TotalItems: 90}
	h.view.Frame()

	key := listKey{Page: "search", Query: "搜索", Number: 3}
	state := h.app.desktopListState(key)
	state.ScrollTo(2, ui.Start)
	h.view.Frame()
	if first, _ := state.Visible(); first != 2 {
		t.Fatalf("search list first row before clear = %d, want 2", first)
	}

	h.click("清空搜索")
	if h.app.searchQuery != "" || len(h.app.searchResults) != 0 || h.app.searchPagination != (forum.Pagination{}) || h.app.searchHasRun {
		t.Fatalf("cleared search state = query=%q results=%d pagination=%+v hasRun=%t", h.app.searchQuery, len(h.app.searchResults), h.app.searchPagination, h.app.searchHasRun)
	}
	if h.app.route.Number != 1 {
		t.Fatalf("cleared search page = %d, want one", h.app.route.Number)
	}
	if first, _ := state.Visible(); first != 0 {
		t.Fatalf("cleared search position = %d, want zero", first)
	}
	if got := h.count("GET /open/search/article.json"); got != 0 {
		t.Fatalf("clear search issued article requests = %d", got)
	}
}

func TestRealResponseFailureIsVisibleAndRetryLoadsSamePage(t *testing.T) {
	h := newHarness(t)
	h.login()
	if len(h.app.boards) == 0 {
		t.Fatal("home did not populate initial section")
	}
	h.mu.Lock()
	h.overrides = map[string]string{"/open/widget/topten.json": `{"article":[{"id":42,"user":{"level":{"invalid":true}}}]}`}
	h.mu.Unlock()
	h.app.navigate(Route{Page: "top"}, true)
	h.view.Frame()
	h.has("加载十大")
	if h.view.HasText("暂无内容") {
		t.Fatal("loading was presented as an empty result")
	}
	h.idle()
	h.has("内容加载失败")
	h.has("论坛返回的数据格式不兼容")
	h.has("诊断信息")
	if h.view.HasText("暂无内容") {
		t.Fatal("decoder failure was presented as an empty result")
	}
	h.click("复制加载诊断")
	if !strings.Contains(h.device.copied, "decode forum data") {
		t.Fatal("diagnostic details missing")
	}
	h.mu.Lock()
	delete(h.overrides, "/open/widget/topten.json")
	h.mu.Unlock()
	h.click("重新加载")
	h.has("测试主题")
	if h.app.route.Page != "top" || h.app.pageError != "" {
		t.Fatal("retry did not restore original page")
	}
}

func TestLoggedInContentFitsNativeWindowSizes(t *testing.T) {
	for _, size := range [][2]int{{1100, 760}, {960, 480}, {1200, 900}} {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			t.Run(fmt.Sprintf("%dx%d-scale%.2f", size[0], size[1], scale), func(t *testing.T) {
				h := newHarness(t)
				h.view.SetSize(size[0], size[1])
				h.view.SetScale(scale)
				h.login()
				contentLeft := h.app.navWidth()
				for _, page := range []string{"home", "top", "compose", "contacts", "self", "settings"} {
					h.goTo(Route{Page: Page(page)})
					label := h.app.title()
					if page == "settings" {
						label = "外观"
					}
					r, ok := h.view.Find(label)
					if !ok || r.X < contentLeft || r.Y < 0 || r.W <= 0 || r.H <= 0 || r.X+r.W > float32(size[0])+1 || r.Y+r.H > float32(size[1])+1 {
						t.Errorf("page %s has invisible content %q at %+v, found %t", page, label, r, ok)
					}
				}
			})
		}
	}
}
func TestDesktopRefreshReturnsToFirstPageAndTop(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "board", Name: "Go", Number: 3}
	h.app.board = forum.Board{Name: "Go", Articles: []forum.Article{
		{ID: "42", Board: "Go", Title: "测试主题"},
		{ID: "43", Board: "Go", Title: "第二主题"},
	}}
	key := listKey{Page: "board", Name: "Go", Number: 1}
	state := h.app.desktopListState(key)
	state.ScrollTo(1, ui.Start)
	h.view.Frame()
	h.click("刷新")
	if h.app.route.Number != 1 {
		t.Fatalf("refreshed board page = %d, want 1", h.app.route.Number)
	}
	state = h.app.desktopListState(key)
	if first, _ := state.Visible(); first != 0 {
		t.Fatalf("refreshed board list first row = %d, want 0", first)
	}
	if h.app.message != "刷新成功" {
		t.Fatalf("refresh message = %q, want 刷新成功", h.app.message)
	}
}

func TestDesktopPrivateMessageEditorRetainsTypedText(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "mail-compose", Name: "bob"}
	h.app.mailEditor = mailEditor{To: "bob", Mode: "send"}
	h.view.Frame()
	h.typeIn("信件正文", "abc")
	if got := h.app.mailEditor.Content; got != "abc" {
		t.Fatalf("private message content = %q, want abc", got)
	}
}

func TestDesktopNoticeExpiresAutomatically(t *testing.T) {
	h := newHarness(t)
	h.app.notice("已复制")
	if h.app.message != "已复制" {
		t.Fatalf("notice = %q", h.app.message)
	}
	time.Sleep(3100 * time.Millisecond)
	h.idle()
	if h.app.message != "" {
		t.Fatalf("transient notice did not expire: %q", h.app.message)
	}
}

func TestLoginBrowseReplyAndDraftLifecycle(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.click("校园")
	h.click("Go 讨论 · Go")
	h.click("测试主题")
	h.has("你好 北邮")
	h.click("复制链接")
	if !strings.Contains(h.device.copied, "/article/Go/42") {
		t.Fatal("share link missing")
	}
	h.click("回复此楼")
	if h.app.editor.ReplyID != "42" {
		t.Fatal("reply target lost")
	}
	h.typeIn("帖子正文", "回复正文")
	key := "composeDraft.Go.42"
	draft, ok, err := h.app.Core.LoadDraft(context.Background(), h.app.Core.SessionInfo(), key)
	if err != nil || !ok || draft.Content != "回复正文" {
		t.Fatalf("draft not saved: %+v %t %v", draft, ok, err)
	}
	h.click("提交帖子")
	if h.app.route.Page != "thread" {
		t.Fatalf("publish failed %s", h.app.message)
	}
	if _, ok, err := h.app.Core.LoadDraft(context.Background(), h.app.Core.SessionInfo(), key); err != nil || ok {
		t.Fatalf("published draft still exists: found=%v error=%v", ok, err)
	}
	for _, r := range h.app.back {
		if r.Page == "compose" {
			t.Fatal("published composer remains in back stack")
		}
	}
	h.goTo(Route{Page: "compose", Name: "Go"})
	h.typeIn("帖子标题", "新标题")
	h.typeIn("帖子正文", "新正文")
	h.click("添加附件")
	if !strings.Contains(h.app.editor.Content, "[upload=1]") {
		t.Fatal("attachment marker missing")
	}
	h.click("返回")
	h.goTo(Route{Page: "compose", Name: "Go"})
	if !strings.Contains(h.app.editor.Content, "新正文") {
		t.Fatal("draft did not restore")
	}
}
func TestHomeShowsSubsectionCardsInBoardPane(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.has("Go 讨论 · Go")
	// The subsection appears as a navigable card in the board pane, not in
	// the sidebar, and its boards load only after entering it.
	h.has("进入子分区 院系")
	if h.view.HasText("计算机学院 · CS") {
		t.Fatal("subsection boards leaked before entering the subsection")
	}
	h.click("进入子分区 院系")
	h.has("计算机学院 · CS")
	h.has("校园 > 院系 · 1 个版面")
	if h.app.section.Name != "1" || len(h.app.boards) != 1 {
		t.Fatalf("subsection not entered: %+v", h.app.section)
	}
	h.click("返回上级分区")
	h.has("Go 讨论 · Go")
}

func TestThreadReactionsCollectionAndQuoteMatchReference(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.click("校园")
	h.click("Go 讨论 · Go")
	h.click("测试主题")
	h.has("赞 2")
	h.click("赞 2")
	h.has("赞 3")
	h.has("赞 3")
	h.click("赞 3")
	if n := h.count("GET /open/article/Go/like/42.json"); n != 1 {
		t.Fatalf("like request count = %d", n)
	}
	h.has("已赞过，服务器不支持取消赞")
	h.click("踩 0")
	h.has("赞 2")
	h.has("踩 1")

	h.click("复制正文")
	if !strings.Contains(h.device.copied, "你好 北邮") {
		t.Fatalf("plain post copy missing: %q", h.device.copied)
	}

	h.click("收藏文章")
	h.has("已收藏文章")
	h.click("刷新")
	h.has("已收藏文章")

	h.click("回复此楼")
	h.has("回复 #1 · alice")
	h.has("引用到正文")
	h.click("引用到正文")
	if !strings.Contains(h.app.editor.Content, "[quote]你好 北邮[/quote]") {
		t.Fatalf("quote insertion missing: %q", h.app.editor.Content)
	}
}

func TestWhisperReactionUsesCanonicalBoard(t *testing.T) {
	h := newHarness(t)
	h.login()

	h.mu.Lock()
	h.overrides = map[string]string{
		"/open/article/IWhisper/like/42.json": `{"like_sum":3,"votedown_sum":0}`,
	}
	h.mu.Unlock()

	author := forum.User{ID: "alice"}
	root := forum.Article{
		ID:      "42",
		Board:   "IWhisper",
		Author:  author,
		LikeSum: 2,
	}
	h.app.route = Route{Page: "thread", Name: "悄悄话", ID: "42", Number: 1}
	h.app.thread = forum.Thread{
		ID:       "42",
		Board:    "IWhisper",
		LikeSum:  2,
		Articles: []forum.Article{root},
	}
	h.view.Frame()

	h.click("赞 2")

	if n := h.count("GET /open/article/IWhisper/like/42.json"); n != 1 {
		t.Fatalf("canonical whisper like requests = %d, want 1", n)
	}
	if n := h.count("GET /open/article/悄悄话/like/42.json"); n != 0 {
		t.Fatalf("localized board reached reaction endpoint %d times", n)
	}
	got := h.app.thread.Articles[0]
	if !bool(got.IsLiked) || got.LikeSum != 3 {
		t.Fatalf("reaction state = %+v", got)
	}
}

func TestAnonymousReactionTreatsUnresolvedAuthorAsRecorded(t *testing.T) {
	h := newHarness(t)
	h.login()

	h.mu.Lock()
	h.overrides = map[string]string{
		"/open/article/IWhisper/like/42.json": `{"code":"0500","msg":"未知的论坛ID"}`,
	}
	h.mu.Unlock()

	root := forum.Article{
		ID:      "42",
		Board:   "IWhisper",
		Author:  forum.User{ID: "IWhisper#175"},
		LikeSum: 2,
	}
	h.app.route = Route{Page: "thread", Name: "IWhisper", ID: "42", Number: 1}
	h.app.thread = forum.Thread{
		ID:       "42",
		Board:    "IWhisper",
		LikeSum:  2,
		Articles: []forum.Article{root},
	}
	h.view.Frame()

	h.click("赞 2")

	if n := h.count("GET /open/article/IWhisper/like/42.json"); n != 1 {
		t.Fatalf("anonymous like requests = %d, want 1", n)
	}
	if h.app.failed {
		t.Fatalf("recorded anonymous like was reported as failed: %q", h.app.message)
	}
	got := h.app.thread.Articles[0]
	if !bool(got.IsLiked) || got.LikeSum != 3 {
		t.Fatalf("anonymous reaction state = %+v", got)
	}

	// The optimistic state must suppress a duplicate like request.
	h.click("赞 3")
	if n := h.count("GET /open/article/IWhisper/like/42.json"); n != 1 {
		t.Fatalf("duplicate anonymous like reached the server %d times", n)
	}
}

func TestDesktopReplyReminderOpensTargetFloor(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.view.SetSize(640, 420)
	h.goTo(Route{Page: "refers", Name: "reply", Number: 1})
	h.click("[未读] 通知主题")
	h.view.Frame()
	if h.app.route.Page != "thread" || h.app.route.ID != "42" || h.app.route.Sub != "44" {
		t.Fatalf("reply reminder route = %+v, want thread 42 floor 44", h.app.route)
	}
	state := h.app.desktopListState(h.app.threadListKey())
	first, _ := state.Visible()
	if first != 2 {
		t.Fatalf("thread list row = %d, want target floor 44 at row 2 after the header row", first)
	}
	h.has("回复正文")
}

func TestDesktopLongPrivateMessageFoldsAndExpands(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "chat", Name: "alice"}
	h.app.chat = []application.ChatMessage{{
		Mail: forum.Mail{Box: "outbox", Index: "12", Content: strings.Repeat("长消息内容。", 100)},
		Mine: true,
	}}
	h.view.Frame()
	h.has("展开全文")
	h.click("展开全文")
	h.has("收起全文")
	if !h.app.expandedMessages["outbox12"] {
		t.Fatal("expanded message state not stored by box/index")
	}
}

func TestTimelineIsSeparateFromFavorites(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "timeline"})
	h.has("时间线新帖")
	h.has("时间线旧帖")
	if len(h.app.articles) != 2 || h.app.articles[0].Title != "时间线新帖" {
		t.Fatalf("timeline not sorted newest first: %+v", h.app.articles)
	}
	h.goTo(Route{Page: "favorites"})
	if h.view.HasText("关注主题") {
		t.Fatal("timeline is still shown inside favorites")
	}
	h.has("文章收藏")
}

func TestSearchFavoritesAndAllMigratedScreens(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "search"})
	h.has("全站搜索")
	if h.view.HasText("AI 搜索") || h.view.HasText("敬请期待") {
		t.Fatalf("search page still has oversized/redundant header content: %q", h.view.Texts())
	}
	h.typeIn("搜索关键词", "测试")
	h.click("开始搜索")
	h.has("测试主题")
	if n := h.count("GET /open/search/article.json"); n < 1 {
		t.Fatalf("search requests = %d", n)
	}
	h.goTo(Route{Page: "favorites", Sub: "collection"})
	h.has("收藏主题")
	h.goTo(Route{Page: "contacts"})
	h.click("bob")
	h.has("发出的完整信件正文")
	h.has("收到的完整信件正文")
	h.goTo(Route{Page: "mailbox", Name: "inbox"})
	h.click("[未读] 来信标题")
	h.has("收到的完整信件正文")
	h.click("回复站内信")
	h.typeIn("信件正文", "回复信件正文")
	h.click("发送信件")
	if h.app.failed {
		t.Fatal(h.app.message)
	}
	for _, r := range []Route{{Page: "refers", Name: "reply"}, {Page: "blacklist"}, {Page: "self"}, {Page: "user", Name: "bob"}, {Page: "history"}, {Page: "settings"}, {Page: "vote", ID: "42"}, {Page: "bet", ID: "42"}, {Page: "top"}} {
		h.goTo(r)
		h.has(h.app.title())
	}
}

func TestCloseAfterServerPublishBeforeUIUpdateCannotRecreateDraft(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "compose", Name: "Go"})
	h.typeIn("帖子标题", "标题")
	h.typeIn("帖子正文", "正文")
	if err := h.view.Click("提交帖子"); err != nil {
		t.Fatal(err)
	}
	var done func()
	select {
	case done = <-h.updates:
	case <-time.After(time.Second):
		t.Fatal("publish did not complete")
	}
	if _, ok, err := h.app.Core.LoadDraft(context.Background(), h.app.Core.SessionInfo(), "composeDraft.Go.new"); err != nil || ok {
		t.Fatalf("server success did not remove draft: found=%v error=%v", ok, err)
	}
	h.app.Close()
	done()
	if _, ok, err := h.app.Core.LoadDraft(context.Background(), h.app.Core.SessionInfo(), "composeDraft.Go.new"); err != nil || ok {
		t.Fatalf("window close recreated an already published draft: found=%v error=%v", ok, err)
	}
}
func TestCloseCancelsPendingReadAndDiscardsQueuedResult(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.run("pending", func(ctx context.Context) (func(), error) {
		<-ctx.Done()
		return func() { h.app.message = "stale result" }, nil
	})
	h.app.Close()
	select {
	case fn := <-h.updates:
		fn()
	case <-time.After(time.Second):
		t.Fatal("canceled read did not complete")
	}
	if h.app.message == "stale result" {
		t.Fatal("closed window accepted stale data")
	}
}
func TestUploadMarkerRenumbering(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		body := renumberUploads("a[upload=1][/upload]b[upload=2][/upload]c[upload=3][/upload]", n)
		if strings.Count(body, "[upload=") != 2 || strings.Contains(body, "[upload=3]") {
			t.Fatal(fmt.Sprint(n, body))
		}
	}
}

func TestDesktopBoardPinnedTopicsCollapseByDefault(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "board", Name: "Go", Number: 1}
	h.app.board = forum.Board{Name: "Go", Articles: []forum.Article{
		{ID: "pin-1", Board: "Go", Title: "置顶主题一", IsTop: true},
		{ID: "pin-2", Board: "Go", Title: "置顶主题二", IsTop: true},
		{ID: "normal-1", Board: "Go", Title: "普通主题一"},
		{ID: "normal-2", Board: "Go", Title: "普通主题二"},
	}}
	h.view.Frame()
	if countText(h.view.Texts(), "置顶主题 · 2") != 1 {
		t.Fatalf("pinned summary count = %d, texts=%q", countText(h.view.Texts(), "置顶主题 · 2"), h.view.Texts())
	}
	if h.view.HasText("置顶主题一") || h.view.HasText("置顶主题二") {
		t.Fatalf("pinned topics were visible before expansion: %q", h.view.Texts())
	}
	h.has("普通主题一")
	h.has("普通主题二")

	h.click("置顶主题 · 2")
	h.has("置顶主题一")
	h.has("置顶主题二")
}

func TestDesktopPinnedStateIsolatedByThreadAndPage(t *testing.T) {
	h := newHarness(t)
	h.login()
	root := forum.Article{ID: "42", Board: "Go", Title: "置顶根楼", IsTop: true, Content: "置顶正文"}
	h.app.route = Route{Page: "thread", Name: "Go", ID: "42", Number: 1}
	h.app.thread = forum.Thread{ID: "42", Board: "Go", Title: "主题", Articles: []forum.Article{root}}
	h.view.Frame()
	if h.view.HasText("置顶正文") {
		t.Fatalf("pinned thread body was visible before expansion: %q", h.view.Texts())
	}
	h.click("展开置顶内容")
	h.has("置顶正文")

	h.app.route.Number = 2
	h.view.Frame()
	if h.view.HasText("置顶正文") {
		t.Fatalf("page 2 reused expanded floor state: %q", h.view.Texts())
	}
	h.app.route.Number = 1
	h.view.Frame()
	h.has("置顶正文")
}

func TestDesktopThreadResourcePreflightRequiresMatchingArticle(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.mu.Lock()
	h.overrides = map[string]string{
		"/open/vote/42.json": `{"vid":7,"aid":42,"title":"投票","options":[{"viid":1,"label":"选项","num":1}]}`,
		"/open/bet/42.json":  `{"code":"404","msg":"not found"}`,
	}
	h.mu.Unlock()
	h.goTo(Route{Page: "thread", Name: "Go", ID: "42", Number: 1})
	h.idle()
	h.view.Frame()
	h.idle()
	h.view.Frame()
	h.has("查看投票")
	if h.view.HasText("查看竞猜") {
		t.Fatalf("not-found bet entry was visible: %q", h.view.Texts())
	}
	if got := h.count("GET /open/vote/42.json"); got != 1 {
		t.Fatalf("vote preflight requests = %d, want one", got)
	}
	if got := h.count("GET /open/bet/42.json"); got != 1 {
		t.Fatalf("bet preflight requests = %d, want one", got)
	}

	h.mu.Lock()
	h.overrides["/open/vote/42.json"] = `{"vid":7,"aid":43,"title":"其他主题投票","options":[{"viid":1,"label":"选项","num":1}]}`
	h.mu.Unlock()
	h.click("刷新")
	h.idle()
	h.view.Frame()
	h.idle()
	h.view.Frame()
	if h.view.HasText("查看投票") || h.view.HasText("查看竞猜") {
		t.Fatalf("mismatched resources exposed: %q", h.view.Texts())
	}
	if got := h.count("GET /open/vote/42.json"); got != 2 {
		t.Fatalf("vote refresh preflight requests = %d, want two", got)
	}
}
