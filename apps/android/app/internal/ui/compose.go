package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func (a *App) openCompose(board, id, sub string) {
	if a.busy {
		return
	}
	switch sub {
	case "reply", "edit":
		if strings.TrimSpace(board) == "" {
			return
		}
	default:
		sub = "new"
	}
	route := Route{Page: PageCompose, Name: strings.TrimSpace(board), ID: id, Sub: sub}
	if !a.push(route) {
		return
	}
	a.prepareCompose(route)
}

func (a *App) prepareCompose(r Route) {
	a.compose = composeState{
		board:       strings.TrimSpace(r.Name),
		boardLocked: strings.TrimSpace(r.Name) != "",
		ready:       true,
	}
	a.compose.titleEditor.SingleLine = true
	switch r.Sub {
	case "reply":
		a.compose.replyID = r.ID
		a.compose.title = replyTitle(a.thread.Title)
	case "edit":
		a.compose.editID = r.ID
	}

	board := a.compose.board
	key := local.DraftKey(board, a.compose.replyID, a.compose.editID)
	if draft, exists := a.core.Local.Draft(a.core.Session().Username, key); exists {
		a.compose.title = draft.Title
		a.compose.content = draft.Content
		a.message = "已恢复本地草稿"
		a.failed = false
	}
	a.compose.titleEditor.SetText(a.compose.title)
	a.compose.contentEditor.SetText(a.compose.content)
	a.composeList.Position = layout.Position{}

	if a.compose.editID != "" && a.compose.title == "" && a.compose.content == "" {
		a.loadCompose()
	}
}

func (a *App) loadCompose() {
	if a.current().Page != PageCompose || a.compose.editID == "" || a.busy {
		return
	}
	route := a.current()
	a.compose.loading = true
	a.run("加载帖子原文", func(ctx context.Context) (func(), error) {
		article, err := a.core.Forum.Article(ctx, route.Name, route.ID)
		return func() {
			a.compose.loading = false
			if a.current() != route {
				return
			}
			// Do not overwrite text entered while a slow edit request was running.
			if a.compose.titleEditor.Text() == "" && a.compose.contentEditor.Text() == "" {
				a.compose.titleEditor.SetText(article.Title)
				a.compose.contentEditor.SetText(article.Content)
			}
		}, err
	})
}

func replyTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Re:"
	}
	if strings.HasPrefix(strings.ToLower(title), "re:") {
		return title
	}
	return "Re: " + title
}

func (a *App) composeBoard() string {
	return strings.TrimSpace(a.compose.board)
}

func (a *App) saveComposeDraft() error {
	c := &a.compose
	if !c.ready || c.submitted || c.publishing || a.core == nil {
		return nil
	}
	title := c.titleEditor.Text()
	content := c.contentEditor.Text()
	if strings.TrimSpace(title) == "" && strings.TrimSpace(content) == "" {
		return nil
	}
	board := a.composeBoard()
	key := local.DraftKey(board, c.replyID, c.editID)
	if err := a.core.Local.SaveDraft(context.Background(), a.core.Session().Username, key,
		local.Draft{Title: title, Content: content}); err != nil {
		return err
	}
	c.board = board
	return nil
}

