package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
)

type mailEditor struct {
	to, title, content, box, id, mode    string
	toEditor, titleEditor, contentEditor widget.Editor
}

func (a *App) openMessages(tab string) {
	if tab == "" {
		tab = "contacts"
	}
	route := Route{Page: PageMessages, Sub: tab, Number: 1}
	if tab == "refers" {
		route.Name = "reply"
	}
	if !a.push(route) {
		return
	}
	a.auxList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.loadMessages()
}

func (a *App) selectMessageTab(tab string) {
	if a.busy || a.current().Page != PageMessages {
		return
	}
	if a.current().Sub == tab && (tab != "refers" || a.current().Name == "reply") {
		return
	}
	a.stack[len(a.stack)-1] = Route{Page: PageMessages, Sub: tab, Name: "reply", Number: 1}
	a.auxList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.loadMessages()
}

func (a *App) selectReferKind(kind string) {
	if a.busy || a.current().Page != PageMessages || a.current().Sub != "refers" {
		return
	}
	a.stack[len(a.stack)-1].Name = kind
	a.stack[len(a.stack)-1].Number = 1
	a.loadMessages()
}

func (a *App) loadMessages() {
	route := a.current()
	page := max(1, route.Number)
	tab := route.Sub
	kind := route.Name
	if kind == "" {
		kind = "reply"
	}
	a.run("加载消息", func(ctx context.Context) (func(), error) {
		switch tab {
		case "inbox", "outbox":
			v, err := a.core.Forum.Mailbox(ctx, tab, forum.Page{Number: page, Size: 30})
			return func() { a.mailbox = v; a.pagination = v.Pagination }, err
		case "refers":
			v, err := a.core.Forum.Refers(ctx, kind, forum.Page{Number: page, Size: 30})
			return func() { a.refers = v; a.pagination = v.Pagination }, err
		default:
			v, err := a.core.Contacts(ctx)
			return func() { a.contacts = v; a.pagination = forum.Pagination{} }, err
		}
	})
}

func (a *App) openChat(name string) {
	name = strings.TrimSpace(name)
	if name == "" || !a.push(Route{Page: PageChat, Name: name}) {
		return
	}
	a.chat = nil
	a.auxList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.loadChat()
}

func (a *App) loadChat() {
	name := a.current().Name
	a.run("加载私信会话", func(ctx context.Context) (func(), error) {
		v, err := a.core.Conversation(ctx, name)
		return func() {
			if a.current().Page == PageChat && a.current().Name == name {
				a.chat = v
			}
		}, err
	})
}

func (a *App) openMailbox(box string) {
	if box == "" {
		box = "inbox"
	}
	a.openMessages(box)
}

func (a *App) openMail(box, id string) {
	if box == "" || id == "" || !a.push(Route{Page: PageMail, Name: box, ID: id}) {
		return
	}
	a.mail = forum.Mail{}
	a.auxList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.forwardIn.SetText("")
	a.loadMail()
}

func (a *App) loadMail() {
	route := a.current()
	a.run("加载站内信", func(ctx context.Context) (func(), error) {
		v, err := a.core.Forum.Mail(ctx, route.Name, route.ID)
		return func() {
			if a.current().Page == PageMail && a.current().ID == route.ID {
				a.mail = v
				a.mail.Box = route.Name
			}
		}, err
	})
}

func (a *App) openMailCompose(to, box, id, mode string) {
	if mode == "" {
		mode = "send"
	}
	route := Route{Page: PageMailCompose, Name: strings.TrimSpace(to), Sub: box, ID: id}
	if !a.push(route) {
		return
	}
	a.mailEditor = mailEditor{to: route.Name, box: box, id: id, mode: mode}
	a.mailEditor.toEditor.SingleLine = true
	a.mailEditor.titleEditor.SingleLine = true
	a.mailEditor.toEditor.SetText(route.Name)
	if mode == "reply" {
		title := a.mail.Title
		if !strings.HasPrefix(strings.ToLower(title), "re:") {
			title = "Re: " + title
		}
		a.mailEditor.titleEditor.SetText(title)
	}
	a.composeList.Position = layout.Position{}
}

