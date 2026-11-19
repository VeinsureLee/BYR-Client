package ui

import (
	"context"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
)

// threadActionsAvailable is deliberately stricter than merely having a thread
// route. A failed or still-loading route must not expose controls that would
// operate on stale or incomplete data.
func (a *App) threadActionsAvailable() bool {
	return a.current().Page == PageThread && a.thread.ID != "" && !a.failed
}

func (a *App) threadTopActions(gtx layout.Context) layout.Dimensions {
	if !a.threadActionsAvailable() {
		return layout.Dimensions{}
	}
	star := "star"
	if bool(a.thread.Collect) {
		star = "star_on"
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims, clicked := a.iconButton(gtx, "thread-collect-top", star)
			if clicked {
				a.threadMoreOpen = false
				a.collectThread()
			}
			return dims
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims, clicked := a.iconButton(gtx, "thread-more", "more")
			if clicked {
				a.threadMoreOpen = !a.threadMoreOpen
				if a.threadMoreOpen {
					a.ensureThreadExtras()
				}
				a.window.Invalidate()
			}
			return dims
		}),
	)
}

func (a *App) threadMoreMenu(gtx layout.Context) layout.Dimensions {
	if !a.threadMoreOpen || !a.threadActionsAvailable() {
		return layout.Dimensions{}
	}
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.pill(gtx, "thread-share", "复制链接", "link", false, false)
					if clicked {
						a.threadMoreOpen = false
						if a.core != nil {
							a.copyText(gtx, a.core.Forum.ThreadURL(a.threadBoard(), a.current().ID))
						}
					}
					return dims
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.pill(gtx, "thread-refresh", "刷新主题", "refresh", false, false)
					if clicked {
						a.threadMoreOpen = false
						a.refreshCurrent()
					}
					return dims
				}),
			}
			if a.threadVote != nil {
				children = append(children,
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dims, clicked := a.pill(gtx, "thread-vote", "投票", "check", false, false)
						if clicked {
							a.threadMoreOpen = false
							a.openVote(a.current().ID)
						}
						return dims
					}),
				)
			}
			if a.threadBet != nil {
				children = append(children,
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dims, clicked := a.pill(gtx, "thread-bet", "竞猜", "hot", false, false)
						if clicked {
							a.threadMoreOpen = false
							a.openBet(a.current().ID)
						}
						return dims
					}),
				)
			}
			if a.canManageThread() {
				children = append(children,
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dims, clicked := a.pill(gtx, "thread-manage", "管理", "edit", false, false)
						if clicked {
							a.manageThread()
						}
						return dims
					}),
				)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func (a *App) canManageThread() bool {
	if a.core == nil {
		return false
	}
	root, ok := a.threadRoot()
	if !ok || root.ID == "" {
		return false
	}
	username := strings.TrimSpace(a.core.Session().Username)
	if username == "" {
		return false
	}
	return string(root.Author.ID) == username || bool(root.IsAdmin) || bool(a.thread.IsAdmin)
}

func (a *App) manageThread() {
	if !a.canManageThread() {
		a.threadMoreOpen = false
		return
	}
	root, ok := a.threadRoot()
	if !ok || root.ID == "" {
		a.threadMoreOpen = false
		return
	}
	a.threadMoreOpen = false
	a.openCompose(a.threadBoard(), string(root.ID), "edit")
}

func (a *App) ensureThreadExtras() {
	if a.threadExtrasDone || a.threadExtrasBusy || a.core == nil || a.thread.ID == "" {
		return
	}
	route := a.current()
	a.threadExtrasBusy = true
	a.run("检查主题附加功能", func(ctx context.Context) (func(), error) {
		vote, voteErr := a.core.Forum.Vote(ctx, route.ID)
		bet, betErr := a.core.Forum.Bet(ctx, route.ID)
		return func() {
			a.threadExtrasBusy = false
			if a.current() != route {
				return
			}
			a.threadVote = nil
			a.threadBet = nil
			if voteErr == nil && vote.ID != "" && (vote.ArticleID == "" || string(vote.ArticleID) == route.ID) {
				v := vote
				a.threadVote = &v
			}
			if betErr == nil && bet.ID != "" && (bet.ArticleID == "" || string(bet.ArticleID) == route.ID) {
				b := bet
				a.threadBet = &b
			}
			a.threadExtrasDone = true
		}, nil
	})
}
