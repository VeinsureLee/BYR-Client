package ui

import (
	"image"
	"testing"

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
	a.stack = []Route{{Page: PageHome}}
	a.setTab(PageSelf)
	a.openBoard("Go")
	if a.Back() {
		t.Fatal("Back reported success while a request was active")
	}
	if got := a.current().Page; got != PageHome {
		t.Fatalf("busy navigation changed page to %v", got)
	}
	if len(a.stack) != 1 {
		t.Fatalf("busy navigation changed route stack: %+v", a.stack)
	}
}