func (a *App) messagesPage(gtx layout.Context) layout.Dimensions {
	tabs := []struct{ sub, label, icon string }{
		{"contacts", "私信会话", "mail"},
		{"inbox", "收件箱", "mail"},
		{"outbox", "发件箱", "send"},
		{"refers", "回复 / @", "clock"},
	}
	if a.current().Sub == "" {
		a.stack[len(a.stack)-1].Sub = "contacts"
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						children := make([]layout.FlexChild, 0, len(tabs)*2)
						for _, tab := range tabs {
							tab := tab
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.pill(gtx, "msg-tab:"+tab.sub, tab.label, tab.icon, a.current().Sub == tab.sub, false)
								if clicked {
									a.selectMessageTab(tab.sub)
								}
								return dims
							}))
							children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout))
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
					}),
				)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			switch a.current().Sub {
			case "inbox", "outbox":
				return a.mailboxBody(gtx)
			case "refers":
				return a.refersBody(gtx)
			default:
				return a.contactsBody(gtx)
			}
		}),
		layout.Rigid(a.pager),
	)
}

func (a *App) contactsBody(gtx layout.Context) layout.Dimensions {
	if len(a.contacts) == 0 {
		return a.empty(gtx, "还没有私信")
	}
	return material.List(a.theme, &a.auxList).Layout(gtx, len(a.contacts), func(gtx layout.Context, i int) layout.Dimensions {
		contact := a.contacts[i]
		btn := a.click("contact:" + string(contact.User.ID))
		clicked := btn.Clicked(gtx)
		dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.avatar(gtx, contact.User, 38)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(14), contact.User.DisplayName())
									l.Font.Weight = 600
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									preview := strings.TrimSpace(contact.Preview)
									if preview == "" {
										preview = "暂无消息预览"
									}
									l := material.Label(a.theme, unit.Sp(11), preview)
									l.Color = colorMuted
									l.MaxLines = 1
									l.Truncator = "…"
									return l.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.drawIcon(gtx, "next", 16, colorMuted)
						}),
					)
				})
			})
		})
		if clicked {
			a.openChat(string(contact.User.ID))
		}
		return dims
	})
}

func (a *App) mailboxBody(gtx layout.Context) layout.Dimensions {
	if len(a.mailbox.Mails) == 0 {
		return a.empty(gtx, "信箱是空的")
	}
	return material.List(a.theme, &a.auxList).Layout(gtx, len(a.mailbox.Mails), func(gtx layout.Context, i int) layout.Dimensions {
		m := a.mailbox.Mails[i]
		title := m.Title
		if !bool(m.IsRead) {
			title = "[未读] " + title
		}
		btn := a.click("mail:" + a.current().Sub + ":" + string(m.Index))
		clicked := btn.Clicked(gtx)
		dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(14), title)
							l.Font.Weight = 600
							l.MaxLines = 2
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(11),
								m.Author.DisplayName()+" · "+formatTime(m.PostTime))
							l.Color = colorMuted
							return l.Layout(gtx)
						}),
					)
				})
			})
		})
		if clicked {
			a.openMail(a.current().Sub, string(m.Index))
		}
		return dims
	})
}

