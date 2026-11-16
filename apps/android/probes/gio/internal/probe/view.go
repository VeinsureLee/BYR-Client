// Package probe is an offline framework experiment, not an application page or
// a proposed forum navigation flow. It must remain outside production modules.
package probe

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font/opentype"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/resources/fonts"
)

const TotalRows = 2004

type View struct {
	Core            *forum.Client
	Theme           *material.Theme
	Editor          widget.Editor
	Selection       widget.Selectable
	List            widget.List
	DrawToggle      widget.Clickable
	Highlighted     bool
	LastLayoutCount int
	FontLicense     string
}

func New() (*View, error) {
	client, err := forum.New(forum.Options{})
	if err != nil {
		return nil, err
	}
	faces, err := opentype.ParseCollection(fonts.Regular)
	if err != nil {
		return nil, err
	}
	theme := material.NewTheme()
	theme.Shaper = text.NewShaper(text.WithCollection(faces), text.NoSystemFonts())
	theme.Face = faces[0].Font.Typeface
	theme.Bg = color.NRGBA{R: 248, G: 249, B: 252, A: 255}
	view := &View{Core: client, Theme: theme, FontLicense: fonts.License, List: widget.List{List: layout.List{Axis: layout.Vertical}}}
	view.Editor.SetText("中文输入验证：请尝试拼音、选词、换行和修改光标位置。")
	return view, nil
}

func (v *View) Layout(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, v.Theme.Bg)
	v.LastLayoutCount = 0
	for v.DrawToggle.Clicked(gtx) {
		v.Highlighted = !v.Highlighted
	}
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.List(v.Theme, &v.List).Layout(gtx, TotalRows, func(gtx layout.Context, index int) layout.Dimensions {
			v.LastLayoutCount++
			return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				switch index {
				case 0:
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(material.H6(v.Theme, "Gio 技术验证 · 非正式客户端").Layout),
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Rigid(material.Body2(v.Theme, "仅验证共享 Go 核心接入、中文输入、文本选择、2000 段长列表和自定义绘制；不登录、不访问论坛。").Layout),
					)
				case 1:
					return material.Editor(v.Theme, &v.Editor, "输入验证").Layout(gtx)
				case 2:
					label := material.Body1(v.Theme, "文本选择验证：可尝试拖动选区和复制。中文 English 123；手机上的长按、选区手柄及菜单需要实机检查。")
					label.State = &v.Selection
					return label.Layout(gtx)
				case 3:
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(material.Button(v.Theme, &v.DrawToggle, "切换自定义绘制状态").Layout),
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Rigid(v.drawPanel),
					)
				default:
					return material.Body1(v.Theme, fmt.Sprintf("测试段落 %d / 2000\n这是一段离线中文排版样本，用于验证快速滚动、可变行高和重排。这里不代表未来论坛的楼层、卡片或页面设计。", index-3)).Layout(gtx)
				}
			})
		})
	})
}

func (v *View) drawPanel(gtx layout.Context) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(88)))
	background := color.NRGBA{R: 42, G: 95, B: 169, A: 255}
	if v.Highlighted {
		background = color.NRGBA{R: 27, G: 131, B: 95, A: 255}
	}
	paint.FillShape(gtx.Ops, background, clip.RRect{Rect: image.Rectangle{Max: size}, SE: 12, SW: 12, NE: 12, NW: 12}.Op(gtx.Ops))
	return layout.Dimensions{Size: size}
}
