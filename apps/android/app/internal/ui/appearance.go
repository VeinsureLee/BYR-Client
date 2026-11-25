package ui

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/appearance"
)

func (a *App) schemeOption(gtx layout.Context, scheme appearance.Scheme, selected bool) (layout.Dimensions, bool) {
	palette := appearance.Resolve(scheme, a.dark)
	key := "settings-scheme:" + string(scheme)
	button := a.click(key)
	clicked := button.Clicked(gtx)
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(48)))
	dims := button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp("选择配色 " + appearance.Name(scheme)).Add(gtx.Ops)
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				bg := colorPill
				if selected {
					bg = palette.Selection
				}
				paint.FillShape(gtx.Ops, bg, clip.RRect{Rect: image.Rectangle{Max: size}, NE: 10, NW: 10, SE: 10, SW: 10}.Op(gtx.Ops))
				return layout.Dimensions{Size: size}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							px := gtx.Dp(unit.Dp(16))
							gtx.Constraints = layout.Exact(image.Pt(px, px))
							paint.FillShape(gtx.Ops, palette.Accent, clip.Ellipse(image.Rectangle{Max: image.Pt(px, px)}).Op(gtx.Ops))
							return layout.Dimensions{Size: image.Pt(px, px)}
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(14), appearance.Name(scheme))
							l.Color = colorFg
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !selected {
								return layout.Dimensions{}
							}
							return a.drawIcon(gtx, "check", 16, palette.Accent)
						}),
					)
				})
			}),
		)
	})
	return dims, clicked
}
