package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/application"
)

type confirmationDialog struct {
	title, detail, confirmLabel string
	confirm                     func()
}

func (a *App) confirmClearHistory() {
	if a.core == nil {
		a.fail(application.ErrStaleSession)
		return
	}
	session := a.core.SessionInfo()
	if !session.Authenticated {
		a.fail(application.ErrStaleSession)
		return
	}
	var retry func()
	retry = func() {
		if a.core.SessionInfo().Generation != session.Generation {
			a.retryAction = nil
			a.fail(application.ErrStaleSession)
			return
		}
		a.run("清空最近阅读", func(ctx context.Context) (func(), error) {
			err := a.core.ClearHistory(ctx, session.Generation)
			return func() {
				a.retryAction = nil
				a.history = nil
				a.notice("最近阅读已清空")
			}, err
		})
		a.retryAction = retry
	}
	a.confirmation = &confirmationDialog{
		title:        "清空最近阅读",
		detail:       fmt.Sprintf("只清空当前账号 %s 的本机阅读记录。草稿、偏好和其他账号数据将保留。", session.Username),
		confirmLabel: "清空最近阅读",
		confirm:      retry,
	}
	a.window.Invalidate()
}

func (a *App) clearCache() {
	a.clearImageCache()
	a.notice("已清除缓存")
}

func (a *App) confirmationOverlay(gtx layout.Context) layout.Dimensions {
	dialog := a.confirmation
	if dialog == nil {
		return layout.Dimensions{}
	}
	maxSize := gtx.Constraints.Max
	// Take keyboard focus and a full-screen pointer surface before the dialog
	// buttons are added. A tap on the dimmed page is consumed here, so the
	// navigation, lists and buttons underneath cannot act.
	gtx.Execute(key.FocusCmd{Tag: &a.confirmationTag})
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: &a.confirmationTag}); !ok {
			break
		}
	}
	area := clip.Rect(image.Rectangle{Max: maxSize}).Push(gtx.Ops)
	event.Op(gtx.Ops, &a.confirmationTag)
	area.Pop()
	for {
		if _, ok := gtx.Event(pointer.Filter{
			Target: &a.confirmationTag,
			Kinds:  pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Scroll | pointer.Cancel,
		}); !ok {
			break
		}
	}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x70},
		clip.Rect(image.Rectangle{Max: maxSize}).Op())
	maxWidth := gtx.Dp(unit.Dp(480))
	if available := maxSize.X - gtx.Dp(unit.Dp(32)); available < maxWidth {
		maxWidth = available
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = 0
		gtx.Constraints.Max.X = maxWidth
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.H6(a.theme, dialog.title)
					l.Font.Weight = 700
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(a.theme, dialog.detail)
					l.Color = colorMuted
					l.MaxLines = 5
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.secondaryButton(gtx, "confirmation-cancel", "取消")
							if clicked {
								a.confirmation = nil
								a.window.Invalidate()
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.primaryButton(gtx, "confirmation-accept", dialog.confirmLabel)
							if clicked {
								a.confirmation = nil
								a.window.Invalidate()
								if dialog.confirm != nil {
									dialog.confirm()
								}
							}
							return dims
						}),
					)
				}),
			)
		})
	})
}
