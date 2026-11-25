package ui

import (
	"image"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestPrivateMessageInputRoundTripsThroughEditorAndForm(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageMessages, Sub: "mail", Name: "contacts"}}
	a.openMailCompose("bob", "", "", "send")

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	render := func() {
		ops.Reset()
		gtx.Ops = &ops
		a.mailComposePage(gtx)
		router.Frame(&ops)
	}
	render()
	gtx.Execute(key.FocusCmd{Tag: &a.mailEditor.titleEditor})

	const typed = "aBcD-URL_9中文😀"
	title := []rune(typed)
	for position, r := range title {
		router.Queue(key.EditEvent{Range: key.Range{Start: position, End: position}, Text: string(r)})
		render()
		if got, want := a.mailEditor.titleEditor.Text(), string(title[:position+1]); got != want {
			t.Fatalf("title editor after rune %d = %q, want %q", position, got, want)
		}
	}
	for _, r := range "-TailEN" {
		position := len([]rune(a.mailEditor.titleEditor.Text()))
		router.Queue(key.EditEvent{Range: key.Range{Start: position, End: position}, Text: string(r)})
		render()
	}
	// Replace the middle D-URL selection. The first eight runes are ASCII, so
	// the test range is identical in rune and byte offsets for this selection.
	router.Queue(key.EditEvent{Range: key.Range{Start: 3, End: 8}, Text: "d-URL"})
	render()
	// A line break pasted into this non-Submit single-line title is normalized
	// to a space; it must neither truncate the suffix nor send the form.
	position := len([]rune(a.mailEditor.titleEditor.Text()))
	router.Queue(key.EditEvent{Range: key.Range{Start: position, End: position}, Text: "\nLineA\nLineB"})
	render()
	expectedTitle := "aBcd-URL_9中文😀-TailEN LineA LineB"
	if got := a.mailEditor.titleEditor.Text(); got != expectedTitle {
		t.Fatalf("title editor = %q, want %q", got, expectedTitle)
	}
	if len(server.find("POST", "/open/mail/send.json")) != 0 {
		t.Fatal("editing or confirming the title submitted the message")
	}

	gtx.Execute(key.FocusCmd{Tag: &a.mailEditor.contentEditor})
	router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: "开头 😀\nLineA\nLineB"})
	render()
	expectedContent := "开头 😀\nLineA\nLineB"
	if got := a.mailEditor.contentEditor.Text(); got != expectedContent {
		t.Fatalf("content editor = %q, want %q", got, expectedContent)
	}
	// Simulate a committed IME candidate as an ordinary text edit, then prove
	// the explicit send action observes that latest text.
	position = len([]rune(a.mailEditor.contentEditor.Text()))
	router.Queue(key.EditEvent{Range: key.Range{Start: position, End: position}, Text: "候选"})
	render()
	expectedContent += "候选"
	if got := a.mailEditor.contentEditor.Text(); got != expectedContent {
		t.Fatalf("content after candidate commit = %q, want %q", got, expectedContent)
	}
	// Delete a selection and undo it through the editor's keyboard command.
	length := len([]rune(a.mailEditor.contentEditor.Text()))
	router.Queue(key.EditEvent{Range: key.Range{Start: length - 2, End: length}, Text: ""})
	render()
	router.Queue(key.Event{Name: "Z", State: key.Press, Modifiers: key.ModCtrl})
	render()
	if got := a.mailEditor.contentEditor.Text(); got != expectedContent {
		t.Fatalf("content after delete and undo = %q, want %q", got, expectedContent)
	}

	a.click("mail-send").Click()
	render()
	waitForIdle(t, a)
	requests := server.find("POST", "/open/mail/send.json")
	if len(requests) != 1 {
		t.Fatalf("mail send request count = %d, want exactly one", len(requests))
	}
	values, err := url.ParseQuery(requests[0].Form)
	if err != nil {
		t.Fatalf("decode mail send form %q: %v", requests[0].Form, err)
	}
	if got := values.Get("title"); got != expectedTitle {
		t.Fatalf("server-decoded title = %q, want %q", got, expectedTitle)
	}
	if got := values.Get("content"); got != expectedContent {
		t.Fatalf("server-decoded content = %q, want %q", got, expectedContent)
	}
}

func TestPasswordMultilinePasteIsRejectedWithoutImplicitLogin(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.password.SetText("keep")

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.loginPage(gtx)
	router.Frame(&ops)
	gtx.Execute(key.FocusCmd{Tag: &a.password})
	paste := "aBcD-URL_9中文😀\nTailEN"
	router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: paste})
	ops.Reset()
	gtx.Ops = &ops
	a.loginPage(gtx)
	router.Frame(&ops)

	if got := a.password.Text(); got != "keep" {
		t.Fatalf("multiline password paste changed the value to %q, want original", got)
	}
	if !a.failed || !strings.Contains(a.message, "换行") {
		t.Fatalf("multiline password paste feedback: failed=%t message=%q", a.failed, a.message)
	}
	if len(server.find("POST", "/oauth2/official")) != 0 {
		t.Fatal("editing the password submitted the login form")
	}
}

