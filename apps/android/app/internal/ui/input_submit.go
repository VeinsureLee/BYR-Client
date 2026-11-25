package ui

import (
	"io"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

// singleLineIntent separates editing from an explicit submit action for a
// focused single-line editor. Gio's Submit flag treats a line break in a text
// edit as a submit and truncates the suffix, so the explicit Enter/Return key
// is consumed here instead. All text edits are then applied before the caller
// reads the value. A multi-line paste is normalized for general single-line
// fields or rejected for policy fields such as passwords.
type singleLineIntent struct {
	Submitted     bool
	RejectedPaste bool
}

// singleLineBatch replays the input events of one frame in their original
// order. The editor applies edits itself only when this code has decided the
// final text for that edit; an editor batch is not opaque, so a selection
// event before a Right key and a selection event after one stay distinct.
type singleLineBatch struct {
	raw       []rune
	rebuilt   []rune
	positions []int

	selectionStart, selectionEnd int

	haveEdit      bool
	editStart     int
	editEnd       int
	editRejected  bool
	defaultSelect bool

	rejectedAny bool
	appliedAny  bool
	rightUsed   bool
	touched     bool
}

func (a *App) applySingleLineIntent(gtx layout.Context, editor *widget.Editor, rejectLineBreaks bool) singleLineIntent {
	var intent singleLineIntent
	for {
		event, ok := gtx.Event(
			key.Filter{Focus: editor, Name: key.NameEnter, Optional: key.ModShift},
			key.Filter{Focus: editor, Name: key.NameReturn, Optional: key.ModShift},
		)
		if !ok {
			break
		}
		if ke, ok := event.(key.Event); ok && ke.State == key.Press {
			intent.Submitted = true
		}
	}
	before := editor.Text()
	selectionStart, selectionEnd := editor.Selection()

	wasSingleLine := editor.SingleLine
	// Keep line breaks visible to this helper while it applies the batch. The
	// editor itself never renders the intermediate value because the frame is
	// normalized before material.Editor lays it out.
	editor.SingleLine = false
	batch, ok := runSingleLineBatch(gtx, editor, before, selectionStart, selectionEnd, rejectLineBreaks)
	editor.SingleLine = wasSingleLine

	if !ok {
		drainEditorUpdates(gtx, editor)
		return legacySingleLineFallback(editor, before, selectionStart, selectionEnd, rejectLineBreaks, intent)
	}
	if batch.rejectedAny {
		intent.Submitted = false
		intent.RejectedPaste = true
	}
	if batch.touched {
		if batch.rejectedAny && !batch.appliedAny && !batch.rightUsed && batch.defaultSelect {
			// A rejected paste that ended in its edit-default state restores the
			// selection the user had before the pasted text arrived.
			editor.SetCaret(selectionStart, selectionEnd)
		} else {
			start := clampIndex(batch.selectionStart, len(batch.positions)-1)
			end := clampIndex(batch.selectionEnd, len(batch.positions)-1)
			editor.SetCaret(batch.positions[start], batch.positions[end])
		}
	}
	drainEditorUpdates(gtx, editor)
	return intent
}

func runSingleLineBatch(gtx layout.Context, editor *widget.Editor, before string, selectionStart, selectionEnd int, rejectLineBreaks bool) (*singleLineBatch, bool) {
	batch := &singleLineBatch{
		raw:            []rune(before),
		rebuilt:        []rune(before),
		selectionStart: selectionStart,
		selectionEnd:   selectionEnd,
	}
	batch.positions = make([]int, len(batch.raw)+1)
	for i := range batch.positions {
		batch.positions[i] = i
	}
	for {
		event, ok := gtx.Event(
			key.FocusFilter{Target: editor},
			key.Filter{Focus: editor, Name: key.NameRightArrow},
			transfer.TargetFilter{Target: editor, Type: "application/text"},
		)
		if !ok {
			break
		}
		switch event := event.(type) {
		case key.EditEvent:
			if !batch.applyReplacement(editor, event.Range.Start, event.Range.End, event.Text, rejectLineBreaks) {
				return nil, false
			}
		case key.SelectionEvent:
			batch.applySelection(event)
		case transfer.DataEvent:
			reader := event.Open()
			data, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				continue
			}
			if !batch.applyReplacement(editor, batch.selectionStart, batch.selectionEnd, string(data), rejectLineBreaks) {
				return nil, false
			}
		case key.Event:
			if event.Name == key.NameRightArrow && event.State == key.Press {
				batch.applyRight()
			}
		case key.FocusEvent, key.SnippetEvent, key.CompositionEvent:
			// These do not carry text, but the editor keeps IME state for
			// them. Replay them through a private router so composition and
			// snippet bookkeeping stays in event order.
			forwardEditorEvent(gtx, editor, event)
		}
	}
	return batch, true
}

