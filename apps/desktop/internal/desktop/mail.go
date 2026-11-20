package desktop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/egoist/mygo/ui"
)

type mailEditor struct{ To, Title, Content, Mode, Box, ID string }

func (a *App) messageTabs(c *ui.Context) {
	selectedKey := "mail"
	if a.route.Page == "refers" {
		selectedKey = "reply"
		if a.route.Name == "at" {
			selectedKey = "at"
		}
	}
	ui.Row(c).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
		for _, tab := range []struct{ key, label, icon string }{
			{"reply", "回复提醒", "reply"},
			{"at", "@ 提醒", "at"},
			{"mail", "私信", "mail"},
		} {
			tab := tab
			tone := quiet
			if selectedKey == tab.key {
				tone = selected
			}
			a.action(c, tab.label, tab.icon, tone, func() {
				switch tab.key {
				case "reply", "at":
					a.navigate(Route{Page: "refers", Name: tab.key}, false)
				case "mail":
					a.navigate(Route{Page: "contacts"}, false)
				}
			})
		}
		if a.route.Page == "refers" {
			a.action(c, "一键已读", "check", quiet, func() {
				kind := "reply"
				if a.route.Name == "at" {
					kind = "at"
				}
				a.run("全部标为已读", func(ctx context.Context) (func(), error) {
					_, err := a.Core.Forum.ReadAllRefers(ctx, kind)
					return func() {
						a.load()
						a.notice("已全部读")
					}, err
				})
			})
		}
	})
}
func (a *App) contactsView(c *ui.Context) {
	a.messageTabs(c)
	if len(a.contacts) == 0 {
		emptyState(c, "message", "还没有私信", "在用户资料页可以给站内联系人发私信。")
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			for _, contact := range a.contacts {
				contact := contact
				ui.Column(c.Key("contact-"+string(contact.User.ID))).FillWidth().Padding(12, 14).Gap(6).
					Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).Children(func() {
					ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
						a.avatar(c, contact.User, 36)
						ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
							textLink(c, contact.User.DisplayName(), func() {
								a.navigate(Route{Page: "chat", Name: string(contact.User.ID)}, true)
							}, a.busy)
							preview := strings.TrimSpace(contact.Preview)
							if preview == "" {
								preview = "暂无消息预览"
							}
							ui.Text(c, preview).FontSize(12).TextColor(c.Theme().TextMuted).MaxLines(1).Ellipsis("…")
						})
						ui.Icon(c, icons["next"]).Size(15, 15).TextColor(c.Theme().TextMuted).Shrink(0)
					})
				})
			}
		})
	})
}
func (a *App) chatMessageBody(c *ui.Context, key string, message forum.Mail) {
	plain := strings.TrimSpace(content.Plain(message.Content))
	runes := []rune(plain)
	if utf8.RuneCountInString(plain) <= 420 {
		a.bodyView(c, message.Content, message.Attachments)
		return
	}
	if a.expandedMessages == nil {
		a.expandedMessages = make(map[string]bool)
	}
	if a.expandedMessages[key] {
		a.bodyView(c, message.Content, message.Attachments)
		a.action(c, "收起全文", "up", quiet, func() { a.expandedMessages[key] = false })
		return
	}
	preview := string(runes[:320]) + "…"
	ui.Text(c, preview).FontSize(13).MaxLines(6).Ellipsis("…")
	a.action(c, "展开全文", "next", quiet, func() { a.expandedMessages[key] = true })
}

func (a *App) messageContent(c *ui.Context, key, raw string, attachments []forum.Attachment) {
	plain := strings.TrimSpace(content.Plain(raw))
	runes := []rune(plain)
	if utf8.RuneCountInString(plain) <= 420 {
		a.bodyView(c, raw, attachments)
		return
	}
	if a.expandedMessages == nil {
		a.expandedMessages = make(map[string]bool)
	}
	if a.expandedMessages[key] {
		a.bodyView(c, raw, attachments)
		a.action(c, "收起全文", "up", quiet, func() { a.expandedMessages[key] = false })
		return
	}
	ui.Text(c, string(runes[:320])+"…").FontSize(13).MaxLines(6).Ellipsis("…")
	a.action(c, "展开全文", "next", quiet, func() { a.expandedMessages[key] = true })
}

