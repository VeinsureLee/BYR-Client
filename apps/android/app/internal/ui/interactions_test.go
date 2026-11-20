package ui

import (
	"context"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

type recordedRequest struct {
	Method string
	Path   string
	Form   string
}

type interactionServer struct {
	mu       sync.Mutex
	requests []recordedRequest
	server   *httptest.Server
}

func newInteractionServer(t *testing.T) *interactionServer {
	t.Helper()
	s := &interactionServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.requests = append(s.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Form: string(body)})
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/open/section.json":
			_, _ = w.Write([]byte(`{"section":[{"name":"0"}]}`))
		case r.URL.Path == "/open/section/0.json":
			_, _ = w.Write([]byte(`{"name":"0","board":[{"name":"Go"}]}`))
		case strings.HasSuffix(r.URL.Path, "/like/42.json"):
			_, _ = w.Write([]byte(`{"status":true,"like_sum":3,"votedown_sum":0}`))
		case strings.HasSuffix(r.URL.Path, "/votedown/44.json"):
			_, _ = w.Write([]byte(`{"status":true,"like_sum":0,"votedown_sum":6}`))
		case r.URL.Path == "/open/threads/Go/42.json":
			_, _ = w.Write([]byte(`{"id":42,"title":"主题","board_name":"Go","article":[{"id":42,"position":1,"content":"正文","like_sum":3}]}`))
		case r.URL.Path == "/open/collection.json":
			_, _ = w.Write([]byte(`{"article":[{"gid":42,"bname":"Go","title":"收藏文章","num":1,"user":{"id":"alice","user_name":"Alice"}}]}`))
		case r.URL.Path == "/open/search/article.json":
			_, _ = w.Write([]byte(`{"article":[{"id":42,"group_id":42,"board_name":"Go","title":"搜索主题","user":{"id":"alice","user_name":"Alice"}}],"pagination":{"page_all_count":1,"page_current_count":1,"item_all_count":1}}`))
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *interactionServer) find(method, path string) []recordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []recordedRequest
	for _, r := range s.requests {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

func interactionApp(t *testing.T, baseURL string) *App {
	t.Helper()
	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	credentials := &testCredentials{session: application.Credential{
		Username: "alice", Token: "token", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}}
	core, err := application.New(application.Options{
		BaseURL:     baseURL,
		HTTPClient:  http.DefaultClient,
		Local:       store,
		Credentials: credentials,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.ready = true
	a.core = core
	a.stack = []Route{{Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	a.thread = sampleThread()
	return a
}

func TestThreadLoadSavesHistoryWithLiveAppContext(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}

	a.openThread("Go", "42")
	waitForIdle(t, a)

	entries := a.core.Local.History("alice")
	if len(entries) != 1 {
		t.Fatalf("history entries = %+v, want one visited thread", entries)
	}
	if entries[0].Board != "Go" || entries[0].ID != "42" || entries[0].Title != "主题" {
		t.Fatalf("saved history = %+v", entries[0])
	}
	if a.failed || strings.Contains(a.message, "context canceled") {
		t.Fatalf("successful thread load reported history failure: failed=%v message=%q", a.failed, a.message)
	}
}

func TestTransientNoticeExpiresAutomatically(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.notice("已复制")
	if a.message != "已复制" {
		t.Fatalf("notice = %q", a.message)
	}
	time.Sleep(2600 * time.Millisecond)
	a.drain()
	if a.message != "" {
		t.Fatalf("transient notice did not expire: %q", a.message)
	}
}

func TestLongPrivateMessageFoldsIndependently(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	message := forum.Mail{Box: "inbox", Index: "1", Content: strings.Repeat("长消息内容。", 80)}
	key := "inbox:1"

	renderAndroidPage(t, func(gtx layout.Context) layout.Dimensions {
		return a.messageBody(gtx, key, message)
	})
	if _, ok := a.clicks["mail-fold:"+key]; !ok {
		t.Fatal("long private message did not render an expand control")
	}
	a.click("mail-fold:" + key).Click()
	renderAndroidPage(t, func(gtx layout.Context) layout.Dimensions {
		return a.messageBody(gtx, key, message)
	})
	if !a.expandedMail[key] {
		t.Fatal("long private message did not expand")
	}
	other := "inbox:2"
	renderAndroidPage(t, func(gtx layout.Context) layout.Dimensions {
		return a.messageBody(gtx, other, message)
	})
	if a.expandedMail[other] {
		t.Fatal("expanded state leaked between messages")
	}
}
func TestThreadAuthorOpensUserProfile(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.thread = sampleThread()
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	a.click("post-author:0").Click()
	ops.Reset()
	a.Layout(gtx)
	if got := a.current().Page; got != PageUser {
		t.Fatalf("author tap page = %v, want user", got)
	}
	if got := a.current().Name; got != "alice" {
		t.Fatalf("user name = %q, want alice", got)
	}
}

func TestContactOpensChatWithoutResettingList(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.contacts = []application.Contact{{
		User: forum.User{ID: "bob", UserName: "Bob"}, Preview: "你好",
	}}
	a.stack = []Route{{Page: PageSelf}, {Page: PageMessages, Sub: "contacts", Number: 1}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	a.click("contact:bob").Click()
	ops.Reset()
	a.Layout(gtx)
	if got := a.current().Page; got != PageChat {
		t.Fatalf("contact tap page = %v, want chat", got)
	}
	if got := a.current().Name; got != "bob" {
		t.Fatalf("chat target = %q", got)
	}
	ops.Reset()
	a.Layout(gtx)
}

func TestSelfSettingsEntryOpensSettings(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.user = forum.User{ID: "alice", UserName: "Alice"}
	a.stack = []Route{{Page: PageSelf}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 760)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	a.click("self-settings").Click()
	ops.Reset()
	a.Layout(gtx)
	if got := a.current().Page; got != PageSettings {
		t.Fatalf("settings entry page = %v, want settings", got)
	}
	// The theme choices must be present and tappable.
	a.click("settings-theme:dark").Click()
	ops.Reset()
	a.Layout(gtx)
	defer func() {
		prefs := a.core.Local.Preferences()
		prefs.Theme = 0
		_ = a.core.Local.SetPreferences(context.Background(), prefs)
		setPalette(false)
		a.dark, a.themed = false, false
		a.theme.Bg, a.theme.Fg = colorBg, colorFg
	}()
	if got := a.core.Local.Preferences().Theme; got != 2 {
		t.Fatalf("theme preference = %d, want dark", got)
	}
}

func TestThreadLikeUsesCanonicalBoardName(t *testing.T) {
	tests := []struct {
		name         string
		threadBoard  string
		articleBoard string
	}{
		{name: "thread field", threadBoard: "IWhisper", articleBoard: "IWhisper"},
		{name: "root article fallback", articleBoard: "IWhisper"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newInteractionServer(t)
			a := interactionApp(t, s.server.URL)
			defer a.Close()

			thread := sampleThread()
			thread.Board = tc.threadBoard
			thread.Articles[0].Board = tc.articleBoard
			thread.Articles[0].Author = forum.User{ID: "alice"}
			a.thread = thread
			a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "悄悄话", ID: "42", Number: 1}}

			a.likeThread()
			waitForIdle(t, a)

			if len(s.find("GET", "/open/article/IWhisper/like/42.json")) != 1 {
				t.Fatalf("like did not use canonical board: %+v", s.requests)
			}
			if len(s.find("GET", "/open/article/悄悄话/like/42.json")) != 0 {
				t.Fatalf("localized board reached endpoint: %+v", s.requests)
			}
			got := a.thread.Articles[0]
			if got.LikeSum != 3 || !bool(got.IsLiked) {
				t.Fatalf("thread reaction state = %+v", got)
			}
			if got.Author.ID != "alice" {
				t.Fatalf("reaction altered the author: %+v", got.Author)
			}
		})
	}
}

func TestAnonymousLikeTreatsUnresolvedAuthorAsRecorded(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/like/42.json") {
			calls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"code":"0500","msg":"未知的论坛ID"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	a := interactionApp(t, server.URL)
	defer a.Close()

	thread := sampleThread()
	thread.Board = "IWhisper"
	thread.Articles[0].Board = "IWhisper"
	thread.Articles[0].Author = forum.User{ID: "IWhisper#175"}
	a.thread = thread
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "IWhisper", ID: "42", Number: 1}}

	a.likeThread()
	waitForIdle(t, a)

	if calls != 1 {
		t.Fatalf("anonymous like requests = %d, want 1", calls)
	}
	if a.failed {
		t.Fatalf("recorded anonymous like was reported as failed: %q", a.message)
	}
	if got := a.thread.Articles[0]; !bool(got.IsLiked) || got.LikeSum != 3 {
		t.Fatalf("anonymous reaction state = %+v", got)
	}
}

