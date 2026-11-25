package ui

import (
	"encoding/base64"
	"image"
	"net/url"
	"strings"
	"testing"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

type submitFixture struct {
	app    *App
	router input.Router
	gtx    layout.Context
}

func newSubmitFixture(t *testing.T, a *App) *submitFixture {
	t.Helper()
	f := &submitFixture{app: a}
	f.gtx = layout.Context{
		Ops:         &op.Ops{},
		Source:      f.router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	f.frame()
	return f
}

func (f *submitFixture) frame() {
	f.gtx.Ops.Reset()
	f.app.Layout(f.gtx)
	f.router.Frame(f.gtx.Ops)
}

func (f *submitFixture) focus(tag event.Tag) {
	f.gtx.Execute(key.FocusCmd{Tag: tag})
	f.frame()
}

func (f *submitFixture) queue(events ...event.Event) {
	f.router.Queue(events...)
	f.frame()
}

func (f *submitFixture) typeAndEnter(editor *widget.Editor, text string, presses int) {
	events := []event.Event{key.EditEvent{Text: text}}
	for i := 0; i < presses; i++ {
		events = append(events, key.Event{Name: key.NameEnter, State: key.Press})
	}
	f.queue(events...)
}

func TestAndroidPasswordEnterAppliesEditAndSubmitsOnce(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageLogin}}
	a.username.SetText("alice")

	f := newSubmitFixture(t, a)
	f.focus(&a.password)
	f.typeAndEnter(&a.password, "LineA", 2)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		a.drain()
		if len(server.find("POST", "/oauth2/official")) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	forms := server.find("POST", "/oauth2/official")
	if len(forms) != 1 {
		t.Fatalf("Enter produced %d login requests, want 1", len(forms))
	}
	values, err := url.ParseQuery(forms[0].Form)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(values.Get("password"))
	if err != nil || string(decoded) != "LineA" {
		t.Fatalf("login form password = %q decoded=%q error=%v", values.Get("password"), decoded, err)
	}
}

func TestAndroidSearchEnterSubmitsOnceAndPlainPasteDoesNot(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"

	f := newSubmitFixture(t, a)
	f.focus(&a.searchEdit)
	f.router.Queue(key.EditEvent{Text: "Go\nTopic"})
	f.frame()
	if got := a.searchEdit.Text(); got != "Go Topic" {
		t.Fatalf("multi-line search paste = %q, want normalized single line", got)
	}
	if len(server.find("GET", "/open/search/article.json")) != 0 {
		t.Fatal("multi-line paste submitted a search")
	}

	a.searchEdit.SetText("")
	f.typeAndEnter(&a.searchEdit, " 北邮 ", 2)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		a.drain()
		if len(server.find("GET", "/open/search/article.json")) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	requests := server.find("GET", "/open/search/article.json")
	if len(requests) != 1 {
		t.Fatalf("Enter produced %d search requests, want 1", len(requests))
	}
	if !strings.Contains(requests[0].Query, "title1=%E5%8C%97%E9%82%AE") {
		t.Fatalf("search request query = %q", requests[0].Query)
	}
}

func TestAndroidSearchButtonSubmitsLatestSingleLineValueOnce(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"

	f := newSubmitFixture(t, a)
	f.focus(&a.searchEdit)
	f.queue(key.EditEvent{Text: "single line"})
	if got := a.searchEdit.Text(); got != "single line" {
		t.Fatalf("single-line paste = %q", got)
	}
	if len(server.find("GET", "/open/search/article.json")) != 0 {
		t.Fatal("plain single-line paste submitted a search")
	}
	a.click("search-submit").Click()
	f.frame()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		a.drain()
		if len(server.find("GET", "/open/search/article.json")) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	requests := server.find("GET", "/open/search/article.json")
	if len(requests) != 1 {
		t.Fatalf("button produced %d search requests, want 1", len(requests))
	}
	if !strings.Contains(requests[0].Query, "title1=single+line") {
		t.Fatalf("button search query = %q", requests[0].Query)
	}
}

