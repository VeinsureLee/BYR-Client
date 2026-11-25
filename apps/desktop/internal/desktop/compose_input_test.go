package desktop

import "testing"

func TestDesktopComposerFieldsPreserveMixedCaseUnicodeAndLinkText(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "compose"}
	h.view.Frame()

	h.typeIn("发帖版面", "Go")
	h.click("载入版面")
	if h.app.route.Page != "compose" || h.app.route.Name != "Go" {
		t.Fatalf("composer board route = %+v", h.app.route)
	}

	title := "aBcD-URL_9中文😀-TailEN"
	body := "正文 aBcD-URL_9中文😀\nLineA\nLineB 尾缀"
	h.typeIn("帖子标题", title)
	h.typeIn("帖子正文", body)
	if got := h.app.editor.Title; got != title {
		t.Fatalf("composer title = %q, want %q", got, title)
	}
	if got := h.app.editor.Content; got != body {
		t.Fatalf("composer body = %q, want %q", got, body)
	}

	h.click("插入链接")
	url := "https://Example.test/Path?Q=Case"
	label := "MiXeD显示文字😀"
	h.typeIn("链接地址", url)
	h.typeIn("链接文字", label)
	h.click("插入")
	wantBody := body + "[url=" + url + "]" + label + "[/url]"
	if got := h.app.editor.Content; got != wantBody {
		t.Fatalf("composer body after link insertion = %q, want %q", got, wantBody)
	}
}