func (a *App) refersBody(gtx layout.Context) layout.Dimensions {
	header := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.pill(gtx, "refer-kind:reply", "回复我的", "reply", a.current().Name != "at", false)
					if clicked {
						a.selectReferKind("reply")
					}
					return dims
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.pill(gtx, "refer-kind:at", "@ 我的", "send", a.current().Name == "at", false)
					if clicked {
						a.selectReferKind("at")
					}
					return dims
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.pill(gtx, "refer-read-all", "全部已读", "check", false, false)
					if clicked && !a.busy {
						kind := a.current().Name
						a.run("全部标为已读", func(ctx context.Context) (func(), error) {
							_, err := a.core.Forum.ReadAllRefers(ctx, kind)
							return func() { a.loadMessages() }, err
						})
					}
					return dims
				}),
			)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(header),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(a.refers.Articles) == 0 {
				return a.empty(gtx, "暂无提醒")
			}
			return material.List(a.theme, &a.auxList).Layout(gtx, len(a.refers.Articles), func(gtx layout.Context, i int) layout.Dimensions {
				refer := a.refers.Articles[i]
				title := refer.Title
				if !bool(refer.IsRead) {
					title = "[未读] " + title
				}
				btn := a.click("refer:" + string(refer.Index))
				clicked := btn.Clicked(gtx)
				dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(14), title)
									l.Font.Weight = 600
									l.MaxLines = 2
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(11),
										refer.Board+" · "+refer.Author.DisplayName())
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						})
					})
				})
				if clicked {
					a.openRefer(refer)
				}
				return dims
			})
		}),
	)
}

func (a *App) openRefer(refer forum.ReferEntry) {
	kind := a.current().Name
	id := string(refer.GroupID)
	if id == "" {
		id = string(refer.ID)
	}
	board := refer.Board
	if board == "" || id == "" {
		return
	}
	if a.busy {
		return
	}
	a.run("读取提醒", func(ctx context.Context) (func(), error) {
		_, err := a.core.Forum.ReadRefer(ctx, kind, string(refer.Index))
		return func() {
			a.push(Route{Page: PageThread, Name: board, ID: id, Number: max(1, (int(refer.Pos)-1)/50+1)})
			a.thread = forum.Thread{}
			a.loadThread()
		}, err
	})
}

func (a *App) chatPage(gtx layout.Context) layout.Dimensions {
	name := a.current().Name
	u := forum.User{ID: forum.Scalar(name), UserName: name}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.avatar(gtx, u, 40)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(15), name)
									l.Font.Weight = 700
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(11), fmt.Sprintf("%d 条消息", len(a.chat)))
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.pill(gtx, "chat-user", "资料", "self", false, false)
							if clicked {
								a.openUser(name)
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.filledPill(gtx, "chat-send", "发私信", "send")
							if clicked {
								a.openMailCompose(name, "", "", "send")
							}
							return dims
						}),
					)
				})
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(a.chat) == 0 {
				return a.empty(gtx, "还没有会话消息")
			}
			return material.List(a.theme, &a.auxList).Layout(gtx, len(a.chat), func(gtx layout.Context, i int) layout.Dimensions {
				return a.chatBubble(gtx, name, a.chat[i])
			})
		}),
	)
}

func (a *App) chatBubble(gtx layout.Context, name string, message application.ChatMessage) layout.Dimensions {
	who := name
	bg := colorSurface
	if message.Mine {
		who = "我"
		bg = colorAccentSoft
	}
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if message.Mine {
					return layout.Spacer{Width: unit.Dp(36)}.Layout(gtx)
				}
				return layout.Dimensions{}
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.cardTint(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(11), who+" · "+formatTime(message.Mail.PostTime))
							l.Color = colorAccent
							l.Font.Weight = 600
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if message.Mail.Title == "" {
								return layout.Dimensions{}
							}
							l := material.Label(a.theme, unit.Sp(13), message.Mail.Title)
							l.Font.Weight = 600
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.postBody(gtx, message.Mail.Content, message.Mail.Attachments)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !message.Mine {
					return layout.Spacer{Width: unit.Dp(36)}.Layout(gtx)
				}
				return layout.Dimensions{}
			}),
		)
	})
}

