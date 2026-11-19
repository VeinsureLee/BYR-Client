package desktop

import "github.com/egoist/mygo/ui"

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
