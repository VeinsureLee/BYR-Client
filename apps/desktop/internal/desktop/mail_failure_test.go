package desktop

import "testing"

func TestDesktopFailedPrivateMessageKeepsEditorContent(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "mail-compose", Name: "bob"}
	h.app.mailEditor = mailEditor{
		To: "bob", Title: "aBcD-URL_9中文😀TailEN", Content: "正文\nLineA\nLineB", Mode: "send",
	}
	h.mu.Lock()
	h.overrides = map[string]string{"/open/mail/send.json": "{invalid"}
	h.mu.Unlock()
	h.view.Frame()

	h.click("发送信件")
	if !h.app.failed {
		t.Fatalf("send failure not reported: message=%q", h.app.message)
	}
	if got := h.app.mailEditor.Title; got != "aBcD-URL_9中文😀TailEN" {
		t.Fatalf("failed send title = %q, want original text", got)
	}
	if got := h.app.mailEditor.Content; got != "正文\nLineA\nLineB" {
		t.Fatalf("failed send content = %q, want original text", got)
	}
}

func TestDesktopBlacklistAndForwardInputsHaveSeparateBuffers(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.forward = "forward-target"
	h.app.route = Route{Page: "blacklist"}
	h.view.Frame()
	h.typeIn("拉黑用户名", "blocked-user")
	if h.app.blacklistUser != "blocked-user" {
		t.Fatalf("blacklist buffer = %q, want blocked-user", h.app.blacklistUser)
	}
	if h.app.forward != "forward-target" {
		t.Fatalf("editing blacklist target changed forward target to %q", h.app.forward)
	}
}
