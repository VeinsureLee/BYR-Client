package ui

import (
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestSearchPageUsesRemoteSearchForAllMode(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	if a.searchMode != "ai" {
		t.Fatalf("default search mode = %q, want ai", a.searchMode)
	}
	a.click("search-submit").Click()
	ops.Reset()
	a.Layout(gtx)
	if len(s.requests) != 0 {
		t.Fatalf("AI search issued forum requests: %+v", s.requests)
	}
	if !strings.Contains(a.message, "AI 搜索尚未配置") {
		t.Fatalf("AI search notice = %q", a.message)
	}

	a.click("search-mode:all").Click()
	ops.Reset()
	a.Layout(gtx)
	a.searchEdit.SetText("搜索")
	a.click("search-submit").Click()
	ops.Reset()
	a.Layout(gtx)
	waitForIdle(t, a)
	if got := a.current().Page; got != PageSearch {
		t.Fatalf("page after search = %v, want search", got)
	}
	if a.renderPanic != "" {
		t.Fatalf("search render panic = %q", a.renderPanic)
	}
	if got := a.core.Session().Token; got != "token" {
		t.Fatalf("search discarded session token %q", got)
	}
	if len(a.searchResults) != 1 || a.searchResults[0].Title != "搜索主题" {
		t.Fatalf("search results = %+v", a.searchResults)
	}
	if len(s.find("GET", "/open/search/article.json")) != 1 {
		t.Fatalf("search requests = %+v", s.requests)
	}

	ops.Reset()
	a.Layout(gtx)
	if got := a.current().Page; got != PageSearch || a.renderPanic != "" {
		t.Fatalf("second search frame page=%v panic=%q", got, a.renderPanic)
	}
}

func TestSearchListResetWaitsForResultApplication(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch}}
	a.searchMode = "all"
	a.searchEdit.SetText("搜索")
	a.searchList.Position.First = 4

	a.loadSearch()
	if got := a.searchList.Position.First; got != 4 {
		t.Fatalf("search list reset synchronously to %d", got)
	}

	waitForIdle(t, a)
	if got := a.searchList.Position.First; got != 0 {
		t.Fatalf("search list position after applying result = %d, want 0", got)
	}
}

func TestSearchFailureKeepsAuthenticatedSearchRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/section.json":
			io.WriteString(w, `{"section":[{"name":"0"}]}`)
		case "/open/section/0.json":
			io.WriteString(w, `{"name":"0","board":[{"name":"Go"}]}`)
		case "/open/search/article.json":
			http.Error(w, "search unavailable", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch}}
	a.searchMode = "all"
	a.searchEdit.SetText("搜索")

	a.loadSearch()
	waitForIdle(t, a)

	if got := a.current().Page; got != PageSearch {
		t.Fatalf("ordinary search error changed page to %v", got)
	}
	if !a.failed || a.message == "" {
		t.Fatalf("search failure was not exposed: failed=%t message=%q", a.failed, a.message)
	}
	if got := a.core.Session().Token; got != "token" {
		t.Fatalf("ordinary search error discarded token %q", got)
	}
}

func TestBottomNavigationComposeAndMessages(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	a.click("nav-search").Click()
	ops.Reset()
	a.Layout(gtx)
	if got := a.current().Page; got != PageSearch {
		t.Fatalf("search nav page = %v, want search", got)
	}
	a.click("nav-home").Click()
	ops.Reset()
	a.Layout(gtx)
	if got := a.current().Page; got != PageHome {
		t.Fatalf("section nav page = %v, want home sections", got)
	}
	waitForIdle(t, a)

	a.click("nav-compose").Click()
	ops.Reset()
	a.Layout(gtx)
	if got := a.current().Page; got != PageCompose {
		t.Fatalf("compose nav page = %v, want compose", got)
	}
	if a.compose.boardLocked {
		t.Fatal("compose started with a locked board")
	}

	a.stack = []Route{{Page: PageHome}}
	a.click("nav-messages").Click()
	ops.Reset()
	a.Layout(gtx)
	if got := a.current().Page; got != PageMessages {
		t.Fatalf("messages nav page = %v, want messages", got)
	}
	if got := a.current().Sub; got != "contacts" {
		t.Fatalf("messages tab = %q, want contacts", got)
	}
}

func TestTopTenAndRecommendationsLoad(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/widget/topten.json":
			_, _ = w.Write([]byte(`{"article":[{"id":42,"topic_id":42,"board_name":"Go","title":"十大主题","user":{"id":"alice","user_name":"Alice"},"num":3}]}`))
		case "/open/widget/recommend.json":
			_, _ = w.Write([]byte(`{"article":[{"id":43,"topic_id":43,"board_name":"Go","title":"推荐主题","user":{"id":"bob","user_name":"Bob"},"num":1}]}`))
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	defer server.Close()

	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageTop}}
	a.topTab = "top"
	a.loadTop()
	waitForIdle(t, a)
	if len(a.articles) != 1 || a.articles[0].Title != "十大主题" {
		t.Fatalf("top ten articles = %+v", a.articles)
	}

	a.selectTopTab("recommend")
	waitForIdle(t, a)
	if len(a.articles) != 1 || a.articles[0].Title != "推荐主题" {
		t.Fatalf("recommendation articles = %+v", a.articles)
	}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	if dims := a.topPage(gtx); dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("top page rendered blank: %+v", dims.Size)
	}
}
