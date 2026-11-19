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

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func TestBackRestoresThePoppedRouteModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/board/Go.json":
			_, _ = w.Write([]byte(`{"name":"Go","description":"Go board"}`))
		case "/open/threads/Go/42.json":
			_, _ = w.Write([]byte(`{"id":42,"title":"Thread A","board_name":"Go","article":[{"id":42,"content":"A"}]}`))
		case "/open/threads/Go/43.json":
			_, _ = w.Write([]byte(`{"id":43,"title":"Thread B","board_name":"Go","article":[{"id":43,"content":"B"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openThread("Go", "42")
	waitForIdle(t, a)
	a.openBoard("Go")
	waitForIdle(t, a)
	a.openThread("Go", "43")
	waitForIdle(t, a)
	if a.thread.Title != "Thread B" {
		t.Fatalf("thread B did not load: %+v", a.thread)
	}

	a.back()
	waitForIdle(t, a)
	if got := a.current().Page; got != PageBoard || a.board.Name != "Go" {
		t.Fatalf("back to board left stale model: route=%+v board=%+v", a.current(), a.board)
	}
	a.back()
	waitForIdle(t, a)
	if got := a.current(); got.Page != PageThread || got.ID != "42" || a.thread.Title != "Thread A" {
		t.Fatalf("back to thread A left stale model: route=%+v thread=%+v", got, a.thread)
	}
}

func TestThreadLoadFailureRendersRetryInsteadOfEmptyState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"code":"500","msg":"temporary failure"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	a := interactionApp(t, server.URL)
	defer a.Close()
	a.openThread("Go", "42")
	waitForIdle(t, a)
	if !a.failed || a.message == "" {
		t.Fatalf("thread failure was not retained: failed=%v message=%q", a.failed, a.message)
	}

	var router input.Router
	var ops op.Ops
	dims := a.threadPage(layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("retry page rendered blank: %+v", dims.Size)
	}
	if a.clicks["retry-page"] == nil {
		t.Fatal("retry action was not rendered for the failed thread request")
	}
}

func TestMobileListStateIsolatedByRoute(t *testing.T) {
	a := startAppForTest(t)
	defer a.Close()
	first := a.mobileList(listKey{Page: PageBoard, Name: "Go", Number: 1})
	first.Position.First = 7
	first.Position.Offset = 13
	second := a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "42", Number: 1})
	if first == second {
		t.Fatal("board and thread shared mobile list state")
	}
	got := a.mobileList(listKey{Page: PageBoard, Name: "Go", Number: 1})
	if got.Position.First != 7 || got.Position.Offset != 13 {
		t.Fatalf("board position = %+v", got.Position)
	}
}

func TestBoardBackRestoresScrollPosition(t *testing.T) {
	a := startAppForTest(t)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}, {Page: PageBoard, Name: "Go", Number: 1}}
	a.board = testBoardWithArticles(3)
	key := listKey{Page: PageBoard, Name: "Go", Number: 1}
	list := a.mobileList(key)
	// An out-of-range position makes this test fail if boardPage still renders
	// the legacy singleton boardList instead of the route-owned list.
	list.Position.First = 99
	renderAndroidPage(t, func(gtx layout.Context) layout.Dimensions { return a.boardPage(gtx) })
	if list.Position.First == 99 {
		t.Fatal("board page did not use route-owned list state")
	}

	list.Position.First = 1
	list.Position.Offset = 13
	a.stack = append(a.stack, Route{Page: PageThread, Name: "Go", ID: "42", Number: 1})
	a.back()
	got := a.mobileList(key).Position
	if got.First != 1 || got.Offset != 13 {
		t.Fatalf("board position after back = %+v", got)
	}
}

func TestThreadPositionsAreIsolatedByThreadRoute(t *testing.T) {
	a := startAppForTest(t)
	defer a.Close()
	a.stack = []Route{{Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	a.thread = testThreadWithArticles(1)
	first := a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "42", Number: 1})
	first.Position.First = 99
	renderAndroidPage(t, func(gtx layout.Context) layout.Dimensions { return a.threadPage(gtx) })
	if first.Position.First == 99 {
		t.Fatal("thread page did not use route-owned list state")
	}
	first.Position.First = 4

	second := a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "43", Number: 1})
	second.Position.First = 9
	if first == second {
		t.Fatal("different thread routes shared mobile list state")
	}
	if got := a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "42", Number: 1}).Position.First; got != 4 {
		t.Fatalf("thread 42 position = %d", got)
	}
	if got := a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "43", Number: 1}).Position.First; got != 9 {
		t.Fatalf("thread 43 position = %d", got)
	}
}

func TestPageChangeStartsAtTopAndRestoresPreviousPosition(t *testing.T) {
	a := startAppForTest(t)
	defer a.Close()
	a.stack = []Route{{Page: PageBoard, Name: "Go", Number: 1}}
	pageOne := a.mobileList(listKey{Page: PageBoard, Name: "Go", Number: 1})
	pageOne.Position.First = 5
	pageOne.Position.Offset = 11

	// Avoid a network request; the route number and keyed state are the
	// contract under test here.
	a.busy = true
	a.changePage(2)
	a.busy = false
	if got := a.current().Number; got != 2 {
		t.Fatalf("page after change = %d, want 2", got)
	}
	pageTwo := a.mobileList(listKey{Page: PageBoard, Name: "Go", Number: 2})
	if pageTwo.Position.First != 0 || pageTwo.Position.Offset != 0 {
		t.Fatalf("new page position = %+v, want top", pageTwo.Position)
	}
	pageTwo.Position.First = 99
	a.board = testBoardWithArticles(1)
	renderAndroidPage(t, func(gtx layout.Context) layout.Dimensions { return a.boardPage(gtx) })
	if pageTwo.Position.First == 99 {
		t.Fatal("page two was not rendered with its keyed list state")
	}

	a.stack[len(a.stack)-1].Number = 1
	got := a.mobileList(listKey{Page: PageBoard, Name: "Go", Number: 1}).Position
	if got.First != 5 || got.Offset != 11 {
		t.Fatalf("page one position after returning = %+v", got)
	}
}

func testBoardWithArticles(n int) forum.Board {
	articles := make([]forum.Article, n)
	for i := range articles {
		articles[i] = forum.Article{ID: forum.Scalar(itoa(i + 1)), Board: "Go", Title: "主题"}
	}
	return forum.Board{Name: "Go", Description: "Go", Articles: articles}
}

func testThreadWithArticles(n int) forum.Thread {
	articles := make([]forum.Article, n)
	for i := range articles {
		articles[i] = forum.Article{ID: forum.Scalar(itoa(i + 1)), Board: "Go", Content: "正文"}
	}
	return forum.Thread{ID: "42", Board: "Go", Title: "主题", Articles: articles}
}

func renderAndroidPage(t *testing.T, page func(layout.Context) layout.Dimensions) {
	t.Helper()
	var router input.Router
	var ops op.Ops
	dims := page(layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("page rendered blank: %+v", dims.Size)
	}
}
