package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

func TestAndroidAllInputEditorsPreserveMixedCaseAndLinePolicy(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openCompose("Go", "", "new")
	composeTitle, composeBody := &a.compose.titleEditor, &a.compose.contentEditor
	linkURL, linkText := &a.compose.linkURL, &a.compose.linkText
	a.setSearchMode("all")
	a.stack = []Route{{Page: PageMessages, Sub: "mail", Name: "contacts"}}
	a.openMailCompose("bob", "", "", "send")

	fields := []struct {
		name      string
		editor    *widget.Editor
		multiline bool
	}{
		{name: "用户名", editor: &a.username},
		{name: "密码", editor: &a.password},
		{name: "全站搜索", editor: &a.searchEdit},
		{name: "收件人", editor: &a.mailEditor.toEditor},
		{name: "信件标题", editor: &a.mailEditor.titleEditor},
		{name: "信件正文", editor: &a.mailEditor.contentEditor, multiline: true},
		{name: "转发目标", editor: &a.forwardIn},
		{name: "黑名单用户名", editor: &a.blacklistIn},
		{name: "竞猜积分", editor: &a.scoreIn},
		{name: "帖子标题", editor: composeTitle},
		{name: "帖子正文", editor: composeBody, multiline: true},
		{name: "链接地址", editor: linkURL},
		{name: "链接文字", editor: linkText},
	}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 820)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	text := "aBcD-URL_9中文😀\nTailEN"
	for _, field := range fields {
		field := field
		t.Run(field.name, func(t *testing.T) {
			field.editor.SetText("")
			ops.Reset()
			gtx.Ops = &ops
			a.inputBox(gtx, field.editor, field.name)
			router.Frame(&ops)
			gtx.Execute(key.FocusCmd{Tag: field.editor})
			router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: text})
			ops.Reset()
			gtx.Ops = &ops
			a.inputBox(gtx, field.editor, field.name)
			router.Frame(&ops)
			want := "aBcD-URL_9中文😀 TailEN"
			if field.multiline {
				want = "aBcD-URL_9中文😀\nTailEN"
			}
			if got := field.editor.Text(); got != want {
				t.Fatalf("editor text = %q, want %q", got, want)
			}
		})
	}
}
