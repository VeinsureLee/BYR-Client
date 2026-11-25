package ui

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

var sectionModuleOrder = []string{"all", "favorites", "timeline", "top", "recommend"}

type sectionSwipeState struct {
	active  bool
	pointer pointer.ID
	start   f32.Point
	route   Route
}

func sectionSwipeDirection(dx, dy, threshold float32) int {
	if math.Abs(float64(dx)) < float64(threshold) || math.Abs(float64(dx)) <= math.Abs(float64(dy))*1.25 {
		return 0
	}
	if dx < 0 {
		return 1
	}
	return -1
}

func (a *App) sectionSwipeLayout(gtx layout.Context, child layout.Widget) layout.Dimensions {
	if !a.sectionSwipeEnabled() {
		a.sectionSwipe.active = false
		return child(gtx)
	}
	bounds := image.Rect(0, gtx.Dp(unit.Dp(48)), gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	area := clip.Rect(bounds).Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &a.sectionSwipeTag)
	pass.Pop()
	area.Pop()

	for {
		e, ok := gtx.Source.Event(pointer.Filter{
			Target: &a.sectionSwipeTag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Move | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		pe, ok := e.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			a.sectionSwipe = sectionSwipeState{active: true, pointer: pe.PointerID, start: pe.Position, route: a.current()}
		case pointer.Drag, pointer.Move:
			if a.sectionSwipe.active && a.sectionSwipe.pointer == pe.PointerID && a.current() != a.sectionSwipe.route {
				a.sectionSwipe.active = false
			}
		case pointer.Release:
			if a.sectionSwipe.active && a.sectionSwipe.pointer == pe.PointerID && a.current() == a.sectionSwipe.route {
				threshold := float32(gtx.Dp(unit.Dp(64)))
				direction := sectionSwipeDirection(pe.Position.X-a.sectionSwipe.start.X, pe.Position.Y-a.sectionSwipe.start.Y, threshold)
				if direction != 0 {
					a.shiftSectionModule(direction)
				}
			}
			a.sectionSwipe.active = false
		case pointer.Cancel:
			a.sectionSwipe.active = false
		}
	}
	passChildren := pointer.PassOp{}.Push(gtx.Ops)
	dims := child(gtx)
	passChildren.Pop()
	return dims
}

func (a *App) sectionSwipeEnabled() bool {
	if a.busy || len(a.stack) == 0 {
		return false
	}
	switch a.current().Page {
	case PageHome, PageTimeline, PageTop:
		return true
	default:
		return false
	}
}

func (a *App) currentSectionModule() string {
	route := a.current()
	switch route.Page {
	case PageHome:
		if a.sectionTab == "all" {
			return "all"
		}
		return "favorites"
	case PageSection:
		return "all"
	case PageTimeline:
		return "timeline"
	case PageTop:
		if route.Name == "recommend" {
			return "recommend"
		}
		return "top"
	default:
		return ""
	}
}

// shiftSectionModule accepts +1 for a leftward finger swipe (next module) and
// -1 for a rightward finger swipe (previous module).
func (a *App) shiftSectionModule(delta int) {
	current := a.currentSectionModule()
	index := -1
	for i, key := range sectionModuleOrder {
		if key == current {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	next := index + delta
	if next < 0 || next >= len(sectionModuleOrder) {
		return
	}
	a.navigateSectionModule(sectionModuleOrder[next])
}

func (a *App) navigateSectionModule(key string) {
	if a.busy {
		return
	}
	target := Route{Number: 1}
	switch key {
	case "all", "favorites":
		target.Page = PageHome
	case "timeline":
		target.Page = PageTimeline
	case "top":
		target.Page = PageTop
	case "recommend":
		target.Page, target.Name = PageTop, "recommend"
	default:
		return
	}
	current := a.current()
	currentModule, rootedInHome := a.homeModuleForStack()
	if rootedInHome && currentModule == key || !rootedInHome && current.Page == target.Page && a.currentSectionModule() == key {
		return
	}
	if rootedInHome {
		a.saveHomeSnapshot(currentModule, !a.failed)
		if a.restoreHomeSnapshot(key) {
			return
		}
	}

	if target.Page == PageHome {
		found := -1
		for i := len(a.stack) - 1; i >= 0; i-- {
			if a.stack[i].Page == PageHome {
				found = i
				break
			}
		}
		if found >= 0 {
			a.stack = a.stack[:found+1]
			a.stack[found] = target
		} else if len(a.stack) > 0 {
			a.stack[len(a.stack)-1] = target
		} else {
			a.stack = []Route{target}
		}
	} else {
		found := -1
		for i := len(a.stack) - 2; i >= 0; i-- {
			if a.stack[i].Page == target.Page && (target.Page != PageTop || a.stack[i].Name == target.Name) {
				found = i
				break
			}
		}
		if found >= 0 {
			a.stack = a.stack[:found+1]
			a.stack[found] = target
		} else if current.Page == target.Page && len(a.stack) > 0 {
			a.stack[len(a.stack)-1] = target
		} else if current.Page == PageHome {
			a.stack = append(a.stack, target)
		} else if len(a.stack) > 0 {
			a.stack[len(a.stack)-1] = target
		} else {
			a.stack = []Route{target}
		}
	}

	if key == "all" || key == "favorites" {
		a.sectionTab = key
		if key == "favorites" {
			a.favoriteTab = "boards"
		}
	} else if key == "top" || key == "recommend" {
		a.topTab = key
	}
	if rootedInHome || target.Page == PageHome {
		a.lastHomeModule = key
	}
	a.failed = false
	a.message = ""
	a.clearRouteModel(target)
	switch target.Page {
	case PageHome:
		if key == "favorites" {
			a.loadFavorites()
		} else if len(a.sections) == 0 {
			a.loadHome()
		}
	case PageTimeline:
		a.loadTimeline()
	case PageTop:
		a.loadTop()
	}
	a.window.Invalidate()
}
