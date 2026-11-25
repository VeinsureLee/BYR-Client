package ui

import (
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// moduleTab is the small, horizontally scrollable module switch used by the
// mobile shell. Key is the route-level value; Selected is kept on the model so
// callers can construct strips without duplicating selection styling.
type moduleTab struct {
	Key      string
	Label    string
	Selected bool
}

const moduleTabGroupSeparator = "\x00"

// moduleTabs lays out a fixed-height horizontal list. Children keep their
// intrinsic width (with a 48dp minimum touch target), so a narrow phone scrolls
// the strip instead of compressing labels or allowing them to paint outside
// the window.
func (a *App) moduleTabs(gtx layout.Context, items []moduleTab, selected string, onSelect func(string)) layout.Dimensions {
	if len(items) == 0 {
		return layout.Dimensions{}
	}

	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = item.Key
	}
	group := strings.Join(keys, moduleTabGroupSeparator)
	if group != a.moduleTabsKey {
		a.moduleTabsKey = group
		a.moduleTabsSelected = ""
		a.moduleTabList = newHorizontalModuleTabList()
	}
	if selected != "" && selected != a.moduleTabsSelected {
		for i, item := range items {
			if item.Key == selected {
				a.moduleTabList.ScrollTo(i)
				break
			}
		}
		a.moduleTabsSelected = selected
	}

	height := gtx.Dp(unit.Dp(48))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	a.moduleTabList.Gap = gtx.Dp(unit.Dp(6))
	return a.moduleTabList.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions {
		item := items[i]
		selected := item.Selected || item.Key == selected
		dims, clicked := a.moduleTab(gtx, group, item.Key, item.Label, selected)
		if clicked && onSelect != nil {
			onSelect(item.Key)
		}
		return dims
	})
}

func newHorizontalModuleTabList() widget.List {
	return widget.List{List: layout.List{Axis: layout.Horizontal}}
}

func (a *App) moduleTab(gtx layout.Context, group, key, label string, selected bool) (layout.Dimensions, bool) {
	clickKey := legacyModuleTabClickKey(group, key)
	btn := a.click(clickKey)
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		dims, labelCall := a.measureModuleTabLabel(gtx, label, selected)
		size := moduleTabSize(gtx, dims.Size)
		bg, _ := moduleTabColors(selected)
		paint.FillShape(gtx.Ops, bg, clip.RRect{
			Rect: image.Rectangle{Max: size}, NE: 16, NW: 16, SE: 16, SW: 16,
		}.Op(gtx.Ops))
		op.Offset(centeredTextOffset(size, dims.Size)).Add(gtx.Ops)
		labelCall.Add(gtx.Ops)
		return layout.Dimensions{Size: size}
	})
	return dims, clicked
}

func (a *App) measureModuleTabLabel(gtx layout.Context, label string, selected bool) (layout.Dimensions, op.CallOp) {
	labelStyle := material.Label(a.theme, unit.Sp(12), label)
	_, fg := moduleTabColors(selected)
	labelStyle.Color = fg
	labelGtx := gtx
	labelGtx.Constraints.Min = image.Point{}
	labelGtx.Constraints.Max = image.Pt(1<<30, labelGtx.Constraints.Max.Y)
	macro := op.Record(gtx.Ops)
	dims := labelStyle.Layout(labelGtx)
	return dims, macro.Stop()
}

func moduleTabSize(gtx layout.Context, label image.Point) image.Point {
	width := label.X + gtx.Dp(unit.Dp(24))
	if min := gtx.Dp(unit.Dp(48)); width < min {
		width = min
	}
	height := label.Y + gtx.Dp(unit.Dp(10))
	if min := gtx.Dp(unit.Dp(48)); height < min {
		height = min
	}
	return image.Pt(width, height)
}

// Keep the existing click identifiers stable while giving moduleTab.Key a
// compact route value. This means old accessibility/programmatic click paths
// remain valid during the shared-strip migration.
func legacyModuleTabClickKey(group, key string) string {
	switch group {
	case "all\x00favorites\x00timeline\x00top", "all\x00favorites\x00timeline\x00top\x00recommend":
		return "sections-" + key
	case "ai\x00all\x00user":
		return "search-mode:" + key
	case "top\x00recommend":
		return "top-tab:" + key
	case "boards\x00collection":
		return "fav-tab:" + key
	case "contacts\x00inbox\x00outbox\x00refers":
		return "msg-tab:" + key
	case "reply\x00at\x00read-all":
		switch key {
		case "reply", "at":
			return "refer-kind:" + key
		case "read-all":
			return "refer-read-all"
		}
	}
	return "module-tab:" + key
}
