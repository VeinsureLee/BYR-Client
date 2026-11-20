package ui

import "testing"

func TestSearchActionWidthMatchesInputWithClearControl(t *testing.T) {
	for _, width := range []int{320, 420} {
		const clearSlot = 46
		got := searchActionWidth(width, clearSlot, true)
		want := width - clearSlot
		if got != want {
			t.Fatalf("search action width at %dp = %d, want input width %d", width, got, want)
		}
		if got := searchActionWidth(width, clearSlot, false); got != width {
			t.Fatalf("search action width without clear control at %dp = %d, want %d", width, got, width)
		}
	}
}
