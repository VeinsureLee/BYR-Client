package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestSystemBackEventPopsInnerRoute(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.ready = true
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	router.Frame(&ops)
	router.Queue(input.SystemEvent{Event: key.Event{Name: key.NameBack, State: key.Press}})
	ops.Reset()
	gtx.Ops = &ops
	a.Layout(gtx)
	if got := a.current().Page; got != PageHome {
		t.Fatalf("system back page = %v, want home", got)
	}
}

func TestNavigationDoesNotChangeRouteWhileBusy(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.ready = true
	a.busy = true
	a.stack = []Route{{Page: PageHome}, {Page: PageBoard, Name: "Go", Number: 1}}
	if !a.Back() {
		t.Fatal("Back should consume the gesture while a request is active")
	}
	if got := a.current().Page; got != PageBoard {
		t.Fatalf("busy navigation changed page to %v", got)
	}
	if len(a.stack) != 2 {
		t.Fatalf("busy navigation changed route stack: %+v", a.stack)
	}
}

func TestLoadingIndicatorIsDelayed(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.busy = true
	a.busySince = time.Now()
	if a.showLoading() {
		t.Fatal("loading indicator appeared immediately")
	}
	a.busySince = time.Now().Add(-600 * time.Millisecond)
	if !a.showLoading() {
		t.Fatal("loading indicator stayed hidden after the delay")
	}
	a.busy = false
	if a.showLoading() {
		t.Fatal("loading indicator remained after the request finished")
	}
}

func TestTabRootBackReturnsSearch(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.ready = true
	a.stack = []Route{{Page: PageTimeline}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	router.Frame(&ops)
	router.Queue(input.SystemEvent{Event: key.Event{Name: key.NameBack, State: key.Press}})
	ops.Reset()
	gtx.Ops = &ops
	a.Layout(gtx)
	if got := a.current().Page; got != PageSearch {
		t.Fatalf("tab-root back page = %v, want search", got)
	}
}
