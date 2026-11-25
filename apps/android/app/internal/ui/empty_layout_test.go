package ui

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestEmptyStateFillsTheRemainingContentViewport(t *testing.T) {
	a := previewApp(t)
	defer a.Close()

	for _, size := range [][2]int{{320, 420}, {360, 420}, {420, 700}} {
		for _, scale := range []float32{1, 1.5} {
			t.Run(itoa(size[0])+"x"+itoa(size[1])+"-font-"+itoa(int(scale*100)), func(t *testing.T) {
				var ops op.Ops
				gtx := layout.Context{
					Ops:         &ops,
					Constraints: layout.Exact(image.Pt(size[0], size[1])),
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: scale},
				}
				var emptySize image.Point
				dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(size[0], 56)}
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						dims := a.empty(gtx, "还没有阅读记录")
						emptySize = dims.Size
						return dims
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(size[0], 64)}
					}),
				)
				wantBody := size[1] - 56 - 64
				if dims.Size != image.Pt(size[0], size[1]) || emptySize != image.Pt(size[0], wantBody) {
					t.Fatalf("empty viewport size=%v body=%v, want window=%v body=%v", dims.Size, emptySize, size, image.Pt(size[0], wantBody))
				}
			})
		}
	}
}