func TestAndroidPasswordRejectsMultiLinePasteAndKeepsOriginal(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageLogin}}
	a.username.SetText("alice")
	a.password.SetText("existing")

	f := newSubmitFixture(t, a)
	f.focus(&a.password)
	f.queue(key.EditEvent{Range: key.Range{Start: 0, End: len([]rune("existing"))}, Text: "LineA\nLineB"})
	if got := a.password.Text(); got != "existing" {
		t.Fatalf("multi-line password paste changed value to %q", got)
	}
	if !a.failed || !strings.Contains(a.message, "换行") {
		t.Fatalf("multi-line password paste feedback: failed=%t message=%q", a.failed, a.message)
	}
	if len(server.find("POST", "/oauth2/official")) != 0 {
		t.Fatal("multi-line password paste submitted a login")
	}
}

func TestPasswordRejectedPasteRestoresTextAndSelection(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageLogin}}
	f := newSubmitFixture(t, a)
	f.focus(&a.password)

	for _, tc := range []struct {
		name      string
		paste     string
		selStart  int
		selEnd    int
		wantStart int
		wantEnd   int
	}{
		{name: "end CRLF", paste: "LineA\r\nLineB", selStart: 8, selEnd: 8, wantStart: 8, wantEnd: 8},
		{name: "end LF", paste: "LineA\nLineB", selStart: 8, selEnd: 8, wantStart: 8, wantEnd: 8},
		{name: "end CR", paste: "LineA\rLineB", selStart: 8, selEnd: 8, wantStart: 8, wantEnd: 8},
		{name: "replace selection", paste: "LineA\r\nLineB", selStart: 0, selEnd: 8, wantStart: 0, wantEnd: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a.password.SetText("existing")
			a.password.SetCaret(tc.selStart, tc.selEnd)
			rawCaret := tc.selStart + len([]rune(tc.paste))
			f.queue(
				key.EditEvent{Range: key.Range{Start: tc.selStart, End: tc.selEnd}, Text: tc.paste},
				key.SelectionEvent(key.Range{Start: rawCaret, End: rawCaret}),
			)
			if got := a.password.Text(); got != "existing" {
				t.Fatalf("rejected paste text = %q, want existing", got)
			}
			start, end := a.password.Selection()
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("rejected paste selection = %d/%d, want %d/%d", start, end, tc.wantStart, tc.wantEnd)
			}
			if !a.failed || !strings.Contains(a.message, "换行") {
				t.Fatalf("missing rejection feedback: failed=%t message=%q", a.failed, a.message)
			}
		})
	}
	if len(server.find("POST", "/oauth2/official")) != 0 {
		t.Fatal("rejected password paste submitted a login")
	}
}

