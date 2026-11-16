package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestRenderPanicRecoversOnNextFrame(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.started = true
	a.ready = true
	// Self without a core cannot render; this forces the guarded panic path.
	a.stack = []Route{{Page: PageSelf}}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	if a.renderPanic == "" {
		t.Fatal("render panic was not recorded")
	}
	ops.Reset()
	gtx.Ops = &ops
	a.Layout(gtx)
	if a.renderPanic == "" {
		t.Fatal("recovery frame cleared the diagnostic before the user acted")
	}
	if got := a.current().Page; got != PageSelf {
		t.Fatalf("recovery frame changed the route: %v", got)
	}
	a.click("recover-login").Click()
	ops.Reset()
	gtx.Ops = &ops
	a.Layout(gtx)
	ops.Reset()
	gtx.Ops = &ops
	a.Layout(gtx)
	if a.renderPanic != "" {
		t.Fatalf("recovery action left a panic behind: %q", a.renderPanic)
	}
	if got := a.current().Page; got != PageLogin {
		t.Fatalf("recovery action page = %v, want login", got)
	}
}