func (a *App) composePage(gtx layout.Context) layout.Dimensions {
	if !a.compose.ready {
		a.prepareCompose(a.current())
	}
	c := &a.compose
	rows := make([]layout.Widget, 0, 7)
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.iconTile(gtx, "edit", 40, colorAccentSoft, colorAccent)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						board := a.composeBoard()
						title := "新建帖子"
						if c.replyID != "" {
							title = "回复帖子"
						} else if c.editID != "" {
							title = "编辑帖子"
						}
						if board != "" {
							title += " · " + board
						}
						detail := "标题与正文，支持 UBB 标记"
						if c.loading {
							detail = "正在加载原文，编辑器仍可使用"
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(16), title)
								l.Font.Weight = 700
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(11), detail)
								l.Color = colorMuted
								return l.Layout(gtx)
							}),
						)
					}),
				)
			})
		})
	})
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						board := a.composeBoard()
						title := board
						if title == "" {
							title = "未选择版面"
						}
						detail := "发帖版面"
						if c.boardLocked {
							detail = "本主题所在版面"
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(11), detail)
								l.Color = colorMuted
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(15), title)
								l.Font.Weight = 700
								if board == "" {
									l.Color = colorDanger
								}
								return l.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if c.boardLocked {
							return layout.Dimensions{}
						}
						dims, clicked := a.pill(gtx, "compose-board", "选择版面", "board", false, false)
						if clicked {
							a.openSection("", true)
						}
						return dims
					}),
				)
			})
		})
	})
	rows = append(rows,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.inputBox(gtx, &c.titleEditor, "帖子标题")
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				height := gtx.Dp(unit.Dp(240))
				gtx.Constraints.Min.Y = height
				gtx.Constraints.Max.Y = height
				return a.inputBox(gtx, &c.contentEditor, "正文（支持 UBB）")
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, material.CheckBox(a.theme, &c.anonymous, "匿名").Layout),
								layout.Flexed(1, material.CheckBox(a.theme, &c.signature, "使用签名").Layout),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, material.CheckBox(a.theme, &c.email, "回复邮件提醒").Layout),
								layout.Flexed(1, material.CheckBox(a.theme, &c.outgo, "允许转出").Layout),
							)
						}),
					)
				})
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, unit.Sp(11), "附件与图片上传需要 Android 文件选择适配；当前正文中的网络图片仍可正常显示。")
					l.Color = colorMuted
					return l.Layout(gtx)
				})
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := "提交帖子"
					if c.publishing {
						label = "正在提交…"
					}
					dims, clicked := a.primaryButton(gtx, "publish", label)
					if clicked {
						a.publish()
					}
					return dims
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.secondaryButton(gtx, "save-draft", "保存草稿")
					if clicked {
						if err := a.saveComposeDraft(); err != nil {
							a.fail(err)
						} else {
							a.message = "草稿已保存"
							a.failed = false
							a.window.Invalidate()
						}
					}
					return dims
				}),
			)
		},
	)
	return material.List(a.theme, &a.composeList).Layout(gtx, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
		return rows[index](gtx)
	})
}

func (a *App) publish() {
	c := &a.compose
	if !c.ready || c.publishing || c.submitted || a.busy {
		return
	}
	board := a.composeBoard()
	title := strings.TrimSpace(c.titleEditor.Text())
	body := c.contentEditor.Text()
	if board == "" || title == "" || strings.TrimSpace(body) == "" {
		a.fail(errors.New("版面、标题和正文不能为空"))
		return
	}
	if err := a.saveComposeDraft(); err != nil {
		a.fail(err)
		return
	}
	threadRoute := ""
	if len(a.stack) > 1 {
		for _, r := range a.stack[:len(a.stack)-1] {
			if r.Page == PageThread {
				threadRoute = r.ID
			}
		}
	}
	post := forum.Post{
		Board: board, Title: title, Content: body, ReplyID: c.replyID,
		Anonymous: c.anonymous.Value, Signature: c.signature.Value,
		Email: c.email.Value, Outgo: c.outgo.Value,
	}
	editID := c.editID
	c.publishing = true
	a.run("提交帖子", func(ctx context.Context) (func(), error) {
		result, err := a.core.Publish(ctx, post, editID)
		return func() {
			a.compose.submitted = true
			a.compose.publishing = false
			a.compose.publishedID = string(result.Article.ID)
			id := threadRoute
			if post.ReplyID == "" && editID == "" {
				id = threadID(result.Article)
			}
			warning := ""
			if result.LocalError != nil {
				warning = fmt.Sprintf("帖子已发布，请勿重复提交；草稿清理失败：%v", result.LocalError)
			}
			if id == "" {
				a.stack = []Route{{Page: PageBoard, Name: board, Number: 1}}
				a.board = forum.Board{Name: board}
				a.loadBoard()
			} else {
				a.stack = []Route{{Page: PageThread, Name: board, ID: id, Number: 1}}
				a.thread = forum.Thread{}
				a.loadThread()
			}
			if warning != "" {
				a.message = warning
				a.failed = true
			}
		}, err
	})
}
