package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
)

type mailEditor struct {
	to, title, content, box, id, mode    string
	toEditor, titleEditor, contentEditor widget.Editor
}

func (a *App) openMessages(tab string) {
	if tab == "at" || tab == "reply" || tab == "refers" {
		kind := tab
		if kind == "refers" {
			kind = "reply"
		}
		a.openMessageRoute(Route{Page: PageMessages, Sub: kind, Number: 1})
		return
	}
	// “站内信” is the combined private-message conversation list. Inbox and
	// outbox remain protocol details, not separate navigation destinations.
	a.openMessageRoute(Route{Page: PageMessages, Sub: "mail", Name: "contacts", Number: 1})
}

func (a *App) openMessageRoute(route Route) {
	if !a.push(route) {
		return
	}
	a.loadMessages()
}

func (a *App) selectMessageTab(tab string) {
	if a.busy || a.current().Page != PageMessages {
		return
	}
	var route Route
	switch tab {
	case "reply", "at", "refers":
		if tab == "refers" {
			tab = "reply"
		}
		route = Route{Page: PageMessages, Sub: tab, Number: 1}
	case "mail", "contacts", "inbox", "outbox":
		route = Route{Page: PageMessages, Sub: "mail", Name: "contacts", Number: 1}
	default:
		return
	}
	if a.current() == route {
		return
	}
	a.stack[len(a.stack)-1] = route
	a.loadMessages()
}

// referKind is the only route-to-endpoint mapping for reminder tabs.
func (a *App) referKind() string {
	route := a.current()
	if route.Sub == "at" || route.Sub == "refers" && route.Name == "at" {
		return "at"
	}
	return "reply"
}

func (a *App) selectReferKind(kind string) {
	if kind != "at" {
		kind = "reply"
	}
	a.selectMessageTab(kind)
}

func (a *App) loadMessages() {
	if a.current().Page == PageMessages {
		a.canonicalizeMessageRoute()
	}
	route := a.current()
	page := max(1, route.Number)
	tab := route.Sub
	if tab == "mail" {
		tab = route.Name
		if tab == "" {
			tab = "contacts"
		}
	} else if tab == "reply" || tab == "at" || tab == "refers" {
		tab = "refers"
	}
	kind := "reply"
	if tab == "refers" {
		kind = a.referKind()
	}
	a.run("加载消息", func(ctx context.Context) (func(), error) {
		switch tab {
		case "inbox", "outbox":
			v, err := a.core.LoadMailbox(ctx, tab, forum.Page{Number: page, Size: 30})
			return func() {
				if a.current() != route {
					return
				}
				a.mailbox = v
				a.pagination = v.Pagination
			}, err
		case "refers":
			v, err := a.core.LoadRefers(ctx, kind, forum.Page{Number: page, Size: 30})
			return func() {
				if a.current() != route {
					return
				}
				a.refers = v
				a.pagination = v.Pagination
				if a.messageAfterLoad != "" {
					message := a.messageAfterLoad
					a.messageAfterLoad = ""
					a.notice(message)
				}
			}, err
		default:
			v, err := a.core.Contacts(ctx)
			return func() {
				if a.current() != route {
					return
				}
				a.contacts = v
				a.pagination = forum.Pagination{}
			}, err
		}
	})
}

func (a *App) openChat(name string) {
	name = strings.TrimSpace(name)
	if name == "" || !a.push(Route{Page: PageChat, Name: name}) {
		return
	}
	a.chat = nil
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
	a.forwardIn.SetText("")
	a.loadMail()
}

func (a *App) loadMail() {
	route := a.current()
	a.run("加载站内信", func(ctx context.Context) (func(), error) {
		v, err := a.core.ReadMail(ctx, route.Name, route.ID)
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
	a.mailEditor.toEditor.InputHint = key.HintEmail
	a.mailEditor.titleEditor.InputHint = key.HintAny
	a.mailEditor.contentEditor.InputHint = key.HintAny
	a.mailEditor.toEditor.Submit = false
	a.mailEditor.titleEditor.Submit = false
	a.mailEditor.contentEditor.SingleLine = false
	a.mailEditor.contentEditor.Submit = false
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

func (a *App) canonicalizeMessageRoute() {
	if len(a.stack) == 0 || a.current().Page != PageMessages {
		return
	}
	route := a.current()
	switch route.Sub {
	case "", "contacts", "inbox", "outbox":
		route.Sub, route.Name = "mail", "contacts"
	case "refers":
		if route.Name == "at" {
			route.Sub = "at"
		} else {
			route.Sub = "reply"
		}
		route.Name = ""
	}
	a.stack[len(a.stack)-1] = route
}

func (a *App) currentMailBox() string {
	route := a.current()
	if route.Sub == "mail" {
		if route.Name == "inbox" || route.Name == "outbox" {
			return route.Name
		}
		return "inbox"
	}
	if route.Sub == "inbox" || route.Sub == "outbox" {
		return route.Sub
	}
	return "inbox"
}

func (a *App) messagesPage(gtx layout.Context) layout.Dimensions {
	a.canonicalizeMessageRoute()
	route := a.current()
	mainTabs := []moduleTab{
		{Key: "reply", Label: "回复提醒"},
		{Key: "at", Label: "@ 提醒"},
		{Key: "mail", Label: "私信"},
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.moduleTabs(gtx, mainTabs, route.Sub, a.selectMessageTab)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			switch route.Sub {
			case "reply", "at", "refers":
				return a.refersBody(gtx)
			case "mail":
				return a.contactsBody(gtx)
			default:
				return a.empty(gtx, "选择消息模块")
			}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if route.Sub == "reply" || route.Sub == "at" || route.Sub == "refers" {
				return a.pager(gtx)
			}
			return layout.Dimensions{}
		}),
	)
}

func (a *App) contactsBody(gtx layout.Context) layout.Dimensions {
	if len(a.contacts) == 0 {
		return a.emptyOrPending(gtx, "还没有私信")
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
		return a.emptyOrPending(gtx, "信箱是空的")
	}
	return material.List(a.theme, &a.auxList).Layout(gtx, len(a.mailbox.Mails), func(gtx layout.Context, i int) layout.Dimensions {
		m := a.mailbox.Mails[i]
		title := m.Title
		if !bool(m.IsRead) {
			title = "[未读] " + title
		}
		box := a.currentMailBox()
		btn := a.click("mail:" + box + ":" + string(m.Index))
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
			a.openMail(box, string(m.Index))
		}
		return dims
	})
}

