package ui

import (
	"fmt"

	"gioui.org/layout"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func threadExpansionKey(route Route, articleID string) string {
	return fmt.Sprintf("%d|%s|%s|%d|%s", PageThread, route.Name, route.ID, max(1, route.Number), articleID)
}

func boardPinnedRouteKey(route Route) string {
	return fmt.Sprintf("%d|%s|%d", PageBoard, route.Name, max(1, route.Number))
}

func (a *App) boardPinnedExpanded(route Route) bool {
	if a.pinnedByRoute == nil {
		a.pinnedByRoute = make(map[string]bool)
	}
	return a.pinnedByRoute[boardPinnedRouteKey(route)]
}

func (a *App) setBoardPinnedExpanded(route Route, expanded bool) {
	if a.pinnedByRoute == nil {
		a.pinnedByRoute = make(map[string]bool)
	}
	key := boardPinnedRouteKey(route)
	if expanded {
		a.pinnedByRoute[key] = true
	} else {
		delete(a.pinnedByRoute, key)
	}
	if a.current() == route {
		a.pinnedExpanded = expanded
	}
}

func (a *App) resetThreadExpanded(route Route) {
	if a.expandedPosts != nil {
		delete(a.expandedPosts, threadExpansionKey(route, ""))
	}
	// Article IDs are part of the key, so remove every article expansion for
	// this route without disturbing the saved state of another route.
	prefix := threadExpansionKey(route, "")
	for key := range a.expandedPosts {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(a.expandedPosts, key)
		}
	}
}

func (a *App) resetBoardPinnedExpanded(route Route) {
	if a.pinnedByRoute != nil {
		delete(a.pinnedByRoute, boardPinnedRouteKey(route))
	}
	if a.current() == route {
		a.pinnedExpanded = false
	}
}

func (a *App) resetRoutePosition(route Route) {
	position := layout.Position{}
	switch route.Page {
	case PageBoard:
		if a.listStates != nil {
			if state := a.listStates[listKey{Page: PageBoard, Name: route.Name, Sub: route.Sub, Number: max(1, route.Number)}]; state != nil {
				state.Position = position
			}
		}
	case PageThread:
		if a.listStates != nil {
			if state := a.listStates[listKey{Page: PageThread, Name: route.Name, ID: route.ID, Number: max(1, route.Number)}]; state != nil {
				state.Position = position
			}
		}
	case PageSearch:
		if a.listStates != nil {
			key := searchListKey(Route{Page: PageSearch, Number: max(1, route.Number)}, a.searchEdit.Text())
			if state := a.listStates[key]; state != nil {
				state.Position = position
			}
		}
	case PageHome:
		a.homeList.Position = position
	case PageSection:
		a.sectionList.Position = position
	case PageTop:
		if route.Name == "recommend" {
			a.recommendList.Position = position
		} else {
			a.topList.Position = position
		}
	case PageTimeline:
		a.timelineList.Position = position
	case PageFavorites:
		a.favoriteList.Position = position
	case PageSelf:
		a.selfList.Position = position
	case PageMessages, PageChat, PageMailbox, PageMail, PageMailCompose, PageRefers, PageUser, PageHistory, PageBlacklist, PageSettings, PageAccounts, PageVote, PageBet, PageCompose:
		a.auxList.Position = position
	}
}

// restoreRoute reloads the state owned by the route that became visible. The
// UI keeps one mutable page model for compact rendering, so popping a route
// must restore that model as well as the navigation stack.
func (a *App) restoreRoute(route Route) {
	if a.core == nil {
		return
	}
	a.clearRouteModel(route)
	switch route.Page {
	case PageHome:
		a.loadHome()
	case PageBoard:
		a.loadBoard()
	case PageThread:
		a.loadThread()
	case PageTop:
		a.loadTop()
	case PageTimeline:
		a.loadTimeline()
	case PageFavorites:
		a.loadFavorites()
	case PageMessages:
		a.loadMessages()
	case PageChat:
		a.loadChat()
	case PageMail:
		a.loadMail()
	case PageUser:
		a.loadUser(route.Name)
	case PageBlacklist:
		a.loadBlacklist()
	case PageSelf:
		a.loadSelf()
	case PageAccounts:
		a.loadAccounts()
	case PageVote:
		a.loadVote()
	case PageBet:
		a.loadBet()
	}
}

// clearRouteModel removes data from the page being revealed before its
// replacement request starts, so a slow response can never display another
// route's content under the new title.
func (a *App) clearRouteModel(route Route) {
	switch route.Page {
	case PageHome:
		a.sections = nil
		a.section = forum.Section{}
		a.boards = nil
		a.pagination = forum.Pagination{}
	case PageBoard:
		a.board = forum.Board{Name: route.Name}
		a.resetBoardPinnedExpanded(route)
	case PageThread:
		a.thread = forum.Thread{}
		a.resetThreadExpanded(route)
		a.threadExtrasBusy = false
		a.threadExtrasDone = false
		a.threadVote = nil
		a.threadBet = nil
	case PageTop, PageTimeline:
		a.articles = nil
		a.pagination = forum.Pagination{}
	case PageFavorites:
		a.favorite = forum.Favorite{}
		a.favoriteBoards = nil
		a.collect = forum.Collection{}
		a.pagination = forum.Pagination{}
	case PageMessages:
		a.contacts = nil
		a.mailbox = forum.Mailbox{}
		a.refers = forum.Refers{}
		a.pagination = forum.Pagination{}
	case PageChat:
		a.chat = nil
	case PageMail:
		a.mail = forum.Mail{}
	case PageUser, PageSelf:
		a.user = forum.User{}
	case PageAccounts:
		a.savedAccounts = nil
	case PageBlacklist:
		a.blacklist = forum.Blacklist{}
		a.pagination = forum.Pagination{}
	case PageVote:
		a.vote = forum.Vote{}
		a.choice = ""
	case PageBet:
		a.bet = forum.Bet{}
		a.choice = ""
	}
}
