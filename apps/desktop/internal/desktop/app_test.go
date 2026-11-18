package desktop

import (
	"context"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/apps/desktop/internal/bootstrap"
	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/resources/fonts"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if err := ui.RegisterFont(fonts.Regular, fonts.Family); err != nil {
		panic(err)
	}
	if os.Getenv("BYR_DIAGNOSE_SAVED_SESSION") == "1" {
		if err := diagnoseLiveSession(); err != nil {
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

type testDevice struct{ copied string }

func (d *testDevice) Copy(v string) error { d.copied = v; return nil }
func (*testDevice) Open(string) error     { return nil }
func (*testDevice) Pick(context.Context) (string, []byte, error) {
	return "test.txt", []byte("file content"), nil
}
func (*testDevice) Save(string, []byte) error { return nil }

type harness struct {
	app       *App
	view      *ui.Tester
	updates   chan func()
	mu        sync.Mutex
	requests  []string
	t         *testing.T
	device    *testDevice
	overrides map[string]string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{updates: make(chan func(), 256), t: t, device: &testDevice{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.requests = append(h.requests, r.Method+" "+r.URL.Path)
		h.mu.Unlock()
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
		h.mu.Unlock()
		if ok {
			body = override
		}
		io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	services, err := bootstrap.New(bootstrap.Config{BaseURL: server.URL, Credentials: &testCredentials{}})
	if err != nil {
		t.Fatal(err)
	}
	h.app = New(services.App, h.device, func(fn func()) { h.updates <- fn })
	h.view = ui.NewTester(h.app.View, 1200, 900)
	t.Cleanup(h.app.Close)
	return h
}
func (h *harness) idle() {
	h.t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		if !h.app.busy {
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
	h.click("回复主题")
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
				for _, page := range []string{"home", "top", "compose", "contacts", "self", "settings"} {
					h.goTo(Route{Page: Page(page)})
					label := h.app.title()
					if page == "settings" {
						label = "外观"
					}
					r, ok := h.view.Find(label)
					if !ok || r.X < 105 || r.Y < 0 || r.W <= 0 || r.H <= 0 || r.X+r.W > float32(size[0])+1 || r.Y+r.H > float32(size[1])+1 {
						t.Errorf("page %s has invisible content %q at %+v, found %t", page, label, r, ok)
					}
				}
			})
		}
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
	h.click("回复主题")
	if h.app.editor.ReplyID != "42" {
		t.Fatal("reply target lost")
	}
	h.typeIn("帖子正文", "回复正文")
	key := "composeDraft.Go.42"
	if d, ok := h.app.Core.Local.Draft("alice", key); !ok || d.Content != "回复正文" {
		t.Fatalf("draft not saved: %+v %t", d, ok)
	}
	h.click("提交帖子")
	if h.app.route.Page != "thread" {
		t.Fatalf("publish failed %s", h.app.message)
	}
	if _, ok := h.app.Core.Local.Draft("alice", key); ok {
		t.Fatal("published draft recreated during navigation")
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
	h.has("已点赞 3")
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
	h.has("AI 搜索")
	h.has("敬请期待")
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
	if _, ok := h.app.Core.Local.Draft("alice", "composeDraft.Go.new"); ok {
		t.Fatal("server success did not remove draft")
	}
	h.app.Close()
	done()
	if _, ok := h.app.Core.Local.Draft("alice", "composeDraft.Go.new"); ok {
		t.Fatal("window close recreated an already published draft")
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