func TestSearchPasteNormalizationRestoresCaretAndSelection(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"
	f := newSubmitFixture(t, a)
	f.focus(&a.searchEdit)

	for _, tc := range []struct {
		name       string
		before     string
		selStart   int
		selEnd     int
		paste      string
		wantText   string
		wantCaret  int
		continueAs string
		wantAfter  string
		wantCaret2 int
	}{
		{name: "middle LF", before: "prefixSuffix", selStart: 6, selEnd: 6, paste: "A\nB", wantText: "prefixA BSuffix", wantCaret: 9, continueAs: "X", wantAfter: "prefixA BXSuffix", wantCaret2: 10},
		{name: "middle CRLF", before: "prefixSuffix", selStart: 6, selEnd: 6, paste: "A\r\nB", wantText: "prefixA BSuffix", wantCaret: 9, continueAs: "X", wantAfter: "prefixA BXSuffix", wantCaret2: 10},
		{name: "middle CR", before: "prefixSuffix", selStart: 6, selEnd: 6, paste: "A\rB", wantText: "prefixA BSuffix", wantCaret: 9, continueAs: "X", wantAfter: "prefixA BXSuffix", wantCaret2: 10},
		{name: "end paste", before: "prefixSuffix", selStart: 12, selEnd: 12, paste: "A\nB", wantText: "prefixSuffixA B", wantCaret: 15, continueAs: "X", wantAfter: "prefixSuffixA BX", wantCaret2: 16},
		{name: "replace suffix", before: "prefixSuffix", selStart: 6, selEnd: 12, paste: "A\r\nB", wantText: "prefixA B", wantCaret: 9, continueAs: "X", wantAfter: "prefixA BX", wantCaret2: 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a.searchEdit.SetText(tc.before)
			a.searchEdit.SetCaret(tc.selStart, tc.selEnd)
			rawCaret := tc.selStart + len([]rune(tc.paste))
			if tc.selEnd > tc.selStart {
				rawCaret = tc.selStart + len([]rune(tc.paste))
			}
			f.queue(
				key.EditEvent{Range: key.Range{Start: tc.selStart, End: tc.selEnd}, Text: tc.paste},
				key.SelectionEvent(key.Range{Start: rawCaret, End: rawCaret}),
			)
			if got := a.searchEdit.Text(); got != tc.wantText {
				t.Fatalf("normalized text = %q, want %q", got, tc.wantText)
			}
			start, end := a.searchEdit.Selection()
			if start != tc.wantCaret || end != tc.wantCaret {
				t.Fatalf("caret after normalization = %d/%d, want %d", start, end, tc.wantCaret)
			}
			f.queue(
				key.EditEvent{Range: key.Range{Start: tc.wantCaret, End: tc.wantCaret}, Text: tc.continueAs},
				key.SelectionEvent(key.Range{Start: tc.wantCaret2, End: tc.wantCaret2}),
			)
			if got := a.searchEdit.Text(); got != tc.wantAfter {
				t.Fatalf("typing after paste = %q, want %q", got, tc.wantAfter)
			}
			start, end = a.searchEdit.Selection()
			if start != tc.wantCaret2 || end != tc.wantCaret2 {
				t.Fatalf("caret after follow-up typing = %d/%d, want %d", start, end, tc.wantCaret2)
			}
		})
	}
}

func TestSearchNormalizedPasteIsOneUndoableEdit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selStart int
		selEnd   int
		paste    string
		want     string
	}{
		{name: "middle LF", selStart: 6, selEnd: 6, paste: "A\nB", want: "prefixA BSuffix"},
		{name: "middle CRLF", selStart: 6, selEnd: 6, paste: "A\r\nB", want: "prefixA BSuffix"},
		{name: "middle CR", selStart: 6, selEnd: 6, paste: "A\rB", want: "prefixA BSuffix"},
		{name: "end LF", selStart: 12, selEnd: 12, paste: "A\nB", want: "prefixSuffixA B"},
		{name: "replace suffix", selStart: 6, selEnd: 12, paste: "A\r\nB", want: "prefixA B"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageSearch, Number: 1}}
			a.searchMode = "all"
			f := newSubmitFixture(t, a)
			f.focus(&a.searchEdit)
			a.searchEdit.SetText("prefixSuffix")
			a.searchEdit.SetCaret(tc.selStart, tc.selEnd)
			rawCaret := tc.selStart + len([]rune(tc.paste))
			f.queue(
				key.EditEvent{Range: key.Range{Start: tc.selStart, End: tc.selEnd}, Text: tc.paste},
				key.SelectionEvent(key.Range{Start: rawCaret, End: rawCaret}),
			)
			if got := a.searchEdit.Text(); got != tc.want {
				t.Fatalf("normalized text = %q, want %q", got, tc.want)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.searchEdit.Text(); got != "prefixSuffix" {
				t.Fatalf("one undo = %q, want original", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut | key.ModShift})
			if got := a.searchEdit.Text(); got != tc.want {
				t.Fatalf("one redo = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPasswordSameBatchKeepsLegalInputAndUndoesIt(t *testing.T) {
	for _, tc := range []struct {
		name       string
		paste      string
		pasteFirst bool
	}{
		{name: "LF legal first", paste: "LineA\nLineB"},
		{name: "CRLF legal first", paste: "LineA\r\nLineB"},
		{name: "CR legal first", paste: "LineA\rLineB"},
		{name: "CRLF paste first", paste: "LineA\r\nLineB", pasteFirst: true},
		{name: "LF paste first", paste: "LineA\nLineB", pasteFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageLogin}}
			f := newSubmitFixture(t, a)
			f.focus(&a.password)
			a.password.SetText("prior")
			a.password.SetCaret(5, 5)
			if tc.pasteFirst {
				rawEnd := 5 + len([]rune(tc.paste))
				f.queue(
					key.EditEvent{Range: key.Range{Start: 5, End: 5}, Text: tc.paste},
					key.SelectionEvent(key.Range{Start: rawEnd, End: rawEnd}),
					key.EditEvent{Range: key.Range{Start: rawEnd, End: rawEnd}, Text: "Z"},
					key.SelectionEvent(key.Range{Start: rawEnd + 1, End: rawEnd + 1}),
				)
			} else {
				f.queue(
					key.EditEvent{Range: key.Range{Start: 5, End: 5}, Text: "Z"},
					key.SelectionEvent(key.Range{Start: 6, End: 6}),
					key.EditEvent{Range: key.Range{Start: 6, End: 6}, Text: tc.paste},
					key.SelectionEvent(key.Range{Start: 6 + len([]rune(tc.paste)), End: 6 + len([]rune(tc.paste))}),
				)
			}
			if got := a.password.Text(); got != "priorZ" {
				t.Fatalf("after rejected paste = %q, want priorZ", got)
			}
			if !a.failed || !strings.Contains(a.message, "换行") {
				t.Fatalf("missing rejection feedback: failed=%t message=%q", a.failed, a.message)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.password.Text(); got != "prior" {
				t.Fatalf("undo legal input = %q, want prior", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut | key.ModShift})
			if got := a.password.Text(); got != "priorZ" {
				t.Fatalf("redo legal input = %q, want priorZ", got)
			}
		})
	}
}