// applyReplacement records one edit or clipboard paste in the raw coordinate
// space used by the input method and applies the text chosen for the
// single-line value. Rejected password pastes leave no history entry;
// normalized search pastes become one undoable replacement.
func (b *singleLineBatch) applyReplacement(editor *widget.Editor, rangeStart, rangeEnd int, text string, rejectLineBreaks bool) bool {
	start, end := rangeStart, rangeEnd
	if start > end {
		start, end = end, start
	}
	start = clampIndex(start, len(b.raw))
	end = clampIndex(end, len(b.raw))
	reverse := append([]rune(nil), b.raw[start:end]...)
	applyRunes := []rune(text)

	chosen := applyRunes
	invalid := containsLineBreak(text)
	rejected := invalid && rejectLineBreaks
	var boundaries []int
	switch {
	case rejected:
		chosen = reverse
		boundaries = linearBoundaries(len(applyRunes), len(reverse))
	case invalid:
		normalized, mapped := normalizeSingleLinePasteWithBoundaries(text)
		chosen = []rune(normalized)
		boundaries = mapped
	default:
		boundaries = linearBoundaries(len(applyRunes), len(applyRunes))
	}

	mappedStart, mappedEnd := b.positions[start], b.positions[end]
	if mappedStart < 0 || mappedStart > len(b.rebuilt) || mappedEnd < mappedStart || mappedEnd > len(b.rebuilt) {
		return false
	}
	if !rejected {
		editor.SetCaret(mappedStart, mappedEnd)
		if inserted := editor.Insert(string(chosen)); inserted != len(chosen) {
			return false
		}
		b.appliedAny = true
	}

	newRaw := append([]rune(nil), b.raw[:start]...)
	newRaw = append(newRaw, applyRunes...)
	newRaw = append(newRaw, b.raw[end:]...)
	newRebuilt := append([]rune(nil), b.rebuilt[:mappedStart]...)
	newRebuilt = append(newRebuilt, chosen...)
	newRebuilt = append(newRebuilt, b.rebuilt[mappedEnd:]...)
	newPositions := make([]int, 0, len(newRaw)+1)
	newPositions = append(newPositions, b.positions[:start]...)
	for _, offset := range boundaries {
		newPositions = append(newPositions, mappedStart+offset)
	}
	shift := len(chosen) - (mappedEnd - mappedStart)
	for boundary := end + 1; boundary <= len(b.raw); boundary++ {
		newPositions = append(newPositions, b.positions[boundary]+shift)
	}
	if len(newPositions) != len(newRaw)+1 {
		return false
	}

	// Mirror how Gio adjusts a selection that is not part of the edited range:
	// a selection covering the edit becomes the inserted text, while a caret
	// elsewhere follows the shift.
	oldStart, oldEnd := b.selectionStart, b.selectionEnd
	newEnd := start + len(applyRunes)
	adjust := func(pos int) int {
		switch {
		case newEnd < pos && pos <= end:
			return newEnd
		case end < pos:
			return pos + newEnd - end
		default:
			return pos
		}
	}
	b.selectionStart, b.selectionEnd = adjust(oldStart), adjust(oldEnd)

	b.raw, b.rebuilt, b.positions = newRaw, newRebuilt, newPositions
	b.haveEdit = true
	b.editStart = start
	b.editEnd = newEnd
	b.editRejected = rejected
	b.rejectedAny = b.rejectedAny || rejected
	b.defaultSelect = selectionTouches(oldStart, oldEnd, start, end)
	b.touched = true
	return true
}

func (b *singleLineBatch) applySelection(event key.SelectionEvent) {
	b.selectionStart = clampIndex(event.Start, len(b.raw))
	b.selectionEnd = clampIndex(event.End, len(b.raw))
	b.defaultSelect = b.haveEdit && b.selectionIsEditDefault(b.selectionStart, b.selectionEnd)
	b.touched = true
}

func (b *singleLineBatch) selectionIsEditDefault(start, end int) bool {
	lo, hi := start, end
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo == b.editStart && hi == b.editEnd {
		return true
	}
	return lo == b.editEnd && hi == b.editEnd
}

