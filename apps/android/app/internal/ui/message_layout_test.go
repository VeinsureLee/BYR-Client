package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestEmptyReminderOccupiesBodyViewportForCentering(t *testing.T) {
	a := previewApp(t)
	defer a.Close()
	a.stack = []Route{{Page: PageMessages, Sub: "reply"}}
	a.refers.Articles = nil
	a.busy, a.failed = false, false

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(320, 420)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	dims := a.refersBody(gtx)
	if dims.Size != image.Pt(320, 420) {
		t.Fatalf("empty reminder dimensions = %v, want full body viewport 320x420", dims.Size)
	}
}
