package ui

import (
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestSearchPageDoesNotCallForumYet(t *testing.T) {
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
		t.Fatalf("search page issued forum requests: %+v", s.requests)
	}
	if !strings.Contains(a.message, "尚未接入") {
		t.Fatalf("search placeholder notice = %q", a.message)
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
