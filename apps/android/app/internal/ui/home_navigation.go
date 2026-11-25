package ui

import (
	"strings"

	"gioui.org/layout"
	"github.com/VeinsureLee/BYR-Client/core/forum"
)

type homeListPositions struct {
	home, section, timeline, favorite, self, compose, top, recommend, auxiliary, module layout.Position
	moduleKey, moduleSelected                                                           string
}

type homeListAnchor struct {
	route  Route
	itemID string
	row    int
	offset int
}

type homeNavigationSnapshot struct {
	username   string
	generation uint64
	module     string
	stack      []Route
	ready      bool

	sectionTab, favoriteTab, topTab string
	positions                       homeListPositions
	listPositions                   map[listKey]layout.Position
	anchor                          homeListAnchor

	sections       []forum.Section
	section        forum.Section
	boards         []forum.Board
	board          forum.Board
	thread         forum.Thread
	articles       []forum.Article
	favorite       forum.Favorite
	favoriteBoards []forum.Board
	collection     forum.Collection
	pagination     forum.Pagination
	user           forum.User

	threadModels         map[string]*postThreadContent
	threadLoadedRoute    Route
	threadLoadedRequest  Route
	threadAnchors        map[listKey]threadAnchor
	threadViewports      map[listKey]threadViewportKey
	threadRequestPage    int
	threadRetryPage      int
	threadRetryRefresh   bool
	threadRefreshPending bool

	pinnedByRoute  map[string]bool
	expandedPosts  map[string]bool
	pinnedExpanded bool
}

func homeRootPage(page Page) bool {
	return page == PageHome || page == PageTop || page == PageTimeline
}

func (a *App) homeModuleForStack() (string, bool) {
	if len(a.stack) == 0 || !homeRootPage(a.stack[0].Page) {
		return "", false
	}
	for i := len(a.stack) - 1; i >= 0; i-- {
		route := a.stack[i]
		switch route.Page {
		case PageHome:
			if a.sectionTab == "all" {
				return "all", true
			}
			return "favorites", true
		case PageSection:
			return "all", true
		case PageTimeline:
			return "timeline", true
		case PageTop:
			if route.Name == "recommend" {
				return "recommend", true
			}
			return "top", true
		}
	}
	return "", false
}

func copyHomeThread(thread forum.Thread) forum.Thread {
	thread.Articles = append([]forum.Article(nil), thread.Articles...)
	return thread
}