// applyRight applies the consumed Right key at its actual position in the
// event stream. A later selection event therefore wins over an earlier Right,
// while a selection set before the Right is collapsed by it.
func (b *singleLineBatch) applyRight() {
	position := 0
	collapsed := b.selectionStart == b.selectionEnd
	switch {
	case b.haveEdit && b.defaultSelect && b.editRejected:
		// The rejected text never existed in the value, so the key collapses
		// the edit selection at the end of the restored text without moving.
		position = b.editEnd
	case b.haveEdit && b.defaultSelect:
		position = b.editEnd + 1
	case !collapsed:
		position = b.selectionStart
		if b.selectionEnd > position {
			position = b.selectionEnd
		}
	default:
		position = b.selectionStart + 1
	}
	position = clampIndex(position, len(b.raw))
	b.selectionStart, b.selectionEnd = position, position
	b.defaultSelect = false
	b.rightUsed = true
	b.touched = true
}

func selectionTouches(selectionStart, selectionEnd, editStart, editEnd int) bool {
	lo, hi := selectionStart, selectionEnd
	if lo > hi {
		lo, hi = hi, lo
	}
	return hi >= editStart && lo <= editEnd
}

func clampIndex(value, limit int) int {
	if value < 0 {
		return 0
	}
	if value > limit {
		return limit
	}
	return value
}

// forwardEditorEvent gives the editor an IME event that this helper does not
// interpret. The private router keeps the editor's composition and snippet
// bookkeeping in the same order as the text events around it.
func forwardEditorEvent(gtx layout.Context, editor *widget.Editor, ev event.Event) {
	var router input.Router
	var ops op.Ops
	temp := gtx
	temp.Ops = &ops
	temp.Source = router.Source()
	temp.Execute(key.FocusCmd{Tag: editor})
	for {
		if _, ok := editor.Update(temp); !ok {
			break
		}
	}
	router.Queue(ev)
	for {
		if _, ok := editor.Update(temp); !ok {
			break
		}
	}
}

// drainEditorUpdates lets the editor process events this helper does not own,
// such as a non-Right key pressed in the same frame, and re-registers its input
// state on the real source so IME commands reflect the final selection.
func drainEditorUpdates(gtx layout.Context, editor *widget.Editor) {
	for {
		if _, ok := editor.Update(gtx); !ok {
			break
		}
	}
}

// legacySingleLineFallback keeps the previous whole-frame behavior when an
// unexpected edit cannot be represented in the raw model.
func legacySingleLineFallback(editor *widget.Editor, before string, selectionStart, selectionEnd int, rejectLineBreaks bool, intent singleLineIntent) singleLineIntent {
	after := editor.Text()
	if rejectLineBreaks {
		editor.SetText(before)
		editor.SetCaret(selectionStart, selectionEnd)
		if containsLineBreak(after) {
			intent.Submitted = false
			intent.RejectedPaste = true
		}
		return intent
	}
	if containsLineBreak(after) {
		normalized, boundaries := normalizeSingleLinePasteWithBoundaries(after)
		start, end := editor.Selection()
		editor.SetText(normalized)
		editor.SetCaret(normalizePasteOffset(boundaries, start), normalizePasteOffset(boundaries, end))
	}
	return intent
}

func containsLineBreak(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

// normalizeSingleLinePaste collapses every CR, LF or CRLF run into one space.
func normalizeSingleLinePaste(s string) string {
	normalized, _ := normalizeSingleLinePasteWithBoundaries(s)
	return normalized
}

// normalizeSingleLinePasteWithBoundaries also returns a rune-boundary map from
// the original text to the normalized text. Callers use it to keep the caret
// and selection at the same logical position after SetText.
func normalizeSingleLinePasteWithBoundaries(s string) (string, []int) {
	runes := []rune(s)
	boundaries := make([]int, len(runes)+1)
	var b strings.Builder
	b.Grow(len(s))
	out := 0
	inLineBreak := false
	for i, r := range runes {
		boundaries[i] = out
		if r == '\r' || r == '\n' {
			if !inLineBreak {
				b.WriteByte(' ')
				out++
				inLineBreak = true
			}
			continue
		}
		b.WriteRune(r)
		out++
		inLineBreak = false
	}
	boundaries[len(runes)] = out
	return b.String(), boundaries
}

func normalizePasteOffset(boundaries []int, offset int) int {
	if len(boundaries) == 0 {
		return 0
	}
	if offset <= 0 {
		return 0
	}
	if offset >= len(boundaries) {
		return boundaries[len(boundaries)-1]
	}
	return boundaries[offset]
}

func linearBoundaries(rawLen, chosenLen int) []int {
	boundaries := make([]int, rawLen+1)
	for i := 0; i < rawLen; i++ {
		if i >= chosenLen {
			boundaries[i] = chosenLen
		} else {
			boundaries[i] = i
		}
	}
	// The end of the replaced range always maps to the end of the restored
	// text, even when the rejected paste was shorter than the selection.
	boundaries[rawLen] = chosenLen
	return boundaries
}