func TestSearchPasteAtRepeatedCharacterBoundaryKeepsTrailingText(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"
	f := newSubmitFixture(t, a)
	f.focus(&a.searchEdit)
	a.searchEdit.SetText("prefixA")
	a.searchEdit.SetCaret(6, 6)
	f.queue(
		key.EditEvent{Range: key.Range{Start: 6, End: 6}, Text: "A\n"},
		key.SelectionEvent(key.Range{Start: 8, End: 8}),
	)
	if got := a.searchEdit.Text(); got != "prefixA A" {
		t.Fatalf("normalized text = %q, want prefixA A", got)
	}
	f.queue(key.EditEvent{Range: key.Range{Start: 8, End: 8}, Text: "X"})
	if got := a.searchEdit.Text(); got != "prefixA XA" {
		t.Fatalf("typing after paste = %q, want prefixA XA", got)
	}
	f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
	if got := a.searchEdit.Text(); got != "prefixA A" {
		t.Fatalf("undo typed char = %q, want prefixA A", got)
	}
	f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
	if got := a.searchEdit.Text(); got != "prefixA" {
		t.Fatalf("undo normalized paste = %q, want prefixA", got)
	}
	f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut | key.ModShift})
	f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut | key.ModShift})
	if got := a.searchEdit.Text(); got != "prefixA XA" {
		t.Fatalf("redo paste and typing = %q, want prefixA XA", got)
	}
}

