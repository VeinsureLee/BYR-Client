package desktop

import (
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDesktopEmptyStateCentersInsideRemainingViewport(t *testing.T) {
	const title = "还没有阅读记录"
	const detail = "打开主题后会在这里留下最近阅读记录。"
	const headerHeight = float32(64)
	for _, size := range [][2]int{{720, 480}, {1100, 760}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			view := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Fill().Children(func() {
					ui.Box(c).FillWidth().Height(headerHeight)
					emptyState(c, "clock", title, detail)
				})
			}, size[0], size[1])
			view.Frame()
			titleRect, ok := view.Find(title)
			if !ok {
				t.Fatal("empty-state title is missing")
			}
			detailRect, ok := view.Find(detail)
			if !ok {
				t.Fatal("empty-state detail is missing")
			}
			// The icon and the first gap precede the title in the centered block.
			blockTop := titleRect.Y - 40
			blockBottom := detailRect.Y + detailRect.H
			blockCenter := (blockTop + blockBottom) / 2
			viewportCenter := headerHeight + float32(size[1]-int(headerHeight))/2
			if math.Abs(float64(blockCenter-viewportCenter)) > .5 {
				t.Fatalf("empty content center y=%.1f, viewport center y=%.1f (title=%+v detail=%+v)",
					blockCenter, viewportCenter, titleRect, detailRect)
			}
		})
	}
}
