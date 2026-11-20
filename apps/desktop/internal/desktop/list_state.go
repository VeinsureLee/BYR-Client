package desktop

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

type listKey struct {
	Page                 Page
	Name, ID, Sub, Query string
	Number               int
}

func (a *App) desktopListState(key listKey) *ui.ListState {
	if a.listStates == nil {
		a.listStates = make(map[listKey]*ui.ListState)
	}
	state := a.listStates[key]
	if state == nil {
		state = &ui.ListState{}
		a.listStates[key] = state
	}
	return state
}

func (a *App) articleListKey() listKey {
	key := listKey{Page: a.route.Page, Name: a.route.Name, Sub: a.route.Sub, Number: a.route.Number}
	if a.route.Page == "search" {
		key.Query = strings.TrimSpace(a.searchQuery)
	}
	return key
}

func (a *App) threadListKey() listKey {
	return listKey{Page: a.route.Page, Name: a.route.Name, ID: a.route.ID, Sub: a.route.Sub, Number: a.route.Number}
}

func (a *App) expandedPostState(key listKey) map[string]bool {
	if a.expandedPosts == nil {
		a.expandedPosts = make(map[listKey]map[string]bool)
	}
	state := a.expandedPosts[key]
	if state == nil {
		state = make(map[string]bool)
		a.expandedPosts[key] = state
	}
	return state
}

func (a *App) setPinnedExpanded(key listKey, expanded bool) {
	if a.pinnedExpanded == nil {
		a.pinnedExpanded = make(map[listKey]bool)
	}
	a.pinnedExpanded[key] = expanded
}

func (a *App) isPinnedExpanded(key listKey) bool {
	return a.pinnedExpanded != nil && a.pinnedExpanded[key]
}