func TestPasswordPasteThenEarlierCursorEditKeepsPosition(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageLogin}}
	f := newSubmitFixture(t, a)
	f.focus(&a.password)
	a.password.SetText("prefixSuffix")
	a.password.SetCaret(6, 6)
	f.queue(
		key.EditEvent{Range: key.Range{Start: 6, End: 6}, Text: "A\nB"},
		key.SelectionEvent(key.Range{Start: 9, End: 9}),
		key.SelectionEvent(key.Range{Start: 3, End: 3}),
		key.EditEvent{Range: key.Range{Start: 3, End: 3}, Text: "X"},
		key.SelectionEvent(key.Range{Start: 4, End: 4}),
	)
	if got := a.password.Text(); got != "preXfixSuffix" {
		t.Fatalf("after same-frame paste and earlier edit = %q, want preXfixSuffix", got)
	}
	start, end := a.password.Selection()
	if start != 4 || end != 4 {
		t.Fatalf("caret = %d/%d, want 4", start, end)
	}
	f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
	if got := a.password.Text(); got != "prefixSuffix" {
		t.Fatalf("undo legal edit = %q, want prefixSuffix", got)
	}
	f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut | key.ModShift})
	if got := a.password.Text(); got != "preXfixSuffix" {
		t.Fatalf("redo legal edit = %q, want preXfixSuffix", got)
	}
}

func TestSearchNormalizedPasteThenEarlierEditAndSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paste string
		want  string
	}{
		{name: "CRLF", paste: "A\r\nB", want: "preXfixA BSuffix"},
		{name: "LF", paste: "A\nB", want: "preXfixA BSuffix"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageSearch, Number: 1}}
			a.searchMode = "all"
			f := newSubmitFixture(t, a)
			f.focus(&a.searchEdit)
			a.searchEdit.SetText("prefixSuffix")
			a.searchEdit.SetCaret(6, 6)
			rawCaret := 6 + len([]rune(tc.paste))
			f.queue(
				key.EditEvent{Range: key.Range{Start: 6, End: 6}, Text: tc.paste},
				key.SelectionEvent(key.Range{Start: rawCaret, End: rawCaret}),
				key.SelectionEvent(key.Range{Start: 3, End: 3}),
				key.EditEvent{Range: key.Range{Start: 3, End: 3}, Text: "X"},
				key.SelectionEvent(key.Range{Start: 4, End: 4}),
			)
			if got := a.searchEdit.Text(); got != tc.want {
				t.Fatalf("after same-frame paste and earlier edit = %q, want %q", got, tc.want)
			}
			start, end := a.searchEdit.Selection()
			if start != 4 || end != 4 {
				t.Fatalf("caret = %d/%d, want 4", start, end)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.searchEdit.Text(); got != "prefixA BSuffix" {
				t.Fatalf("undo legal edit = %q, want prefixA BSuffix", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.searchEdit.Text(); got != "prefixSuffix" {
				t.Fatalf("undo normalized paste = %q, want prefixSuffix", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut | key.ModShift})
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut | key.ModShift})
			if got := a.searchEdit.Text(); got != tc.want {
				t.Fatalf("redo paste and edit = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSearchNormalizedPasteKeepsSameFrameSelectionBeforePaste(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"
	f := newSubmitFixture(t, a)
	f.focus(&a.searchEdit)
	a.searchEdit.SetText("prefixSuffix")
	a.searchEdit.SetCaret(6, 6)
	f.queue(
		key.EditEvent{Range: key.Range{Start: 6, End: 6}, Text: "A\r\nB"},
		key.SelectionEvent(key.Range{Start: 9, End: 9}),
		key.SelectionEvent(key.Range{Start: 3, End: 5}),
	)
	if got := a.searchEdit.Text(); got != "prefixA BSuffix" {
		t.Fatalf("text = %q, want prefixA BSuffix", got)
	}
	start, end := a.searchEdit.Selection()
	if start != 3 || end != 5 {
		t.Fatalf("same-frame selection = %d/%d, want 3/5", start, end)
	}
}

func TestPasswordRejectedPasteShorterThanSelectionKeepsLaterEditsAndSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paste string
	}{
		{name: "shorter", paste: "A\nB"},
		{name: "equal", paste: "A\nBCDE"},
		{name: "longer", paste: "A\nBCDEFGH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageLogin}}
			f := newSubmitFixture(t, a)
			f.focus(&a.password)
			a.password.SetText("prefixMIDDLESuffix")
			a.password.SetCaret(6, 12)
			rawAfterPaste := []rune("prefix" + tc.paste + "Suffix")
			sPos := len([]rune("prefix" + tc.paste))
			f.queue(
				key.EditEvent{Range: key.Range{Start: 6, End: 12}, Text: tc.paste},
				key.SelectionEvent(key.Range{Start: sPos + 1, End: sPos + 1}),
				key.EditEvent{Range: key.Range{Start: sPos, End: sPos + 1}, Text: "X"},
				key.SelectionEvent(key.Range{Start: 1, End: 4}),
			)
			_ = rawAfterPaste
			if got := a.password.Text(); got != "prefixMIDDLEXuffix" {
				t.Fatalf("text = %q, want prefixMIDDLEXuffix", got)
			}
			start, end := a.password.Selection()
			if start != 1 || end != 4 {
				t.Fatalf("selection = %d/%d, want 1/4", start, end)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("undo = %q, want original", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut | key.ModShift})
			if got := a.password.Text(); got != "prefixMIDDLEXuffix" {
				t.Fatalf("redo = %q, want replaced suffix", got)
			}
		})
	}
}

