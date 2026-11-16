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
	if board == "" {
		return
	}
	a.compose = composeState{board: board, ready: false}
	switch sub {
	case "reply":
		a.compose.replyID = id
		if a.current().Page == PageThread {
			a.compose.title = "Re: " + a.thread.Title
		}
	case "edit":
		a.compose.editID = id
	default:
		sub = "new"
	}
	a.compose.titleEditor.SingleLine = true
	a.push(Route{Page: PageCompose, Name: board, ID: id, Sub: sub})
	a.loadCompose()
}

func (a *App) loadCompose() {
	r := a.current()
	state := composeState{board: r.Name, replyID: "", editID: "", ready: false}
	if r.Sub == "reply" {
		state.replyID = r.ID
	}
	if r.Sub == "edit" {
		state.editID = r.ID
	}
	threadTitle := a.thread.Title
	a.run("准备编辑器", func(ctx context.Context) (func(), error) {
		if _, err := a.core.Forum.Board(ctx, r.Name, forum.Page{Size: 1}); err != nil {
			return nil, err
		}
		if state.editID != "" {
			v, err := a.core.Forum.Article(ctx, r.Name, state.editID)
			if err != nil {
				return nil, err
			}
			state.title, state.content = v.Title, v.Content
		}
		if state.replyID != "" {
			state.title = "Re: " + threadTitle
		}
		draft, exists := a.core.Local.Draft(a.core.Session().Username, local.DraftKey(r.Name, state.replyID, state.editID))
		if exists {
			state.title, state.content = draft.Title, draft.Content
		}
		return func() {
			a.compose = state
			a.compose.titleEditor.SingleLine = true
			a.compose.titleEditor.SetText(state.title)
			a.compose.contentEditor.SetText(state.content)
			a.compose.ready = true
			if exists {
				a.message = "已恢复本地草稿"
			}
		}, nil
	})
}

func (a *App) saveComposeDraft() {
	c := a.compose
	if !c.ready || c.submitted || c.publishing || c.board == "" {
		return
	}
	key := local.DraftKey(c.board, c.replyID, c.editID)
	err := a.core.Local.SaveDraft(context.Background(), a.core.Session().Username, key,
		local.Draft{Title: c.titleEditor.Text(), Content: c.contentEditor.Text()})
	if err != nil {
		a.fail(err)
	}
}

func (a *App) composePage(gtx layout.Context) layout.Dimensions {
	c := a.compose
	if !c.ready {
		return a.loadingPage(gtx, "正在准备编辑器…")
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(a.theme, "版面："+c.board)
			l.Color = a.theme.ContrastBg
			return l.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.inputBox(gtx, &a.compose.titleEditor, "帖子标题")
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(160))
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return a.inputBox(gtx, &a.compose.contentEditor, "正文（支持 UBB）")
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(a.theme, "Android 版暂不支持附件与图片上传。")
			l.Color = colorHint
			return l.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.primaryButton(gtx, "publish", "提交帖子")
					if clicked {
						a.publish()
					}
					return dims
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.primaryButton(gtx, "save-draft", "保存草稿")
					if clicked {
						a.saveComposeDraft()
						a.message = "草稿已保存"
						a.failed = false
						a.window.Invalidate()
					}
					return dims
				}),
			)
		}),
	)
}

func (a *App) publish() {
	c := a.compose
	if !c.ready || c.publishing || c.submitted || a.busy {
		return
	}
	title := strings.TrimSpace(c.titleEditor.Text())
	body := c.contentEditor.Text()
	if title == "" || strings.TrimSpace(body) == "" {
		a.fail(errors.New("标题和正文不能为空"))
		return
	}
	a.saveComposeDraft()
	threadRoute := ""
	if len(a.stack) > 1 {
		for _, r := range a.stack[:len(a.stack)-1] {
			if r.Page == PageThread {
				threadRoute = r.ID
			}
		}
	}
	a.compose.publishing = true
	a.run("提交帖子", func(ctx context.Context) (func(), error) {
		p := forum.Post{Board: c.board, Title: title, Content: body, ReplyID: c.replyID}
		result, err := a.core.Publish(ctx, p, c.editID)
		return func() {
			a.compose.submitted = true
			a.compose.publishing = false
			if result.LocalError != nil {
				a.fail(fmt.Errorf("帖子已发布，请勿重复提交；草稿清理失败：%w", result.LocalError))
			}
			id := threadRoute
			if c.replyID == "" && c.editID == "" {
				id = threadID(result.Article)
			}
			if id == "" {
				a.stack = []Route{{Page: PageBoard, Name: c.board, Number: 1}}
				a.loadBoard()
				return
			}
			a.stack = []Route{{Page: PageThread, Name: c.board, ID: id, Number: 1}}
			a.loadThread()
		}, err
	})
}
