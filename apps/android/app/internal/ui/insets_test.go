package ui

import (
	"image"
	"testing"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"
)

func TestFrameInsetsReduceUsableViewport(t *testing.T) {
	event := app.FrameEvent{
		Size:   image.Pt(1080, 2400),
		Insets: app.Insets{Top: unit.Dp(24), Bottom: unit.Dp(42)},
		Metric: unit.Metric{PxPerDp: 3, PxPerSp: 3},
	}
	gtx := layoutWithFrameInsets(event)
	want := image.Pt(1080, 2400-24*3-42*3)
	if got := gtx.Constraints.Max; got != want {
		t.Fatalf("safe viewport=%v, want %v", got, want)
	}

	body := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
	)
	if body.Size.Y > want.Y {
		t.Fatalf("body height=%d exceeds inset-reduced max height=%d", body.Size.Y, want.Y)
	}
}