func (a *App) readAllRefers() {
	sub := a.current().Sub
	if a.busy || a.current().Page != PageMessages || sub != "reply" && sub != "at" && sub != "refers" {
		return
	}
	kind := a.referKind()
	a.run("全部标为已读", func(ctx context.Context) (func(), error) {
		_, err := a.core.MarkAllRefersRead(ctx, kind)
		return func() {
			a.refers = forum.Refers{}
			a.pagination = forum.Pagination{}
			a.messageAfterLoad = "已全部读"
			a.loadMessages()
		}, err
	})
}

func (a *App) refersBody(gtx layout.Context) layout.Dimensions {
	if len(a.refers.Articles) == 0 {
		if a.busy || a.failed {
			return a.emptyOrPending(gtx, "暂未提醒")
		}
		gtx.Constraints.Min = gtx.Constraints.Max
		return a.empty(gtx, "暂未提醒")
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
							l := material.Label(a.theme, unit.Sp(11), refer.Board+" · "+refer.Author.DisplayName())
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
}

func (a *App) openRefer(refer forum.ReferEntry) {
	kind := a.referKind()
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
		_, err := a.core.MarkReferRead(ctx, kind, string(refer.Index))
		return func() {
			targetID := string(refer.ID)
			route := Route{Page: PageThread, Name: board, ID: id, Sub: targetID, Number: max(1, (int(refer.Pos)-1)/30+1)}
			if a.push(route) {
				a.thread = forum.Thread{}
				a.loadThread()
			}
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
							btn := a.click("chat-avatar")
							clicked := btn.Clicked(gtx)
							dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.avatar(gtx, u, 40)
							})
							if clicked {
								a.openUser(name)
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									btn := a.click("chat-name")
									clicked := btn.Clicked(gtx)
									dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										l := material.Label(a.theme, unit.Sp(15), name)
										l.Font.Weight = 700
										return l.Layout(gtx)
									})
									if clicked {
										a.openUser(name)
									}
									return dims
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(11), fmt.Sprintf("%d 条消息", len(a.chat)))
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						}),
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
				return a.emptyOrPending(gtx, "还没有会话消息")
			}
			return material.List(a.theme, &a.auxList).Layout(gtx, len(a.chat), func(gtx layout.Context, i int) layout.Dimensions {
				return a.chatBubble(gtx, name, a.chat[i])
			})
		}),
	)
}

func (a *App) messageBody(gtx layout.Context, key string, message forum.Mail) layout.Dimensions {
	plain := strings.TrimSpace(content.Plain(message.Content))
	runes := []rune(plain)
	foldable := len(runes) > 320
	expanded := a.expandedMail[key]
	if !foldable || expanded {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.postBody(gtx, message.Content, message.Attachments)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !foldable {
					return layout.Dimensions{}
				}
				dims, clicked := a.compactIconAction(gtx, "mail-fold:"+key, "收起长消息", "expand_less", "", 0, false, false)
				if clicked {
					a.expandedMail[key] = false
					a.window.Invalidate()
				}
				return dims
			}),
		)
	}
	preview := string(runes[:240]) + "…"
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.theme, preview)
			l.MaxLines = 5
			l.Truncator = "…"
			return l.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims, clicked := a.compactIconAction(gtx, "mail-fold:"+key, "展开长消息", "expand_more", "", 0, false, false)
			if clicked {
				a.expandedMail[key] = true
				a.window.Invalidate()
			}
			return dims
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
							key := message.Mail.Box + ":" + string(message.Mail.Index)
							return a.messageBody(gtx, key, message.Mail)
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
	if m.Index == "" && a.busy {
		return a.pendingPage(gtx, "正在加载站内信…")
	}
	if m.Index == "" && a.failed {
		return a.requestErrorPage(gtx)
	}
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
										_, err := a.core.DeleteMail(ctx, box, id)
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
							key := box + ":" + a.current().ID
							return a.messageBody(gtx, key, m)
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
												_, err := a.core.ForwardMail(ctx, box, id, target)
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
	title := e.titleEditor.Text()
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
			_, err = a.core.ReplyMail(ctx, box, id, title, content)
		} else {
			_, err = a.core.SendMail(ctx, to, title, content)
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
