package ui

import (
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

var (
	colorSurface     = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorBorder      = color.NRGBA{R: 0xE2, G: 0xE8, B: 0xF2, A: 0xFF}
	colorMuted       = color.NRGBA{R: 0x6C, G: 0x7C, B: 0x93, A: 0xFF}
	colorAccent      = color.NRGBA{R: 0x0A, G: 0x5C, B: 0xBF, A: 0xFF}
	colorAccentSoft  = color.NRGBA{R: 0xE8, G: 0xF0, B: 0xFC, A: 0xFF}
	colorDanger      = color.NRGBA{R: 0xB3, G: 0x26, B: 0x1E, A: 0xFF}
	colorDangerSoft  = color.NRGBA{R: 0xFD, G: 0xEC, B: 0xEC, A: 0xFF}
	colorWarning     = color.NRGBA{R: 0xB2, G: 0x6A, B: 0x00, A: 0xFF}
	colorWarningSoft = color.NRGBA{R: 0xFF, G: 0xF4, B: 0xDE, A: 0xFF}
	colorBg          = color.NRGBA{R: 0xF4, G: 0xF7, B: 0xFC, A: 0xFF}
	colorFg          = color.NRGBA{R: 0x1E, G: 0x2B, B: 0x40, A: 0xFF}
	colorPill        = color.NRGBA{R: 0xF1, G: 0xF5, B: 0xFB, A: 0xFF}
)

// setPalette switches the shared surfaces between the light and dark desktop
// themes. Colors are package values because every mobile widget consumes the
// same small set; keeping one palette avoids per-widget theme plumbing.
func setPalette(dark bool) {
	if dark {
		colorSurface = color.NRGBA{R: 0x18, G: 0x22, B: 0x2F, A: 0xFF}
		colorBorder = color.NRGBA{R: 0x2C, G: 0x3A, B: 0x4C, A: 0xFF}
		colorMuted = color.NRGBA{R: 0x9A, G: 0xAA, B: 0xBF, A: 0xFF}
		colorAccent = color.NRGBA{R: 0x4C, G: 0x8F, B: 0xE8, A: 0xFF}
		colorAccentSoft = color.NRGBA{R: 0x1C, G: 0x2E, B: 0x46, A: 0xFF}
		colorDanger = color.NRGBA{R: 0xF0, G: 0x8A, B: 0x82, A: 0xFF}
		colorDangerSoft = color.NRGBA{R: 0x3A, G: 0x22, B: 0x25, A: 0xFF}
		colorWarning = color.NRGBA{R: 0xE8, G: 0xB0, B: 0x5C, A: 0xFF}
		colorWarningSoft = color.NRGBA{R: 0x3A, G: 0x2F, B: 0x1A, A: 0xFF}
		colorBg = color.NRGBA{R: 0x0E, G: 0x16, B: 0x20, A: 0xFF}
		colorFg = color.NRGBA{R: 0xE6, G: 0xED, B: 0xF5, A: 0xFF}
		colorPill = color.NRGBA{R: 0x22, G: 0x30, B: 0x43, A: 0xFF}
		return
	}
	colorSurface = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorBorder = color.NRGBA{R: 0xE2, G: 0xE8, B: 0xF2, A: 0xFF}
	colorMuted = color.NRGBA{R: 0x6C, G: 0x7C, B: 0x93, A: 0xFF}
	colorAccent = color.NRGBA{R: 0x0A, G: 0x5C, B: 0xBF, A: 0xFF}
	colorAccentSoft = color.NRGBA{R: 0xE8, G: 0xF0, B: 0xFC, A: 0xFF}
	colorDanger = color.NRGBA{R: 0xB3, G: 0x26, B: 0x1E, A: 0xFF}
	colorDangerSoft = color.NRGBA{R: 0xFD, G: 0xEC, B: 0xEC, A: 0xFF}
	colorWarning = color.NRGBA{R: 0xB2, G: 0x6A, B: 0x00, A: 0xFF}
	colorWarningSoft = color.NRGBA{R: 0xFF, G: 0xF4, B: 0xDE, A: 0xFF}
	colorBg = color.NRGBA{R: 0xF4, G: 0xF7, B: 0xFC, A: 0xFF}
	colorFg = color.NRGBA{R: 0x1E, G: 0x2B, B: 0x40, A: 0xFF}
	colorPill = color.NRGBA{R: 0xF1, G: 0xF5, B: 0xFB, A: 0xFF}
}

func (a *App) iconTarget(gtx layout.Context, name string) layout.Dimensions {
	px := gtx.Dp(unit.Dp(40))
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.drawIcon(gtx, name, 22, a.theme.Fg)
	})
}

