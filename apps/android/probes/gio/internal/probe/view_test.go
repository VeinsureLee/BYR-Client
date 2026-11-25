package probe

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestOfflineProbeLoadsChineseFontAndUsesVirtualList(t *testing.T) {
	view, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if view.Theme.Face == "" {
		t.Fatal("bundled font not initialized")
	}
	var operations op.Ops
	context := layout.Context{Ops: &operations, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(390, 844))}
	dimensions := view.Layout(context)
	if dimensions.Size.X == 0 || dimensions.Size.Y == 0 {
		t.Fatal("probe produced an empty layout")
	}
	if view.LastLayoutCount == 0 || view.LastLayoutCount >= TotalRows/2 {
		t.Fatalf("long list was not built lazily: %d", view.LastLayoutCount)
	}
	if view.Editor.Text() == "" {
		t.Fatal("input probe is missing")
	}
}
