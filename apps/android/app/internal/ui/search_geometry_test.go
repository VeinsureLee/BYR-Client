package ui

import (
	"image"
	"math"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestSearchButtonAndModeTabTextUseMeasuredCenter(t *testing.T) {
	a := previewApp(t)
	defer a.Close()

	for _, width := range []int{320, 360, 420} {
		for _, scale := range []float32{1, 1.3} {
			t.Run(itoa(width)+"-font-"+itoa(int(scale*100)), func(t *testing.T) {
				var ops op.Ops
				gtx := layout.Context{
					Ops:         &ops,
					Constraints: layout.Exact(image.Pt(width, 48)),
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: scale},
				}
				buttonSize := image.Pt(width, gtx.Dp(unit.Dp(48)))
				for _, label := range []string{"查找人物", "开始全站搜索"} {
					ops.Reset()
					text, _ := a.measureButtonLabel(gtx, label, a.theme.ContrastFg, buttonSize)
					offset := centeredTextOffset(buttonSize, text.Size)
					if math.Abs(float64(offset.X+text.Size.X/2-buttonSize.X/2)) > 2 ||
						math.Abs(float64(offset.Y+text.Size.Y/2-buttonSize.Y/2)) > 2 {
						t.Fatalf("button %q text box %v at %v is not centered in %v", label, text.Size, offset, buttonSize)
					}
				}
				for _, label := range []string{"全站搜索", "人物"} {
					ops.Reset()
					text, _ := a.measureModuleTabLabel(gtx, label, false)
					tabSize := moduleTabSize(gtx, text.Size)
					offset := centeredTextOffset(tabSize, text.Size)
					if tabSize.Y < gtx.Dp(unit.Dp(48)) ||
						math.Abs(float64(offset.X+text.Size.X/2-tabSize.X/2)) > 2 ||
						math.Abs(float64(offset.Y+text.Size.Y/2-tabSize.Y/2)) > 2 {
						t.Fatalf("tab %q text box %v at %v is not centered in %v", label, text.Size, offset, tabSize)
					}
				}

				accessory := gtx.Dp(unit.Dp(searchClearAccessoryDp))
				for _, hasClear := range []bool{false, true} {
					want := width
					if hasClear {
						want = max(0, width-accessory)
					}
					if got := searchActionWidth(width, accessory, hasClear); got != want {
						t.Fatalf("button width with clear=%t is %d, want input width %d", hasClear, got, want)
					}
				}
			})
		}
	}
}
