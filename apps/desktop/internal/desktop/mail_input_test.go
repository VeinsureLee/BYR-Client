package desktop

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDesktopPrivateMessageInputRoundTripsEditedUnicodeThroughForm(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "mail-compose", Name: "bob"}
	h.app.mailEditor = mailEditor{To: "bob", Mode: "send"}
	h.view.Frame()

	if err := h.view.Click("信件标题"); err != nil {
		t.Fatal(err)
	}
	const initialTitle = "aBcD-URL_9中文😀"
	for _, r := range initialTitle {
		h.view.Type(string(r))
	}
	h.view.Type("-TailEN")
	h.idle()
	titleRunes := []rune(initialTitle + "-TailEN")
	for i := 0; i < len(titleRunes)-8; i++ {
		h.view.Key(0, ui.KeyLeft)
	}
	for i := 0; i < 5; i++ {
		h.view.Key(ui.Shift, ui.KeyLeft)
	}
	h.view.Type("d-URL")
	h.idle()
	expectedTitle := "aBcd-URL_9中文😀-TailEN"
	if got := h.app.mailEditor.Title; got != expectedTitle {
		t.Fatalf("title after selection replacement = %q, want %q", got, expectedTitle)
	}
	h.view.Key(0, ui.KeyEnd)
	h.view.SetClipboard("\nLineA\nLineB")
	h.view.Command("paste")
	h.idle()
	expectedTitle += " LineA LineB"
	if got := h.app.mailEditor.Title; got != expectedTitle {
		t.Fatalf("title after two-line paste = %q, want %q", got, expectedTitle)
	}

	if err := h.view.Click("信件正文"); err != nil {
		t.Fatal(err)
	}
	h.view.Type("开头 😀")
	h.view.Compose("候选", 2)
	h.view.Type("候选")
	h.idle()
	h.view.SetClipboard("LineA\nLineB")
	h.view.Command("paste")
	h.idle()
	expectedContent := "开头 😀候选LineA\nLineB"
	if got := h.app.mailEditor.Content; got != expectedContent {
		t.Fatalf("body after multiline paste = %q, want %q", got, expectedContent)
	}
	h.view.Command("undo")
	h.idle()
	if got := h.app.mailEditor.Content; got != "开头 😀候选" {
		t.Fatalf("body after undo = %q, want %q", got, "开头 😀候选")
	}
	h.view.Command("redo")
	h.idle()
	if got := h.app.mailEditor.Content; got != expectedContent {
		t.Fatalf("body after redo = %q, want %q", got, expectedContent)
	}

	if _, ok := h.view.Find("发送信件"); !ok {
		t.Fatal("send button is not visible")
	}
	h.click("发送信件")
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.forms) != 1 || h.forms[0].path != "/open/mail/send.json" {
		t.Fatalf("mail send forms = %+v, want exactly one send form", h.forms)
	}
	if got := h.forms[0].values.Get("title"); got != expectedTitle {
		t.Fatalf("decoded title = %q, want %q", got, expectedTitle)
	}
	if got := h.forms[0].values.Get("content"); got != expectedContent {
		t.Fatalf("decoded content = %q, want %q", got, expectedContent)
	}
}
