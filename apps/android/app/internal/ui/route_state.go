package ui

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

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
		a.boardList = widget.List{List: layout.List{Axis: layout.Vertical}}
	case PageThread:
		a.thread = forum.Thread{}
		a.threadList = widget.List{List: layout.List{Axis: layout.Vertical}}
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
	case PageBlacklist:
		a.blacklist = forum.Blacklist{}
		a.pagination = forum.Pagination{}
	case PageVote:
		a.vote = forum.Vote{}
	case PageBet:
		a.bet = forum.Bet{}
	}
}
