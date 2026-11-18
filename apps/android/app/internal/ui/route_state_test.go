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
