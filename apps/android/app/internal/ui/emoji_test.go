package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestEmbeddedEmojiLoads(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	entry := a.imageEntry("emoji:em15.gif")
	deadline := time.Now().Add(3 * time.Second)
	for entry.loading && time.Now().Before(deadline) {
		a.drain()
		time.Sleep(5 * time.Millisecond)
	}
	a.drain()
	if entry.loading || entry.err != nil {
		t.Fatalf("emoji did not load: loading=%v err=%v", entry.loading, entry.err)
	}
	if entry.img == nil || entry.w == 0 || entry.h == 0 {
		t.Fatalf("emoji decoded empty: %+v", entry)
	}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(200, 100)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	dims := a.inlineEmoji(gtx, "emoji:em15.gif", "[em15]")
	if dims.Size != image.Pt(24, 24) {
		t.Fatalf("emoji dimensions = %v, want 24x24", dims.Size)
	}
}