func copyHomeThreadModels(src map[string]*postThreadContent) map[string]*postThreadContent {
	if src == nil {
		return nil
	}
	out := make(map[string]*postThreadContent, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func copyHomeThreadAnchors(src map[listKey]threadAnchor) map[listKey]threadAnchor {
	if src == nil {
		return nil
	}
	out := make(map[listKey]threadAnchor, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func copyHomeThreadViewports(src map[listKey]threadViewportKey) map[listKey]threadViewportKey {
	if src == nil {
		return nil
	}
	out := make(map[listKey]threadViewportKey, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func copyHomeBoolMap(src map[string]bool) map[string]bool {
	if src == nil {
		return nil
	}
	out := make(map[string]bool, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func (a *App) captureHomeSnapshot(module string, ready bool) *homeNavigationSnapshot {
	if a.core == nil {
		return nil
	}
	session := a.core.SessionInfo()
	snapshot := &homeNavigationSnapshot{
		username: session.Username, generation: session.Generation, module: module,
		stack: append([]Route(nil), a.stack...), ready: ready,
		sectionTab: a.sectionTab, favoriteTab: a.favoriteTab, topTab: a.topTab,
		positions: homeListPositions{
			home: a.homeList.Position, section: a.sectionList.Position, timeline: a.timelineList.Position,
			favorite: a.favoriteList.Position, self: a.selfList.Position, compose: a.composeList.Position,
			top: a.topList.Position, recommend: a.recommendList.Position, auxiliary: a.auxList.Position,
			module: a.moduleTabList.Position, moduleKey: a.moduleTabsKey, moduleSelected: a.moduleTabsSelected,
		},
		listPositions: make(map[listKey]layout.Position),
		sections:      append([]forum.Section(nil), a.sections...), section: a.section,
		boards: append([]forum.Board(nil), a.boards...), board: a.board,
		thread: copyHomeThread(a.thread), articles: append([]forum.Article(nil), a.articles...),
		favorite: a.favorite, favoriteBoards: append([]forum.Board(nil), a.favoriteBoards...),
		collection: a.collect, pagination: a.pagination, user: a.user,
		threadModels:      copyHomeThreadModels(a.threadModels),
		threadLoadedRoute: a.threadLoadedRoute, threadLoadedRequest: a.threadLoadedRequest,
		threadAnchors:     copyHomeThreadAnchors(a.threadAnchors),
		threadViewports:   copyHomeThreadViewports(a.threadViewports),
		threadRequestPage: a.threadRequestPage, threadRetryPage: a.threadRetryPage,
		threadRetryRefresh: a.threadRetryRefresh, threadRefreshPending: a.threadRefreshPending,
		pinnedByRoute: copyHomeBoolMap(a.pinnedByRoute),
		expandedPosts: copyHomeBoolMap(a.expandedPosts), pinnedExpanded: a.pinnedExpanded,
	}
	for key, state := range a.listStates {
		for _, route := range snapshot.stack {
			if key.Page == route.Page && key.Name == route.Name && key.ID == route.ID && key.Sub == route.Sub {
				snapshot.listPositions[key] = state.Position
				break
			}
		}
	}
	snapshot.anchor = a.captureHomeAnchor(a.current())
	return snapshot
}

func (a *App) saveHomeSnapshot(module string, ready bool) {
	if module == "" || len(a.stack) == 0 || !homeRootPage(a.stack[0].Page) {
		return
	}
	if a.homeSnapshots == nil {
		a.homeSnapshots = make(map[string]*homeNavigationSnapshot)
	}
	a.homeSnapshots[module] = a.captureHomeSnapshot(module, ready)
	a.lastHomeModule = module
}

func (a *App) restoreHomeSnapshot(module string) bool {
	if a.core == nil || a.homeSnapshots == nil {
		return false
	}
	snapshot := a.homeSnapshots[module]
	if snapshot == nil {
		return false
	}
	session := a.core.SessionInfo()
	if session.Generation != snapshot.generation || session.Username != snapshot.username {
		delete(a.homeSnapshots, module)
		if a.lastHomeModule == module {
			a.lastHomeModule = ""
		}
		return false
	}
	a.stack = append([]Route(nil), snapshot.stack...)
	a.sectionTab, a.favoriteTab, a.topTab = snapshot.sectionTab, snapshot.favoriteTab, snapshot.topTab
	a.sections = append([]forum.Section(nil), snapshot.sections...)
	a.section = snapshot.section
	a.boards = append([]forum.Board(nil), snapshot.boards...)
	a.board = snapshot.board
	a.thread = copyHomeThread(snapshot.thread)
	a.articles = append([]forum.Article(nil), snapshot.articles...)
	a.favorite = snapshot.favorite
	a.favoriteBoards = append([]forum.Board(nil), snapshot.favoriteBoards...)
	a.collect = snapshot.collection
	a.pagination = snapshot.pagination
	a.user = snapshot.user
	a.threadModels = copyHomeThreadModels(snapshot.threadModels)
	a.threadLoadedRoute, a.threadLoadedRequest = snapshot.threadLoadedRoute, snapshot.threadLoadedRequest
	a.threadAnchors = copyHomeThreadAnchors(snapshot.threadAnchors)
	a.threadViewports = copyHomeThreadViewports(snapshot.threadViewports)
	a.threadRequestPage, a.threadRetryPage = snapshot.threadRequestPage, snapshot.threadRetryPage
	a.threadRetryRefresh, a.threadRefreshPending = snapshot.threadRetryRefresh, snapshot.threadRefreshPending
	a.pinnedByRoute = copyHomeBoolMap(snapshot.pinnedByRoute)
	a.expandedPosts = copyHomeBoolMap(snapshot.expandedPosts)
	a.pinnedExpanded = snapshot.pinnedExpanded
	a.homeRestorePending = snapshot
	a.homeRestoreWait = !snapshot.ready
	a.lastHomeModule = module
	a.failed = false
	a.message = ""
	a.window.Invalidate()
	if a.homeRestoreWait {
		a.restoreRoute(a.current())
	}
	return true
}

func (a *App) applyHomeSnapshotPositions(snapshot *homeNavigationSnapshot) {
	if snapshot == nil {
		return
	}
	p := snapshot.positions
	a.homeList.Position = p.home
	a.sectionList.Position = p.section
	a.timelineList.Position = p.timeline
	a.favoriteList.Position = p.favorite
	a.selfList.Position = p.self
	a.composeList.Position = p.compose
	a.topList.Position = p.top
	a.recommendList.Position = p.recommend
	a.auxList.Position = p.auxiliary
	a.moduleTabList.Position = p.module
	a.moduleTabsKey, a.moduleTabsSelected = p.moduleKey, p.moduleSelected
	for key, position := range snapshot.listPositions {
		a.mobileList(key).Position = position
	}
	a.pinnedByRoute = copyHomeBoolMap(snapshot.pinnedByRoute)
	a.expandedPosts = copyHomeBoolMap(snapshot.expandedPosts)
	a.pinnedExpanded = snapshot.pinnedExpanded
	a.restoreHomeAnchor(snapshot.anchor)
}

func (a *App) applyHomeSnapshotAfterLayout() {
	snapshot := a.homeRestorePending
	if snapshot == nil || a.homeRestoreWait || a.current() != snapshot.stack[len(snapshot.stack)-1] {
		return
	}
	a.applyHomeSnapshotPositions(snapshot)
	a.homeRestorePending = nil
}

func (a *App) finishHomeSnapshotReload() {
	snapshot := a.homeRestorePending
	if snapshot == nil || !a.homeRestoreWait || a.current() != snapshot.stack[len(snapshot.stack)-1] {
		return
	}
	a.applyHomeSnapshotPositions(snapshot)
	a.homeRestorePending = nil
	a.homeRestoreWait = false
}

func (a *App) navigationRows(route Route) []string {
	var rows []string
	switch route.Page {
	case PageHome:
		if a.sectionTab == "favorites" {
			rows = []string{"section-actions", "favorite-boards-title"}
			for _, board := range a.favoriteBoards {
				rows = append(rows, "board:"+string(board.Name))
			}
			return rows
		}
		rows = []string{"section-actions", "home-overview", "sections-title"}
		for _, section := range a.sections {
			rows = append(rows, "section:"+string(section.Name))
		}
		return rows
	case PageSection:
		if route.Sub != "pick" {
			rows = append(rows, "section-actions")
		}
		rows = append(rows, "section-header")
		if route.Name == "" {
			for _, section := range a.sections {
				rows = append(rows, "section:"+string(section.Name))
			}
			return rows
		}
		for _, section := range a.section.Subsections {
			rows = append(rows, "subsection:"+string(section.Name))
		}
		for _, board := range a.boards {
			rows = append(rows, "board:"+string(board.Name))
		}
		return rows
	case PageBoard:
		return a.boardNavigationRows(route)
	case PageThread:
		rows = append(rows, "thread-header")
		for _, article := range a.thread.Articles {
			rows = append(rows, "article:"+string(article.ID))
		}
		return rows
	case PageTop, PageTimeline:
		for _, article := range a.articles {
			rows = append(rows, "article:"+string(article.ID))
		}
	}
	return rows
}

func (a *App) boardNavigationRows(route Route) []string {
	var pinned, normal []forum.Article
	for _, article := range a.board.Articles {
		if bool(article.IsTop) {
			pinned = append(pinned, article)
		} else {
			normal = append(normal, article)
		}
	}
	rows := make([]string, 0, len(a.board.Articles)+1)
	expanded := a.boardPinnedExpanded(route)
	if len(pinned) > 0 {
		rows = append(rows, "pinned-summary")
		if expanded {
			for _, article := range pinned {
				rows = append(rows, "article:"+string(article.ID))
			}
		}
	}
	for _, article := range normal {
		rows = append(rows, "article:"+string(article.ID))
	}
	return rows
}

func (a *App) captureHomeAnchor(route Route) homeListAnchor {
	anchor := homeListAnchor{route: route}
	var position layout.Position
	switch route.Page {
	case PageHome:
		position = a.homeList.Position
	case PageSection:
		position = a.sectionList.Position
	case PageBoard:
		position = a.mobileList(listKey{Page: PageBoard, Name: route.Name, Sub: route.Sub, Number: max(1, route.Number)}).Position
	case PageThread:
		key := listKey{Page: PageThread, Name: route.Name, ID: route.ID, Sub: route.Sub, Number: max(1, route.Number)}
		position = a.mobileList(key).Position
		if saved, ok := a.threadAnchors[key]; ok && saved.ArticleID != "" {
			anchor.itemID, anchor.row, anchor.offset = "article:"+saved.ArticleID, saved.Row, saved.Offset
			return anchor
		}
	case PageTop:
		if route.Name == "recommend" {
			position = a.recommendList.Position
		} else {
			position = a.topList.Position
		}
	case PageTimeline:
		position = a.timelineList.Position
	default:
		return anchor
	}
	anchor.row, anchor.offset = position.First, position.Offset
	rows := a.navigationRows(route)
	if anchor.row >= 0 && anchor.row < len(rows) {
		anchor.itemID = rows[anchor.row]
	}
	return anchor
}

func (a *App) restoreHomeAnchor(anchor homeListAnchor) {
	if anchor.route.Page == 0 {
		return
	}
	row := anchor.row
	if anchor.itemID != "" {
		for i, item := range a.navigationRows(anchor.route) {
			if item == anchor.itemID {
				row = i
				break
			}
		}
	}
	position := layout.Position{First: row, Offset: anchor.offset}
	switch anchor.route.Page {
	case PageHome:
		a.homeList.Position = position
	case PageSection:
		a.sectionList.Position = position
	case PageBoard:
		key := listKey{Page: PageBoard, Name: anchor.route.Name, Sub: anchor.route.Sub, Number: max(1, anchor.route.Number)}
		a.mobileList(key).Position = position
	case PageThread:
		key := listKey{Page: PageThread, Name: anchor.route.Name, ID: anchor.route.ID, Sub: anchor.route.Sub, Number: max(1, anchor.route.Number)}
		a.mobileList(key).Position = position
		if strings.HasPrefix(anchor.itemID, "article:") {
			id := strings.TrimPrefix(anchor.itemID, "article:")
			a.threadAnchors[key] = threadAnchor{Row: row, Offset: anchor.offset, ArticleID: id}
		}
	case PageTop:
		if anchor.route.Name == "recommend" {
			a.recommendList.Position = position
		} else {
			a.topList.Position = position
		}
	case PageTimeline:
		a.timelineList.Position = position
	}
}