func (a *App) drawIcon(gtx layout.Context, name string, size unit.Dp, fg color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	icon := a.icons[name]
	if icon == nil {
		return layout.Dimensions{Size: image.Pt(px, px)}
	}
	return icon.Layout(gtx, fg)
}

func (a *App) iconButton(gtx layout.Context, key, name string) (layout.Dimensions, bool) {
	btn := a.click(key)
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.iconTarget(gtx, name)
	})
	return dims, clicked
}

// pill draws a compact rounded action and returns whether it was clicked.
// Selection uses the accent tint; negative selections use the danger tint.
func (a *App) pill(gtx layout.Context, key, label, icon string, selected, negative bool) (layout.Dimensions, bool) {
	btn := a.click(key)
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		inner := layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(11), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if icon == "" {
						return layout.Dimensions{}
					}
					return a.drawIcon(gtx, icon, 16, pillColor(selected, negative))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if icon == "" {
						return layout.Dimensions{}
					}
					return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, unit.Sp(13), label)
					l.Color = pillColor(selected, negative)
					return l.Layout(gtx)
				}),
			)
		})
		call := m.Stop()
		size := inner.Size
		bg := colorPill
		if selected {
			bg = colorAccentSoft
			if negative {
				bg = colorDangerSoft
			}
		}
		paint.FillShape(gtx.Ops, bg, clip.RRect{Rect: image.Rectangle{Max: size}, NE: 16, NW: 16, SE: 16, SW: 16}.Op(gtx.Ops))
		call.Add(gtx.Ops)
		return inner
	})
	return dims, clicked
}

// compactIconAction keeps frequent floor actions icon-first. The description is
// emitted as semantic metadata so the compact visual does not remove the
// accessible action name; counts are shown only when they carry information.
func (a *App) compactIconAction(gtx layout.Context, key, description, icon, selectedIcon string, count int, selected, negative bool) (layout.Dimensions, bool) {
	btn := a.click(key)
	clicked := btn.Clicked(gtx)
	iconName := icon
	if selected && selectedIcon != "" {
		iconName = selectedIcon
	}
	fg := pillColor(selected, negative)
	bg := colorPill
	if selected {
		bg = colorAccentSoft
		if negative {
			bg = colorDangerSoft
		}
	}
	if btn.Pressed() {
		bg = pressedSurface(bg)
	}
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		if description != "" {
			semantic.DescriptionOp(description).Add(gtx.Ops)
		}
		m := op.Record(gtx.Ops)
		inner := layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.drawIcon(gtx, iconName, 18, fg)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if count <= 0 {
						return layout.Dimensions{}
					}
					return layout.Spacer{Width: unit.Dp(4)}.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if count <= 0 {
						return layout.Dimensions{}
					}
					return a.badgeChip(gtx, itoa(count), bg, fg)
				}),
			)
		})
		call := m.Stop()
		paint.FillShape(gtx.Ops, bg, clip.RRect{Rect: image.Rectangle{Max: inner.Size}, NE: 12, NW: 12, SE: 12, SW: 12}.Op(gtx.Ops))
		call.Add(gtx.Ops)
		return inner
	})
	return dims, clicked
}