func TestSearchSingleLinePasteKeepsSuffixWithoutImplicitSubmit(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"
	a.searchHasRun = false

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.Layout(gtx)
	router.Frame(&ops)
	gtx.Execute(key.FocusCmd{Tag: &a.searchEdit})
	paste := "aBcD-URL_9中文😀\nTailEN"
	router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: paste})
	ops.Reset()
	gtx.Ops = &ops
	a.Layout(gtx)
	router.Frame(&ops)

	if got, want := a.searchEdit.Text(), "aBcD-URL_9中文😀 TailEN"; got != want {
		t.Fatalf("search text after multiline paste = %q, want %q", got, want)
	}
	if len(server.find("GET", "/open/search/article.json")) != 0 {
		t.Fatal("search editor submitted while applying a text edit")
	}
	a.click("search-submit").Click()
	ops.Reset()
	gtx.Ops = &ops
	a.Layout(gtx)
	router.Frame(&ops)
	waitForIdle(t, a)
	requests := server.find("GET", "/open/search/article.json")
	if len(requests) != 1 {
		t.Fatalf("one full-site submission made %d board requests, want exactly one", len(requests))
	}
	for _, request := range requests {
		values, err := url.ParseQuery(request.Query)
		if err != nil {
			t.Fatalf("decode search query %q: %v", request.Query, err)
		}
		if got := values.Get("title1"); got != "aBcD-URL_9中文😀 TailEN" {
			t.Fatalf("search request title1 = %q, want latest editor value", got)
		}
	}
	if !a.searchHasRun || a.searchListQuery != "aBcD-URL_9中文😀 TailEN" {
		t.Fatalf("search state = run:%t query:%q", a.searchHasRun, a.searchListQuery)
	}
}

func TestAndroidComposeLinkEditorsInsertAtUnicodeSelection(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openCompose("Go", "", "new")
	a.compose.contentEditor.SetText("前保留文本后")
	a.compose.contentEditor.SetCaret(2, 5)
	renderAppAt(t, a, 420, 700, 1)

	a.click("compose-link-open").Click()
	renderAppAt(t, a, 420, 700, 1)
	if !a.compose.linkOpen {
		t.Fatal("link inputs did not open")
	}
	if a.compose.linkURL.InputHint != key.HintURL || a.compose.linkText.InputHint != key.HintAny {
		t.Fatalf("link input hints = %v/%v, want URL/Any", a.compose.linkURL.InputHint, a.compose.linkText.InputHint)
	}
	a.compose.linkURL.SetText("https://Example.com/Path?Q=Case")
	a.compose.linkText.SetText("")
	a.click("compose-link-insert").Click()
	renderAppAt(t, a, 420, 700, 1)
	want := "前保[url=https://Example.com/Path?Q=Case]留文本[/url]后"
	if got := a.compose.contentEditor.Text(); got != want {
		t.Fatalf("body after link insertion = %q, want %q", got, want)
	}
	if a.compose.linkOpen || a.compose.linkURL.Text() != "" || a.compose.linkText.Text() != "" {
		t.Fatalf("link editor state after insert = open:%t url:%q text:%q", a.compose.linkOpen, a.compose.linkURL.Text(), a.compose.linkText.Text())
	}
}

func TestAndroidFailedPrivateMessageKeepsEditorContent(t *testing.T) {
	var mu sync.Mutex
	sends := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open/mail/send.json" {
			mu.Lock()
			sends++
			mu.Unlock()
			http.Error(w, "send unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0000"}`))
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageMessages, Sub: "mail", Name: "contacts"}}
	a.openMailCompose("bob", "", "", "send")
	a.mailEditor.titleEditor.SetText("aBcD-URL_9中文😀TailEN")
	a.mailEditor.contentEditor.SetText("正文\nLineA\nLineB")

	a.sendMail()
	waitForIdle(t, a)
	mu.Lock()
	gotSends := sends
	mu.Unlock()
	if gotSends != 1 || !a.failed {
		t.Fatalf("failed mail state: sends=%d failed=%t message=%q", gotSends, a.failed, a.message)
	}
	if got := a.mailEditor.titleEditor.Text(); got != "aBcD-URL_9中文😀TailEN" {
		t.Fatalf("failed send title = %q, want original text", got)
	}
	if got := a.mailEditor.contentEditor.Text(); got != "正文\nLineA\nLineB" {
		t.Fatalf("failed send content = %q, want original text", got)
	}
}

func TestAndroidMailSendPreservesTitleBoundarySpaces(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageMessages, Sub: "mail", Name: "contacts"}}
	a.openMailCompose("bob", "", "", "send")
	want := " aBcD-URL_9中文😀TailEN "
	a.mailEditor.titleEditor.SetText(want)
	a.mailEditor.contentEditor.SetText("正文")

	a.sendMail()
	waitForIdle(t, a)
	requests := server.find("POST", "/open/mail/send.json")
	if len(requests) != 1 {
		t.Fatalf("mail sends = %d, want one", len(requests))
	}
	values, err := url.ParseQuery(requests[0].Form)
	if err != nil {
		t.Fatalf("decode mail form: %v", err)
	}
	if got := values.Get("title"); got != want {
		t.Fatalf("server-decoded title = %q, want exact %q", got, want)
	}
}

func TestAndroidLoginRejectsUnsupportedUnicodeAtSubmit(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.username.SetText("用户😀")
	a.password.SetText("TestPassword")
	a.login()
	waitForIdle(t, a)
	if !a.failed || !strings.Contains(a.message, "单字节") {
		t.Fatalf("unsupported login characters did not produce visible validation: failed=%t message=%q", a.failed, a.message)
	}
	if got := len(server.find("POST", "/oauth2/official")); got != 0 {
		t.Fatalf("invalid Unicode login issued %d OAuth requests", got)
	}
}