func TestSearchNormalizedPasteShorterThanSelectionKeepsLaterEditsAndSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paste string
	}{
		{name: "shorter", paste: "A\r\nB"},
		{name: "equal", paste: "A\r\nBCD"},
		{name: "longer", paste: "A\r\nBCDEF"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageSearch, Number: 1}}
			a.searchMode = "all"
			f := newSubmitFixture(t, a)
			f.focus(&a.searchEdit)
			a.searchEdit.SetText("prefixMIDDLESuffix")
			a.searchEdit.SetCaret(6, 12)
			rawS := 6 + len([]rune(tc.paste))
			f.queue(
				key.EditEvent{Range: key.Range{Start: 6, End: 12}, Text: tc.paste},
				key.SelectionEvent(key.Range{Start: rawS + 1, End: rawS + 1}),
				key.EditEvent{Range: key.Range{Start: rawS, End: rawS + 1}, Text: "X"},
				key.SelectionEvent(key.Range{Start: 1, End: 4}),
			)
			normalized := "prefix" + normalizeSingleLinePaste(tc.paste)
			if got, want := a.searchEdit.Text(), normalized+"Xuffix"; got != want {
				t.Fatalf("text = %q, want %q", got, want)
			}
			start, end := a.searchEdit.Selection()
			if start != 1 || end != 4 {
				t.Fatalf("selection = %d/%d, want 1/4", start, end)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got, want := a.searchEdit.Text(), normalized+"Suffix"; got != want {
				t.Fatalf("undo later edit = %q, want %q", got, want)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.searchEdit.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("undo paste = %q, want original", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut | key.ModShift})
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut | key.ModShift})
			if got, want := a.searchEdit.Text(), normalized+"Xuffix"; got != want {
				t.Fatalf("redo = %q, want %q", got, want)
			}
		})
	}
}

func TestPasswordRejectedPasteKeepsIndependentSameFrameSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paste string
	}{
		{name: "short paste", paste: "A\nB"},
		{name: "equal paste", paste: "A\nBCDE"},
		{name: "long paste", paste: "A\nBCDEFGH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageLogin}}
			f := newSubmitFixture(t, a)
			f.focus(&a.password)
			a.password.SetText("prefixMIDDLESuffix")
			a.password.SetCaret(6, 12)
			rawCaret := 6 + len([]rune(tc.paste))
			f.queue(
				key.EditEvent{Range: key.Range{Start: 6, End: 12}, Text: tc.paste},
				key.SelectionEvent(key.Range{Start: rawCaret, End: rawCaret}),
				key.SelectionEvent(key.Range{Start: 1, End: 4}),
			)
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("text = %q, want original", got)
			}
			start, end := a.password.Selection()
			if start != 1 || end != 4 {
				t.Fatalf("selection = %d/%d, want independent 1/4", start, end)
			}
			f.queue(key.EditEvent{Range: key.Range{Start: 1, End: 4}, Text: "X"})
			if got := a.password.Text(); got != "pXixMIDDLESuffix" {
				t.Fatalf("next-frame edit = %q, want pXixMIDDLESuffix", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("undo = %q, want original", got)
			}
		})
	}
}

