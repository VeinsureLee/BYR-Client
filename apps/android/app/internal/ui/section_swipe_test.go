package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestSectionSwipeDirectionRequiresHorizontalMovement(t *testing.T) {
	if got := sectionSwipeDirection(-100, 12, 64); got != 1 {
		t.Fatalf("left swipe direction = %d, want next-module +1", got)
	}
	if got := sectionSwipeDirection(100, 12, 64); got != -1 {
		t.Fatalf("right swipe direction = %d, want previous-module -1", got)
	}
	for _, tc := range []struct {
		dx, dy float32
	}{
		{20, 0},
		{40, 80},
		{-70, 100},
	} {
		if got := sectionSwipeDirection(tc.dx, tc.dy, 64); got != 0 {
			t.Fatalf("movement (%v,%v) direction = %d, want no module change", tc.dx, tc.dy, got)
		}
	}
}

func TestSectionModuleNavigationKeepsOneRoutePerDestination(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.sectionTab = "all"

	a.shiftSectionModule(1)
	waitForIdle(t, a)
	if got := a.current().Page; got != PageHome || a.sectionTab != "favorites" {
		t.Fatalf("all -> favorites swipe = route:%+v sectionTab:%q", a.current(), a.sectionTab)
	}
	a.shiftSectionModule(1)
	waitForIdle(t, a)
	if got := a.current().Page; got != PageTimeline {
		t.Fatalf("favorites -> timeline swipe = %+v", a.current())
	}
	a.shiftSectionModule(1)
	waitForIdle(t, a)
	if got := a.current(); got.Page != PageTop || got.Name != "" {
		t.Fatalf("timeline -> Today's Top swipe = %+v", got)
	}
	a.shiftSectionModule(1)
	waitForIdle(t, a)
	if got := a.current(); got.Page != PageTop || got.Name != "recommend" {
		t.Fatalf("Today's Top -> recommendations swipe = %+v", got)
	}
	if len(a.stack) != 2 {
		t.Fatalf("swiping accumulated module routes: %+v", a.stack)
	}
	a.shiftSectionModule(1)
	if got := a.currentSectionModule(); got != "recommend" {
		t.Fatalf("right boundary changed module to %q", got)
	}
}

func TestSectionPagePhysicalSwipeChangesModule(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.sectionTab = "all"

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 760)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	router.Frame(&ops)
	router.Queue(
		pointer.Event{Source: pointer.Touch, Kind: pointer.Press, PointerID: 1, Position: f32.Pt(320, 420)},
		pointer.Event{Source: pointer.Touch, Kind: pointer.Move, PointerID: 1, Position: f32.Pt(210, 430)},
		pointer.Event{Source: pointer.Touch, Kind: pointer.Release, PointerID: 1, Position: f32.Pt(210, 430)},
	)
	ops.Reset()
	gtx.Ops = &ops
	a.Layout(gtx)
	if got := a.currentSectionModule(); got != "favorites" {
		t.Fatalf("physical left swipe selected module %q, want favorites", got)
	}
}