func TestReplyReminderOpensTargetFloorAtCorrectPage(t *testing.T) {
	var threadQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/refer/reply/setRead/1.json":
			_, _ = io.WriteString(w, `{"code":"0000"}`)
		case "/open/threads/Go/42.json":
			threadQuery = r.URL.RawQuery
			_, _ = io.WriteString(w, `{"id":42,"title":"主题","board_name":"Go","article":[{"id":43,"position":30,"content":"上一楼","user":{"id":"alice"}},{"id":44,"position":31,"content":"提醒目标楼层","user":{"id":"bob"}}],"pagination":{"page_current_count":2,"page_all_count":3}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageMessages, Sub: "reply", Number: 1}}
	a.openRefer(forum.ReferEntry{Index: "1", ID: "44", GroupID: "42", Board: "Go", Pos: 31})
	waitForIdle(t, a)

	if got := a.current(); got.Page != PageThread || got.ID != "42" || got.Sub != "44" || got.Number != 2 {
		t.Fatalf("opened reminder route = %+v", got)
	}
	if !strings.Contains(threadQuery, "page=2") || !strings.Contains(threadQuery, "count=30") {
		t.Fatalf("thread request query = %q, want page=2 count=30", threadQuery)
	}
	key := listKey{Page: PageThread, Name: "Go", ID: "42", Sub: "44", Number: 2}
	if got := a.mobileList(key).Position.First; got != 2 {
		t.Fatalf("target floor list position = %d, want row 2 (header plus first floor)", got)
	}
}

func TestThreadLikeWhileLoadingIsRejected(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.thread = forum.Thread{}
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	a.likeThread()
	waitForIdle(t, a)
	if len(s.find("GET", "/open/article/Go/like/42.json")) != 0 {
		t.Fatalf("empty thread sent a reaction request: %+v", s.requests)
	}
	if !a.failed || !strings.Contains(a.message, "稍候") {
		t.Fatalf("loading reaction was not explained: failed=%v message=%q", a.failed, a.message)
	}
}

func TestPinnedPostStartsCollapsedAndExpands(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	thread := sampleThread()
	thread.Articles[0].IsTop = true
	a.thread = thread
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 760)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	if a.expandedPosts[threadExpansionKey(a.current(), "42")] {
		t.Fatal("pinned post should start collapsed")
	}
	a.click("pin-toggle:42:0").Click()
	ops.Reset()
	a.Layout(gtx)
	if !a.expandedPosts[threadExpansionKey(a.current(), "42")] {
		t.Fatal("pinned post did not expand")
	}
}

func TestBoardPinnedTopicsCollapseByDefault(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.board = forum.Board{Name: "Go", Articles: []forum.Article{
		{ID: "1", Title: "置顶公告", IsTop: true},
		{ID: "2", Title: "普通主题"},
	}}
	a.stack = []Route{{Page: PageHome}, {Page: PageBoard, Name: "Go", Number: 1}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 760)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	if a.pinnedExpanded {
		t.Fatal("pinned topics should start collapsed")
	}
	a.click("board-pinned-toggle").Click()
	ops.Reset()
	a.Layout(gtx)
	if !a.pinnedExpanded {
		t.Fatal("pinned topics did not expand")
	}
}

func TestThreadReactionUpdatesServerCounts(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.likeThread()
	waitForIdle(t, a)
	if len(s.find("GET", "/open/article/Go/like/42.json")) != 1 {
		t.Fatalf("like request missing: %+v", s.requests)
	}
	if got := a.thread.LikeSum; got != 3 {
		t.Fatalf("like count = %d, want 3", got)
	}
	if !bool(a.thread.IsLiked) {
		t.Fatal("thread was not marked liked")
	}
	if len(a.thread.Articles) == 0 || !bool(a.thread.Articles[0].IsLiked) {
		t.Fatal("root post was not updated")
	}

	// A second like must not call the server again.
	a.likeThread()
	waitForIdle(t, a)
	if len(s.find("GET", "/open/article/Go/like/42.json")) != 1 {
		t.Fatalf("duplicate like reached the server: %+v", s.requests)
	}
}

func TestPostDownvoteUpdatesServerCounts(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.react(1, a.thread.Articles[1], application.Downvote)
	waitForIdle(t, a)
	if len(s.find("GET", "/open/article/Go/votedown/44.json")) != 1 {
		t.Fatalf("downvote request missing: %+v", s.requests)
	}
	got := a.thread.Articles[1]
	if got.VotedownSum != 6 || !bool(got.IsVotedown) {
		t.Fatalf("downvote state = %+v", got)
	}
}

func TestPostReplyOpensComposer(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.openCompose("Go", "44", "reply")
	if got := a.current().Page; got != PageCompose {
		t.Fatalf("page = %v, want compose", got)
	}
	if a.compose.replyID != "44" || !a.compose.ready {
		t.Fatalf("reply composer state = %+v", a.compose)
	}
}

func TestCollectThreadTogglesServerState(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.collectThread()
	waitForIdle(t, a)
	if len(s.find("POST", "/open/collection/add.json")) != 1 {
		t.Fatalf("collect request missing: %+v", s.requests)
	}
	if !bool(a.thread.Collect) {
		t.Fatal("thread was not marked collected")
	}
	a.collectThread()
	waitForIdle(t, a)
	if len(s.find("POST", "/open/collection/delete.json")) != 1 {
		t.Fatalf("uncollect request missing: %+v", s.requests)
	}
	if bool(a.thread.Collect) {
		t.Fatal("thread stayed collected")
	}
}

func TestFavoritesTabSwitchLoadsCollection(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageFavorites, Number: 1}}
	a.favoriteTab = "boards"
	a.loadFavorites()
	waitForIdle(t, a)

	a.selectFavoriteTab("collection")
	waitForIdle(t, a)
	if a.favoriteTab != "collection" {
		t.Fatalf("favourite tab = %q, want collection", a.favoriteTab)
	}
	if len(s.find("GET", "/open/collection.json")) != 1 {
		t.Fatalf("collection request missing: %+v", s.requests)
	}
	if len(a.collect.Entries) != 1 || a.collect.Entries[0].ID != "42" {
		t.Fatalf("collection entries = %+v", a.collect.Entries)
	}
	if a.stack[0].Number != 1 {
		t.Fatalf("collection page number = %d, want 1", a.stack[0].Number)
	}
}

func TestFavoritesTabSwitchIgnoredWhileBusy(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageFavorites, Number: 1}}
	a.favoriteTab = "boards"
	a.busy = true
	a.selectFavoriteTab("collection")
	if a.favoriteTab != "boards" {
		t.Fatalf("busy tab switch changed tab to %q", a.favoriteTab)
	}
	a.busy = false
}

func TestCollectionCardOpensThread(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageFavorites, Number: 1}}
	a.favoriteTab = "collection"
	entry := forum.CollectionEntry{
		ID: "42", Board: "Go", Title: "收藏文章", ReplyCount: 1,
		Author: forum.User{ID: "alice", UserName: "Alice"},
	}
	a.openCollectionEntry(entry)
	if got := a.current().Page; got != PageThread {
		t.Fatalf("page = %v, want thread", got)
	}
	if got := a.current().ID; got != "42" {
		t.Fatalf("thread id = %q, want 42", got)
	}
	if !a.core.CollectionState("Go", "42", true) {
		t.Fatal("collection state was not remembered for the opened thread")
	}
}

func TestCollectionRemoveUsesBoardQualifiedRequest(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageFavorites, Number: 1}}
	a.favoriteTab = "collection"

	entry := forum.Article{ID: "42", Board: "Go"}
	a.run("取消收藏", func(ctx context.Context) (func(), error) {
		_, err := a.core.SetCollected(ctx, entry.Board, string(entry.ID), false)
		return func() { a.loadFavorites() }, err
	})
	waitForIdle(t, a)
	requests := s.find("POST", "/open/collection/delete.json")
	if len(requests) != 1 {
		t.Fatalf("uncollect request missing: %+v", s.requests)
	}
	if !strings.Contains(requests[0].Form, "board=Go") || !strings.Contains(requests[0].Form, "id=42") {
		t.Fatalf("uncollect form = %q", requests[0].Form)
	}
}

func TestMessageTabsCanonicalizeRouteNames(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()

	for _, tab := range []string{"contacts", "inbox", "outbox"} {
		a.stack = []Route{{Page: PageMessages, Sub: "mail", Name: tab, Number: 1}}
		a.busy = false
		a.selectMessageTab(tab)
		waitForIdle(t, a)
		if got := a.current(); got.Page != PageMessages || got.Sub != "mail" || got.Name != "contacts" {
			t.Fatalf("mail destination %q route = %+v, want aggregated mail/contacts", tab, got)
		}
	}

	a.stack = []Route{{Page: PageMessages, Sub: "mail", Name: "contacts", Number: 1}}
	a.selectMessageTab("reply")
	waitForIdle(t, a)
	if got := a.current(); got.Sub != "reply" || got.Name != "" {
		t.Fatalf("reply reminder route = %+v", got)
	}
	a.selectMessageTab("at")
	waitForIdle(t, a)
	if got := a.current(); got.Sub != "at" || got.Name != "" {
		t.Fatalf("at reminder route = %+v", got)
	}
	if got := a.referKind(); got != "at" {
		t.Fatalf("refer kind = %q, want at", got)
	}
}

func TestReferReadAllUsesCanonicalEndpointAndReloads(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageMessages, Sub: "reply", Number: 1}}
	a.refers = forum.Refers{Articles: []forum.ReferEntry{{Index: "1", Title: "回复提醒"}}}

	a.readAllRefers()
	waitForIdle(t, a)

	if got := len(s.find("POST", "/open/refer/reply/setRead.json")); got != 1 {
		t.Fatalf("reply read-all requests = %d, want one: %+v", got, s.requests)
	}
	if got := len(s.find("GET", "/open/refer/reply.json")); got != 1 {
		t.Fatalf("reply reload requests = %d, want one: %+v", got, s.requests)
	}
	if got := len(s.find("POST", "/open/refer//setRead.json")); got != 0 {
		t.Fatalf("empty refer kind requests = %d", got)
	}
	if a.message != "已全部读" {
		t.Fatalf("read-all feedback = %q, want 已全部读", a.message)
	}
}

func TestThreadFloorUsesCompactActionsAndCopyFeedback(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	a.thread = sampleThread()

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 760)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.threadPage(gtx)
	for _, key := range []string{"like:42", "down:42", "reply:42", "copy:42"} {
		if _, ok := a.clicks[key]; !ok {
			t.Fatalf("compact floor action %q was not registered", key)
		}
	}

	a.click("copy:42").Click()
	ops.Reset()
	a.Layout(gtx)
	if a.message != "已复制" {
		t.Fatalf("copy feedback = %q, want 已复制", a.message)
	}
	if a.failed {
		t.Fatal("copy success was marked as failed")
	}
}

func TestCollectionSuccessShowsFeedback(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()

	a.collectThread()
	waitForIdle(t, a)
	if got := a.message; got != "已收藏文章" {
		t.Fatalf("thread collect feedback = %q, want 已收藏文章", got)
	}

	a.thread.Collect = true
	a.collectThread()
	waitForIdle(t, a)
	if got := a.message; got != "已取消收藏文章" {
		t.Fatalf("thread uncollect feedback = %q, want 已取消收藏文章", got)
	}

	a.stack = []Route{{Page: PageBoard, Name: "Go", Number: 1}}
	a.board = forum.Board{Name: "Go"}
	a.toggleBoardFavorite()
	waitForIdle(t, a)
	if got := a.message; got != "已收藏版面" {
		t.Fatalf("board collect feedback = %q, want 已收藏版面", got)
	}
}

func TestBusyFloorClickDoesNotOverwriteSuccessNotice(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	a.thread = sampleThread()
	a.message = "首次成功"
	a.busy = true

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 760)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.threadPage(gtx)
	a.click("copy:42").Click()
	ops.Reset()
	a.threadPage(gtx)
	if got := a.message; got != "首次成功" {
		t.Fatalf("busy copy changed success notice to %q", got)
	}
}

func TestPinnedStateIsolatedByThreadAndPage(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	thread := sampleThread()
	thread.Articles[0].IsTop = true

	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	a.thread = thread
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 760)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.threadPage(gtx)
	a.click("pin-toggle:42:0").Click()
	ops.Reset()
	a.threadPage(gtx)
	first := a.current()
	if !a.expandedPosts[threadExpansionKey(first, "42")] {
		t.Fatal("first thread pinned post did not expand")
	}

	a.stack[len(a.stack)-1] = Route{Page: PageThread, Name: "Go", ID: "43", Number: 1}
	a.thread = thread
	ops.Reset()
	a.threadPage(gtx)
	if a.expandedPosts[threadExpansionKey(a.current(), "42")] {
		t.Fatal("pinned state leaked to a different thread")
	}

	a.stack[len(a.stack)-1].Number = 2
	ops.Reset()
	a.threadPage(gtx)
	if a.expandedPosts[threadExpansionKey(a.current(), "42")] {
		t.Fatal("pinned state leaked to a different page")
	}
}