func (a *App) mailPage(gtx layout.Context) layout.Dimensions {
	m := a.mail
	box := a.current().Name
	return material.List(a.theme, &a.auxList).Layout(gtx, 2, func(gtx layout.Context, i int) layout.Dimensions {
		switch i {
		case 0:
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(18), m.Title)
						l.Font.Weight = 700
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(11),
							m.Author.DisplayName()+" · "+formatTime(m.PostTime))
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.filledPill(gtx, "mail-reply", "回复", "reply")
								if clicked {
									a.openMailCompose(string(m.Author.ID), box, a.current().ID, "reply")
								}
								return dims
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.pill(gtx, "mail-delete", "删除", "trash", true, true)
								if clicked && !a.busy {
									id := a.current().ID
									a.run("删除信件", func(ctx context.Context) (func(), error) {
										_, err := a.core.Forum.DeleteMail(ctx, box, id)
										return func() {
											a.stack = []Route{{Page: PageMessages, Sub: box, Number: 1}}
											a.loadMessages()
										}, err
									})
								}
								return dims
							}),
						)
					}),
				)
			})
		default:
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.postBody(gtx, m.Content, m.Attachments)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.inputBox(gtx, &a.forwardIn, "转发给用户名")
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									dims, clicked := a.pill(gtx, "mail-forward", "转发信件", "send", false, false)
									if clicked && !a.busy {
										target := strings.TrimSpace(a.forwardIn.Text())
										if target == "" {
											a.fail(errors.New("请输入转发目标"))
										} else {
											id := a.current().ID
											a.run("转发信件", func(ctx context.Context) (func(), error) {
												_, err := a.core.Forum.ForwardMail(ctx, box, id, target)
												return func() {
													a.forwardIn.SetText("")
													a.message = "信件已转发"
													a.failed = false
												}, err
											})
										}
									}
									return dims
								}),
							)
						})
					}),
				)
			})
		}
	})
}

func (a *App) mailComposePage(gtx layout.Context) layout.Dimensions {
	e := &a.mailEditor
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(18), "写站内信")
						l.Font.Weight = 700
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						hint := "收件人用户名"
						if e.mode == "reply" {
							hint = "回复信件"
						}
						l := material.Label(a.theme, unit.Sp(11), hint)
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			if e.mode == "reply" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.inputBox(gtx, &e.toEditor, "收件人用户名")
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.inputBox(gtx, &e.titleEditor, "信件标题")
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				height := gtx.Dp(unit.Dp(220))
				gtx.Constraints.Min.Y = height
				gtx.Constraints.Max.Y = height
				return a.inputBox(gtx, &e.contentEditor, "信件正文")
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				dims, clicked := a.primaryButton(gtx, "mail-send", "发送信件")
				if clicked {
					a.sendMail()
				}
				return dims
			})
		},
	}
	return material.List(a.theme, &a.composeList).Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		return rows[i](gtx)
	})
}

func (a *App) sendMail() {
	e := a.mailEditor
	to := strings.TrimSpace(e.toEditor.Text())
	title := strings.TrimSpace(e.titleEditor.Text())
	content := e.contentEditor.Text()
	if to == "" || strings.TrimSpace(content) == "" {
		a.fail(errors.New("收件人和正文不能为空"))
		return
	}
	if a.busy {
		return
	}
	mode, box, id := e.mode, e.box, e.id
	a.run("发送信件", func(ctx context.Context) (func(), error) {
		var err error
		if mode == "reply" {
			_, err = a.core.Forum.ReplyMail(ctx, box, id, title, content)
		} else {
			_, err = a.core.Forum.SendMail(ctx, to, title, content)
		}
		return func() {
			a.mailEditor = mailEditor{to: to, mode: "send"}
			a.mailEditor.toEditor.SingleLine = true
			a.mailEditor.titleEditor.SingleLine = true
			a.stack = []Route{{Page: PageChat, Name: to}}
			a.chat = nil
			a.loadChat()
		}, err
	})
}

func (a *App) cardTint(gtx layout.Context, bg color.NRGBA, content layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	inner := layout.UniformInset(unit.Dp(12)).Layout(gtx, content)
	call := m.Stop()
	paint.FillShape(gtx.Ops, bg,
		clip.RRect{Rect: image.Rectangle{Max: inner.Size}, NE: 12, NW: 12, SE: 12, SW: 12}.Op(gtx.Ops))
	call.Add(gtx.Ops)
	return inner
}