func pressedSurface(bg color.NRGBA) color.NRGBA {
	return color.NRGBA{
		R: uint8(int(bg.R) * 82 / 100),
		G: uint8(int(bg.G) * 82 / 100),
		B: uint8(int(bg.B) * 82 / 100),
		A: bg.A,
	}
}

func pillColor(selected, negative bool) color.NRGBA {
	if selected {
		if negative {
			return colorDanger
		}
		return colorAccent
	}
	return colorMuted
}

func moduleTabColors(selected bool) (color.NRGBA, color.NRGBA) {
	if selected {
		return colorAccentSoft, colorAccent
	}
	return colorPill, colorMuted
}

func (a *App) filledPill(gtx layout.Context, key, label, icon string) (layout.Dimensions, bool) {
	btn := a.click(key)
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		inner := layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(13), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.drawIcon(gtx, icon, 16, a.theme.ContrastFg)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, unit.Sp(13), label)
					l.Color = a.theme.ContrastFg
					return l.Layout(gtx)
				}),
			)
		})
		call := m.Stop()
		size := inner.Size
		paint.FillShape(gtx.Ops, a.theme.ContrastBg, clip.RRect{Rect: image.Rectangle{Max: size}, NE: 16, NW: 16, SE: 16, SW: 16}.Op(gtx.Ops))
		call.Add(gtx.Ops)
		return inner
	})
	return dims, clicked
}

func (a *App) badgeChip(gtx layout.Context, label string, bg, fg color.NRGBA) layout.Dimensions {
	return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		inner := layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Label(a.theme, unit.Sp(10), label)
			l.Color = fg
			return l.Layout(gtx)
		})
		call := m.Stop()
		paint.FillShape(gtx.Ops, bg, clip.RRect{Rect: image.Rectangle{Max: inner.Size}, NE: 5, NW: 5, SE: 5, SW: 5}.Op(gtx.Ops))
		call.Add(gtx.Ops)
		return inner
	})
}

func (a *App) avatarCircle(gtx layout.Context, name string, size unit.Dp, bg, fg color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	paint.FillShape(gtx.Ops, bg, clip.Ellipse(image.Rectangle{Max: image.Pt(px, px)}).Op(gtx.Ops))
	initial := "?"
	if r := []rune(name); len(r) > 0 {
		initial = string(r[0])
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Label(a.theme, unit.Sp(float32(size)*0.42), initial)
		l.Color = fg
		l.Font.Weight = 700
		return l.Layout(gtx)
	})
}

// iconTile is a rounded accent square holding a small icon, used for boards
// and subsections.
func (a *App) iconTile(gtx layout.Context, icon string, size unit.Dp, bg, fg color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	paint.FillShape(gtx.Ops, bg, clip.RRect{Rect: image.Rectangle{Max: image.Pt(px, px)}, NE: 12, NW: 12, SE: 12, SW: 12}.Op(gtx.Ops))
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.drawIcon(gtx, icon, size*0.48, fg)
	})
}

func (a *App) secondaryButton(gtx layout.Context, key, label string) (layout.Dimensions, bool) {
	btn := a.click(key)
	clicked := btn.Clicked(gtx)
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(48)))
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(size)
		paint.FillShape(gtx.Ops, colorBorder, clip.RRect{Rect: image.Rectangle{Max: size}, NE: 12, NW: 12, SE: 12, SW: 12}.Op(gtx.Ops))
		paint.FillShape(gtx.Ops, colorSurface, clip.RRect{Rect: image.Rect(1, 1, size.X-1, size.Y-1), NE: 11, NW: 11, SE: 11, SW: 11}.Op(gtx.Ops))
		labelMacro := op.Record(gtx.Ops)
		l := material.Label(a.theme, unit.Sp(15), label)
		l.Color = colorAccent
		l.Alignment = text.Middle
		labelDims := l.Layout(gtx)
		labelCall := labelMacro.Stop()
		op.Offset(image.Pt((size.X-labelDims.Size.X)/2, (size.Y-labelDims.Size.Y)/2)).Add(gtx.Ops)
		labelCall.Add(gtx.Ops)
		return layout.Dimensions{Size: size}
	})
	return dims, clicked
}