func (a *App) chatView(c *ui.Context) {
	ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
		a.action(c, "查看用户", "user", quiet, func() { a.navigate(Route{Page: "user", Name: a.route.Name}, true) })
		a.action(c, "发送私信", "send", filled, func() { a.navigate(Route{Page: "mail-compose", Name: a.route.Name}, true) })
	})
	if len(a.chat) == 0 {
		emptyState(c, "message", "还没有会话消息", "发送第一封私信开始聊天。")
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			for _, message := range a.chat {
				message := message
				m := message.Mail
				who := a.route.Name
				background := c.Theme().Surface
				if message.Mine {
					who = "我"
					background = accentSurface(c)
				}
				ui.Row(c).FillWidth().Children(func() {
					if message.Mine {
						ui.Spacer(c)
					}
					ui.Column(c.Key(m.Box+string(m.Index))).MaxWidth(700).Padding(12, 14).Gap(6).
						Background(background).Border(1, c.Theme().Border).Radius(12).Children(func() {
						ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, who).FontSize(12).Bold().TextColor(c.Theme().Accent)
							if t := formatTime(m.PostTime); t != "" {
								ui.Text(c, t).FontSize(11).TextColor(c.Theme().TextMuted)
							}
						})
						if m.Title != "" {
							ui.Text(c, m.Title).FontSize(13).Bold()
						}
						a.chatMessageBody(c, m.Box+string(m.Index), m)
					})
					if !message.Mine {
						ui.Spacer(c)
					}
				})
			}
		})
	})
}
func (a *App) mailboxView(c *ui.Context) {
	a.messageTabs(c)
	ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		description := a.mailbox.Description
		if description == "" {
			description = "站内信"
		}
		ui.Text(c, description).FontSize(16).Bold()
		if a.mailbox.NewNum > 0 {
			badge(c, fmt.Sprintf("新信 %d", a.mailbox.NewNum), c.Theme().Accent)
		}
	})
	if len(a.mailbox.Mails) == 0 {
		emptyState(c, "mail", "信箱是空的", "这里会显示收到的站内信。")
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			for _, m := range a.mailbox.Mails {
				m := m
				title := m.Title
				if !bool(m.IsRead) {
					title = "[未读] " + title
				}
				ui.Column(c.Key("mail-"+string(m.Index))).FillWidth().Padding(12, 14).Gap(5).
					Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						textLink(c, title, func() {
							a.navigate(Route{Page: "mail", Name: a.route.Name, ID: string(m.Index)}, true)
						}, a.busy)
						ui.Icon(c, icons["next"]).Size(15, 15).TextColor(c.Theme().TextMuted).Shrink(0)
					})
					meta := []string{}
					if m.Author.DisplayName() != "" {
						meta = append(meta, m.Author.DisplayName())
					}
					if t := formatTime(m.PostTime); t != "" {
						meta = append(meta, t)
					}
					if len(meta) > 0 {
						ui.Text(c, strings.Join(meta, " · ")).FontSize(12).TextColor(c.Theme().TextMuted)
					}
				})
			}
		})
	})
	a.pager(c, a.mailbox.Pagination, a.pageNumber)
}
func (a *App) mailView(c *ui.Context) {
	m := a.mail
	r := a.route
	ui.Column(c).FillWidth().Padding(14).Gap(9).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, m.Title).FontSize(18).Bold().MaxLines(2)
				ui.Text(c, m.Author.DisplayName()+" · "+formatTime(m.PostTime)).
					FontSize(12).TextColor(c.Theme().TextMuted)
			})
			a.action(c, "回复站内信", "reply", filled, func() {
				a.navigate(Route{Page: "mail-compose", Name: string(m.Author.ID), Sub: r.Name, ID: r.ID}, true)
			})
			a.action(c, "删除信件", "trash", destructive, func() {
				a.confirm = &confirmation{text: "确定删除这封站内信？", run: func() {
					a.run("删除信件", func(ctx context.Context) (func(), error) {
						_, err := a.Core.Forum.DeleteMail(ctx, r.Name, r.ID)
						return func() { a.navigate(Route{Page: "contacts"}, false) }, err
					})
				}}
			})
		})
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.TextInput(c, &a.forward).Label("转发目标").Placeholder("转发给用户名").Grow(1)
			a.action(c, "转发信件", "send", quiet, func() {
				target := strings.TrimSpace(a.forward)
				if target == "" {
					a.fail(errors.New("请输入转发目标"))
					return
				}
				a.run("转发信件", func(ctx context.Context) (func(), error) {
					_, err := a.Core.Forum.ForwardMail(ctx, r.Name, r.ID, target)
					return func() { a.forward = ""; a.notice("信件已转发") }, err
				})
			})
		})
		ui.Divider(c)
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() { a.chatMessageBody(c, m.Box+string(m.Index), m) })
		})
	})
}
func (a *App) mailComposeView(c *ui.Context) {
	e := &a.mailEditor
	ui.Column(c).FillWidth().Grow(1).Padding(16).Gap(10).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
		ui.Text(c, "写站内信").FontSize(18).Bold()
		ui.TextInput(c, &e.To).Label("收件人").Placeholder("收件人用户名").ReadOnly(e.Mode == "reply").FillWidth().Disabled(a.busy)
		ui.TextInput(c, &e.Title).Label("信件标题").Placeholder("标题").FillWidth().Disabled(a.busy)
		ui.TextArea(c, &e.Content).Label("信件正文").Placeholder("站内信正文").FillWidth().Grow(1).MinHeight(180).Disabled(a.busy)
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "发送信件").Disabled(a.busy).Clicked() {
				v := *e
				if strings.TrimSpace(v.To) == "" || strings.TrimSpace(v.Content) == "" {
					a.fail(errors.New("收件人和正文不能为空"))
					return
				}
				a.run("发送信件", func(ctx context.Context) (func(), error) {
					var err error
					if v.Mode == "reply" {
						_, err = a.Core.Forum.ReplyMail(ctx, v.Box, v.ID, v.Title, v.Content)
					} else {
						_, err = a.Core.Forum.SendMail(ctx, v.To, v.Title, v.Content)
					}
					return func() { a.mailEditor = mailEditor{}; a.navigate(Route{Page: "chat", Name: v.To}, true) }, err
				})
			}
			a.action(c, "取消", "close", quiet, func() { a.goBack() })
		})
	})
}
func (a *App) refersView(c *ui.Context) {
	a.messageTabs(c)
	if len(a.refers.Articles) == 0 {
		emptyState(c, "clock", "没有新的提醒", "有人回复或 @ 你时，会显示在这里。")
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			for _, v := range a.refers.Articles {
				v := v
				title := v.Title
				if !bool(v.IsRead) {
					title = "[未读] " + title
				}
				ui.Column(c.Key("refer-"+string(v.Index))).FillWidth().Padding(12, 14).Gap(5).
					Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						textLink(c, title, func() {
							kind := a.route.Name
							a.run("读取提醒", func(ctx context.Context) (func(), error) {
								_, err := a.Core.Forum.ReadRefer(ctx, kind, string(v.Index))
								return func() {
									id := v.GroupID
									if id == "" {
										id = v.ID
									}
									a.navigate(Route{Page: "thread", Name: v.Board, ID: string(id), Sub: string(v.ID), Number: max(1, (int(v.Pos)-1)/50+1)}, true)
								}, err
							})
						}, a.busy)
						ui.Icon(c, icons["next"]).Size(15, 15).TextColor(c.Theme().TextMuted).Shrink(0)
					})
					meta := []string{}
					if v.Author.DisplayName() != "" {
						meta = append(meta, v.Author.DisplayName())
					}
					if v.Board != "" {
						meta = append(meta, v.Board)
					}
					if t := formatTime(v.Time); t != "" {
						meta = append(meta, t)
					}
					if len(meta) > 0 {
						ui.Text(c, strings.Join(meta, " · ")).FontSize(12).TextColor(c.Theme().TextMuted)
					}
				})
			}
		})
	})
	a.pager(c, a.refers.Pagination, a.pageNumber)
}
