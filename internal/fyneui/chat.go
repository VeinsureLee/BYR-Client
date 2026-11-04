package fyneui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (u *UI) buildChat() fyne.CanvasObject {
	u.chatMode = subtle("模拟流 · 无需 API Key")
	configure := widget.NewButtonWithIcon("模型", theme.SettingsIcon(), u.configureAI)
	demo := widget.NewButton("模拟", func() {
		u.controller.StopChat()
		u.chatConfig = nil
		u.chatMode.SetText("模拟流 · 无需 API Key")
	})
	header := container.NewBorder(nil, nil, nil, container.NewHBox(demo, configure),
		container.NewVBox(heading("AI 问答"), u.chatMode))
	u.answer = widget.NewRichTextFromMarkdown("### 用一段对话验证跨端体验\n\n输入一个问题，观察文本逐步出现。点击停止，已经收到的内容会保留。\n\n默认是**本地模拟流**，不会调用外部模型。")
	u.answer.Wrapping = fyne.TextWrapWord
	u.chatScroll = container.NewVScroll(padded(u.answer, 12, 12, 8, 8))
	u.question = widget.NewMultiLineEntry()
	u.question.PlaceHolder = "输入问题，检查中文输入与键盘适配…"
	u.question.Wrapping = fyne.TextWrapWord
	u.question.SetMinRowsVisible(2)
	u.chatSend = widget.NewButtonWithIcon("发送", theme.MailSendIcon(), func() {
		u.controller.StartChat(u.question.Text, u.chatConfig)
	})
	u.chatSend.Importance = widget.HighImportance
	u.question.OnSubmitted = func(string) { u.controller.StartChat(u.question.Text, u.chatConfig) }
	u.chatStop = widget.NewButton("停止", u.controller.StopChat)
	u.chatStop.Disable()
	u.chatStatus = subtle("等待提问")
	u.chatTrace = subtle("")
	u.autoFollow = widget.NewCheck("跟随回答", nil)
	u.autoFollow.SetChecked(true)
	buttons := container.NewHBox(u.chatSend, u.chatStop, layout.NewSpacer(), u.autoFollow)
	composer := container.NewVBox(widget.NewSeparator(), u.chatStatus, u.chatTrace, u.question, buttons)
	return panel(container.NewBorder(header, composer, nil, nil, u.chatScroll))
}

func (u *UI) syncChat() {
	chat := u.state.Chat
	status := chat.Status
	if chat.Error != "" {
		status = chat.Error
	}
	if status == "" {
		status = "等待提问"
	}
	u.chatStatus.SetText(status)
	setEnabled(u.chatSend, !chat.Running)
	setEnabled(u.chatStop, chat.Running)
	if len(chat.Trace) > 0 {
		u.chatTrace.SetText(chat.Trace[len(chat.Trace)-1])
	}
	if chat.Answer != u.answerText || (u.answerText == "" && chat.Running) {
		u.answerText = chat.Answer
		prefix := ""
		if chat.Question != "" {
			prefix = "**问题：** " + strings.ReplaceAll(chat.Question, "\n", " ") + "\n\n---\n\n"
		}
		u.answer.ParseMarkdown(prefix + chat.Answer)
		if u.autoFollow.Checked {
			u.chatScroll.ScrollToBottom()
		}
	}
}
