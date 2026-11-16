package content

import "testing"

func TestEditorSelectionUsesUnicodeRunes(t *testing.T) {
	v, start, end := WrapSelection("甲😀乙", 1, 2, "b")
	if v != "甲[b]😀[/b]乙" || start != 4 || end != 5 {
		t.Fatalf("%s %d %d", v, start, end)
	}
	v, caret := Insert("甲😀乙", 2, 1, "[em1]")
	if v != "甲[em1]乙" || caret != 6 {
		t.Fatalf("%s %d", v, caret)
	}
}