// tile paints a bordered card, optionally with the accent selection tint.
func (a *App) tile(gtx layout.Context, selected bool, content layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	inner := layout.Inset{Top: unit.Dp(13), Bottom: unit.Dp(13), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, content)
	call := m.Stop()
	size := inner.Size
	bg, border := colorSurface, colorBorder
	if selected {
		bg, border = colorAccentSoft, colorAccentSoft
	}
	paint.FillShape(gtx.Ops, border, clip.RRect{Rect: image.Rectangle{Max: size}, NE: 13, NW: 13, SE: 13, SW: 13}.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, bg, clip.RRect{Rect: image.Rect(1, 1, size.X-1, size.Y-1), NE: 12, NW: 12, SE: 12, SW: 12}.Op(gtx.Ops))
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

// articleTile is the shared thread row used by boards, timelines and search.
func (a *App) articleTile(gtx layout.Context, key string, v forum.Article, fallbackBoard string) layout.Dimensions {
	board := v.Board
	if board == "" {
		board = fallbackBoard
	}
	title := v.Title
	if title == "" {
		title = "无标题"
	}
	author := v.Author.DisplayName()
	meta := board
	if v.ReplyCount > 0 {
		meta += " · " + itoa(int(v.ReplyCount)) + " 回复"
	}
	if t := formatTime(v.PostTime); t != "" {
		meta += " · " + t
	}
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					avatar := func(gtx layout.Context) layout.Dimensions {
						return a.avatar(gtx, v.Author, 34)
					}
					if v.Author.Anonymous() || v.Author.ID == "" {
						return avatar(gtx)
					}
					btn := a.click("article-user:" + key + ":" + string(v.ID))
					clicked := btn.Clicked(gtx)
					dims := btn.Layout(gtx, avatar)
					if clicked {
						a.openUser(string(v.Author.ID))
					}
					return dims
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(11)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							btn := a.click("article:" + key + ":" + string(v.ID))
							clicked := btn.Clicked(gtx)
							dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												if !bool(v.IsTop) {
													return layout.Dimensions{}
												}
												return a.badgeChip(gtx, "置顶", colorWarningSoft, colorWarning)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												l := material.Label(a.theme, unit.Sp(15), title)
												l.Font.Weight = 600
												l.MaxLines = 2
												l.Truncator = "…"
												return l.Layout(gtx)
											}),
										)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.drawIcon(gtx, "next", 16, colorMuted)
									}),
								)
							})
							if clicked {
								a.openThread(board, threadID(v))
							}
							return dims
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									name := func(gtx layout.Context) layout.Dimensions {
										l := material.Label(a.theme, unit.Sp(11), author)
										l.Color = colorMuted
										l.MaxLines = 1
										return l.Layout(gtx)
									}
									if v.Author.Anonymous() || v.Author.ID == "" {
										return name(gtx)
									}
									btn := a.click("article-author:" + key + ":" + string(v.ID))
									clicked := btn.Clicked(gtx)
									dims := btn.Layout(gtx, name)
									if clicked {
										a.openUser(string(v.Author.ID))
									}
									return dims
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if meta == "" {
										return layout.Dimensions{}
									}
									l := material.Label(a.theme, unit.Sp(11), " · "+meta)
									l.Color = colorMuted
									l.MaxLines = 1
									return l.Layout(gtx)
								}),
							)
						}),
					)
				}),
			)
		})
	})
}

func (a *App) sectionTitle(gtx layout.Context, title, detail string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.theme, unit.Sp(13), title)
				l.Font.Weight = 700
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if detail == "" {
					return layout.Dimensions{}
				}
				l := material.Label(a.theme, unit.Sp(11), detail)
				l.Color = colorMuted
				return l.Layout(gtx)
			}),
		)
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
