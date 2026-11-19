package ui

import (
	"gioui.org/layout"
	"gioui.org/widget"
)

type listKey struct {
	Page   Page
	Name   string
	ID     string
	Sub    string
	Query  string
	Number int
}

func (a *App) mobileList(key listKey) *widget.List {
	if a.listStates == nil {
		a.listStates = make(map[listKey]*widget.List)
	}
	if state := a.listStates[key]; state != nil {
		return state
	}
	state := &widget.List{List: layout.List{Axis: layout.Vertical}}
	a.listStates[key] = state
	return state
}