func TestPasswordRejectedPasteRightArrowCollapsesAtRestoredEnd(t *testing.T) {
	for _, tc := range []struct {
		name        string
		paste       string
		withDefault bool
	}{
		{name: "short no default", paste: "A\nB"},
		{name: "short default", paste: "A\nB", withDefault: true},
		{name: "equal no default", paste: "A\nBCDE"},
		{name: "equal default", paste: "A\nBCDE", withDefault: true},
		{name: "long no default", paste: "A\nBCDEFGH"},
		{name: "long default", paste: "A\nBCDEFGH", withDefault: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageLogin}}
			f := newSubmitFixture(t, a)
			f.focus(&a.password)
			a.password.SetText("prefixMIDDLESuffix")
			a.password.SetCaret(6, 12)
			rawCaret := 6 + len([]rune(tc.paste))
			events := []event.Event{
				key.EditEvent{Range: key.Range{Start: 6, End: 12}, Text: tc.paste},
			}
			if tc.withDefault {
				events = append(events, key.SelectionEvent(key.Range{Start: rawCaret, End: rawCaret}))
			}
			events = append(events, key.Event{Name: key.NameRightArrow, State: key.Press})
			f.queue(events...)
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("text = %q, want original", got)
			}
			start, end := a.password.Selection()
			if start != 12 || end != 12 {
				t.Fatalf("selection = %d/%d, want collapsed at restored MIDDLE end 12", start, end)
			}
			f.queue(key.EditEvent{Range: key.Range{Start: 12, End: 12}, Text: "X"})
			if got := a.password.Text(); got != "prefixMIDDLEXSuffix" {
				t.Fatalf("next-frame edit = %q, want prefixMIDDLEXSuffix", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("undo = %q, want original", got)
			}
		})
	}
}

func TestPasswordLegalReplaceRightArrowMovesAfterReplacement(t *testing.T) {
	for _, withDefault := range []bool{false, true} {
		name := "no default selection"
		if withDefault {
			name = "default selection"
		}
		t.Run(name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageLogin}}
			f := newSubmitFixture(t, a)
			f.focus(&a.password)
			a.password.SetText("prefixMIDDLESuffix")
			a.password.SetCaret(6, 12)
			events := []event.Event{
				key.EditEvent{Range: key.Range{Start: 6, End: 12}, Text: "X"},
			}
			if withDefault {
				events = append(events, key.SelectionEvent(key.Range{Start: 7, End: 7}))
			}
			events = append(events, key.Event{Name: key.NameRightArrow, State: key.Press})
			f.queue(events...)
			if got := a.password.Text(); got != "prefixXSuffix" {
				t.Fatalf("text after replace = %q, want prefixXSuffix", got)
			}
			start, end := a.password.Selection()
			if start != 8 || end != 8 {
				t.Fatalf("caret after right = %d/%d, want 8", start, end)
			}
			f.queue(key.EditEvent{Range: key.Range{Start: 8, End: 8}, Text: "Y"})
			if got := a.password.Text(); got != "prefixXSYuffix" {
				t.Fatalf("next-frame edit = %q, want prefixXSYuffix", got)
			}
		})
	}
}

func TestPasswordRejectedPasteRightBeforeLaterSelectionKeepsSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paste string
	}{
		{name: "short", paste: "A\nB"},
		{name: "equal", paste: "A\nBCDE"},
		{name: "long", paste: "A\nBCDEFGH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageLogin}}
			f := newSubmitFixture(t, a)
			f.focus(&a.password)
			a.password.SetText("prefixMIDDLESuffix")
			a.password.SetCaret(6, 12)
			f.queue(
				key.EditEvent{Range: key.Range{Start: 6, End: 12}, Text: tc.paste},
				key.Event{Name: key.NameRightArrow, State: key.Press},
				key.SelectionEvent(key.Range{Start: 1, End: 4}),
			)
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("text = %q, want original", got)
			}
			start, end := a.password.Selection()
			if start != 1 || end != 4 {
				t.Fatalf("selection = %d/%d, want later user selection 1/4", start, end)
			}
			f.queue(key.EditEvent{Range: key.Range{Start: 1, End: 4}, Text: "X"})
			if got := a.password.Text(); got != "pXixMIDDLESuffix" {
				t.Fatalf("next-frame edit = %q, want pXixMIDDLESuffix", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("undo = %q, want original", got)
			}
		})
	}
}

