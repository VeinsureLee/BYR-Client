package ui

import (
	"image"
	"net/http"
	"net/http/httptest"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestBottomNavigationOpensSearch(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.click("nav-search").Click()

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	if got := a.current().Page; got != PageSearch {
		t.Fatalf("page = %v, want search", got)
	}
	ops.Reset()
	a.Layout(gtx)
	if got := a.current().Page; got != PageSearch {
		t.Fatalf("search page did not survive a frame: %v", got)
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
