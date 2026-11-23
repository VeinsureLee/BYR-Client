package ui

import (
	"context"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/forum"
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
					a.threadMenuList = widget.List{List: layout.List{Axis: layout.Vertical}}
					a.ensureThreadExtras()
				}
				a.window.Invalidate()
			}
			return dims
		}),
	)
}

type threadMenuAction struct {
	key, label, icon string
	run              func(layout.Context)
}

func (a *App) threadMoreMenu(gtx layout.Context) layout.Dimensions {
	if !a.threadMoreOpen || !a.threadActionsAvailable() {
		return layout.Dimensions{}
	}
	actions := []threadMenuAction{
		{key: "thread-share", label: "复制链接", icon: "link", run: func(gtx layout.Context) {
			if a.core != nil {
				a.copyText(gtx, a.core.Forum.ThreadURL(a.threadBoard(), a.current().ID))
			}
		}},
		{key: "thread-refresh", label: "刷新主题", icon: "refresh", run: func(layout.Context) {
			a.refreshCurrent()
		}},
	}
	if a.threadVote != nil {
		actions = append(actions, threadMenuAction{key: "thread-vote", label: "投票", icon: "check", run: func(layout.Context) {
			a.openVote(a.current().ID)
		}})
	}
	if a.threadBet != nil {
		actions = append(actions, threadMenuAction{key: "thread-bet", label: "竞猜", icon: "hot", run: func(layout.Context) {
			a.openBet(a.current().ID)
		}})
	}
	if a.canManageThread() {
		actions = append(actions, threadMenuAction{key: "thread-manage", label: "管理", icon: "edit", run: func(layout.Context) {
			a.manageThread()
		}})
	}
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			// Reserve the pager and card padding, then let the menu scroll
			// inside the remaining viewport. A short window therefore keeps
			// its last action reachable instead of clipping it at height zero.
			available := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(52)) - gtx.Dp(unit.Dp(28))
			if available < gtx.Dp(unit.Dp(48)) {
				available = gtx.Dp(unit.Dp(48))
			}
			if max := gtx.Constraints.Max.Y; available > max {
				available = max
			}
			gtx.Constraints.Max.Y = available
			return material.List(a.theme, &a.threadMenuList).Layout(gtx, len(actions), func(gtx layout.Context, i int) layout.Dimensions {
				action := actions[i]
				dims, clicked := a.menuButton(gtx, action.key, action.label, action.icon)
				if clicked {
					a.threadMoreOpen = false
					action.run(gtx)
				}
				return dims
			})
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

type threadExtrasKey struct {
	Generation uint64
	Board      string
	ID         string
}

type threadExtrasState struct {
	Busy       bool
	Serial     uint64
	Cancel     context.CancelFunc
	Vote       *forum.Vote
	Bet        *forum.Bet
	VoteLoaded bool
	BetLoaded  bool
}

func (s *threadExtrasState) done() bool {
	return s != nil && s.VoteLoaded && s.BetLoaded
}

func (a *App) currentThreadExtrasKey() (threadExtrasKey, bool) {
	if a.core == nil || a.current().Page != PageThread || a.thread.ID == "" {
		return threadExtrasKey{}, false
	}
	board, id := a.threadBoard(), a.current().ID
	if strings.TrimSpace(board) == "" || strings.TrimSpace(id) == "" {
		return threadExtrasKey{}, false
	}
	return threadExtrasKey{Generation: a.core.SessionInfo().Generation, Board: board, ID: id}, true
}

func (a *App) syncThreadExtras() {
	key, ok := a.currentThreadExtrasKey()
	if !ok {
		a.threadExtrasBusy, a.threadExtrasDone = false, false
		a.threadVote, a.threadBet = nil, nil
		return
	}
	state := a.threadExtras[key]
	if state == nil {
		a.threadExtrasBusy, a.threadExtrasDone = false, false
		a.threadVote, a.threadBet = nil, nil
		return
	}
	a.threadExtrasBusy, a.threadExtrasDone = state.Busy, state.done()
	a.threadVote, a.threadBet = state.Vote, state.Bet
}

func (a *App) cancelThreadExtras() {
	for _, state := range a.threadExtras {
		if state.Cancel != nil {
			state.Cancel()
			state.Cancel = nil
		}
	}
}

// ensureThreadExtras keeps vote/bet state per account generation, canonical
// board and thread ID. Switching threads switches the state before any request,
// and a late response only updates its own entry. A transient failure leaves
// that item unloaded so a later refresh can retry it while a successful item
// stays cached.
func (a *App) ensureThreadExtras() {
	key, ok := a.currentThreadExtrasKey()
	if !ok {
		return
	}
	if a.threadExtras == nil {
		a.threadExtras = make(map[threadExtrasKey]*threadExtrasState)
	}
	state := a.threadExtras[key]
	if state == nil {
		state = &threadExtrasState{}
		a.threadExtras[key] = state
	}
	if state.Busy {
		a.syncThreadExtras()
		return
	}
	needVote := !state.VoteLoaded
	needBet := !state.BetLoaded
	if !needVote && !needBet {
		a.syncThreadExtras()
		return
	}
	state.Busy = true
	a.threadExtrasSerial++
	serial := a.threadExtrasSerial
	state.Serial = serial
	ctx, cancel := context.WithCancel(a.ctx)
	state.Cancel = cancel
	a.threadExtrasBusy, a.threadExtrasDone = true, false
	a.syncThreadExtras()
	go func() {
		var vote forum.Vote
		var voteErr error
		if needVote {
			vote, voteErr = a.core.Forum.Vote(ctx, key.ID)
		}
		var bet forum.Bet
		var betErr error
		if needBet {
			bet, betErr = a.core.Forum.Bet(ctx, key.ID)
		}
		a.post(func() {
			if a.closed.Load() || state.Serial != serial {
				return
			}
			cancel := state.Cancel
			state.Cancel = nil
			if cancel != nil {
				cancel()
			}
			state.Busy = false
			if needVote {
				if voteErr == nil {
					state.VoteLoaded = true
					state.Vote = nil
					if vote.ID != "" && (vote.ArticleID == "" || string(vote.ArticleID) == key.ID) {
						v := vote
						state.Vote = &v
					}
				}
			}
			if needBet {
				if betErr == nil {
					state.BetLoaded = true
					state.Bet = nil
					if bet.ID != "" && (bet.ArticleID == "" || string(bet.ArticleID) == key.ID) {
						b := bet
						state.Bet = &b
					}
				}
			}
			if current, ok := a.currentThreadExtrasKey(); ok && current == key {
				a.threadExtrasBusy = false
				a.threadExtrasDone = state.done()
				a.threadVote, a.threadBet = state.Vote, state.Bet
			}
			a.window.Invalidate()
		})
	}()
}