func TestPasswordRejectedPasteTailSelectionThenRightCollapsesAtRestoredEnd(t *testing.T) {
	for _, tc := range []struct {
		name        string
		paste       string
		withDefault bool
	}{
		{name: "short", paste: "A\nB"},
		{name: "short default caret", paste: "A\nB", withDefault: true},
		{name: "equal", paste: "A\nBCDE"},
		{name: "equal default caret", paste: "A\nBCDE", withDefault: true},
		{name: "long", paste: "A\nBCDEFGH"},
		{name: "long default caret", paste: "A\nBCDEFGH", withDefault: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageLogin}}
			f := newSubmitFixture(t, a)
			f.focus(&a.password)
			a.password.SetText("prefixMIDDLESuffix")
			a.password.SetCaret(6, 12)
			pasteLen := len([]rune(tc.paste))
			events := []event.Event{
				key.EditEvent{Range: key.Range{Start: 6, End: 12}, Text: tc.paste},
			}
			if tc.withDefault {
				events = append(events, key.SelectionEvent(key.Range{Start: 6 + pasteLen, End: 6 + pasteLen}))
			}
			// The input method selects the last character it just pasted, then
			// the user presses Right in the same frame.
			events = append(events,
				key.SelectionEvent(key.Range{Start: 6 + pasteLen - 1, End: 6 + pasteLen}),
				key.Event{Name: key.NameRightArrow, State: key.Press},
			)
			f.queue(events...)
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("text = %q, want original", got)
			}
			start, end := a.password.Selection()
			if start != 12 || end != 12 {
				t.Fatalf("selection = %d/%d, want collapsed at restored MIDDLE end 12", start, end)
			}
			f.queue(key.EditEvent{Range: key.Range{Start: 12, End: 12}, Text: "X"})
			if got := a.password.Text(); got != "prefixMIDDLEXSuffix" {
				t.Fatalf("next-frame edit = %q, want prefixMIDDLEXSuffix", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("undo = %q, want original", got)
			}
		})
	}
}

func TestPasswordRejectedPasteSelectionThenRightCollapsesSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paste string
	}{
		{name: "short", paste: "A\nB"},
		{name: "equal", paste: "A\nBCDE"},
		{name: "long", paste: "A\nBCDEFGH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := interactionApp(t, "http://127.0.0.1:1")
			defer a.Close()
			a.stack = []Route{{Page: PageLogin}}
			f := newSubmitFixture(t, a)
			f.focus(&a.password)
			a.password.SetText("prefixMIDDLESuffix")
			a.password.SetCaret(6, 12)
			f.queue(
				key.EditEvent{Range: key.Range{Start: 6, End: 12}, Text: tc.paste},
				key.SelectionEvent(key.Range{Start: 1, End: 4}),
				key.Event{Name: key.NameRightArrow, State: key.Press},
			)
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("text = %q, want original", got)
			}
			start, end := a.password.Selection()
			if start != 4 || end != 4 {
				t.Fatalf("selection = %d/%d, want collapsed at the selected prefix end 4", start, end)
			}
			f.queue(key.EditEvent{Range: key.Range{Start: 4, End: 4}, Text: "X"})
			if got := a.password.Text(); got != "prefXixMIDDLESuffix" {
				t.Fatalf("next-frame edit = %q, want prefXixMIDDLESuffix", got)
			}
			f.queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModShortcut})
			if got := a.password.Text(); got != "prefixMIDDLESuffix" {
				t.Fatalf("undo = %q, want original", got)
			}
		})
	}
}
