package desktop

import (
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/appearance"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/resources/fonts"
	"github.com/egoist/mygo/ui"
)

// Shared semantic colors keep the native desktop theme aligned with Gio while
// leaving the rendered theme owned by the current desktop window.
func forumTheme(scheme appearance.Scheme, dark bool, scale float32) *ui.Theme {
	palette := appearance.Resolve(scheme, dark)
	t := *ui.LightTheme()
	if dark {
		t = *ui.DarkTheme()
	}
	t.Background = ui.Hex(appearance.Hex(palette.Background))
	t.Dark = dark
	t.Surface = ui.Hex(appearance.Hex(palette.Surface))
	t.SurfaceHover = ui.Hex(appearance.Hex(palette.SurfaceHover))
	t.SurfacePressed = ui.Hex(appearance.Hex(palette.SurfacePressed))
	t.Border = ui.Hex(appearance.Hex(palette.Border))
	t.Text = ui.Hex(appearance.Hex(palette.Text))
	t.TextMuted = ui.Hex(appearance.Hex(palette.TextMuted))
	t.Accent = ui.Hex(appearance.Hex(palette.Accent))
	t.AccentHover = ui.Hex(appearance.Hex(palette.AccentHover))
	t.AccentPressed = ui.Hex(appearance.Hex(palette.AccentPressed))
	t.AccentText = ui.Hex(appearance.Hex(palette.AccentText))
	t.Selection = ui.Hex(appearance.Hex(palette.Selection))
	t.Danger = ui.Hex(appearance.Hex(palette.Danger))
	t.Warning = ui.Hex(appearance.Hex(palette.Warning))
	t.Success = ui.Hex(appearance.Hex(palette.Success))
	t.Font, t.FontSize, t.Radius, t.Spacing = fonts.Family, 14*scale, 8, 4
	t.Focus, t.InactiveAccent = t.Accent, t.Accent
	return &t
}

// forumThemeWithPreferences applies the desktop's accessibility settings while
// keeping the user's selected BYR palette. MyGo's Theme.WithPreferences also
// follows the desktop accent, which would otherwise replace the chosen scheme
// whenever the OS exposes one.
func forumThemeWithPreferences(scheme appearance.Scheme, dark bool, scale float32, prefs ui.Preferences) *ui.Theme {
	palette := appearance.Resolve(scheme, dark)
	theme := forumTheme(scheme, dark, scale).WithPreferences(prefs)
	theme.Accent = ui.Hex(appearance.Hex(palette.Accent))
	theme.AccentHover = ui.Hex(appearance.Hex(palette.AccentHover))
	theme.AccentPressed = ui.Hex(appearance.Hex(palette.AccentPressed))
	theme.AccentText = ui.Hex(appearance.Hex(palette.AccentText))
	theme.Selection = ui.Hex(appearance.Hex(palette.Selection))
	theme.Focus = theme.Accent
	theme.InactiveAccent = theme.Accent
	if prefs.HighContrast {
		theme.Focus.A = 255
	}
	return theme
}

func themedTextInput(c *ui.Context, value *string) ui.Element {
	palette := appearance.Resolve(appearance.BYRBlue, c.Theme().Dark)
	border := appearance.Hex(palette.InputBorder)
	return ui.TextInput(c, value).PlaceholderColor(ui.Hex(appearance.Hex(palette.TextFaint))).Border(1, ui.Hex(border))
}

func themedTextArea(c *ui.Context, value *string) ui.Element {
	palette := appearance.Resolve(appearance.BYRBlue, c.Theme().Dark)
	border := appearance.Hex(palette.InputBorder)
	return ui.TextArea(c, value).PlaceholderColor(ui.Hex(appearance.Hex(palette.TextFaint))).Border(1, ui.Hex(border))
}

// textSize applies the application font scale to explicit typography sizes.
// The system text scale is folded in by View, so each size is scaled once.
func (a *App) textSize(_ *ui.Context, base float32) float32 {
	scale := a.fontScale
	if scale <= 0 {
		scale = 1
	}
	return base * scale
}

func accentSurface(c *ui.Context) ui.Color {
	return c.Theme().Selection
}

func card(c *ui.Context) ui.Element {
	return ui.Column(c).FillWidth().MinWidth(0).Padding(18).Gap(12).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14)
}

type buttonTone uint8

const (
	quiet buttonTone = iota
	filled
	tonal
	selected
	destructive
	negative
)

// A composed native control, with one accessible name and keyboard focus.
// SVG icons stay sharp at any DPI; selection never relies on color alone.
func control(c *ui.Context, label, icon string, tone buttonTone) ui.Element {
	t := c.Theme()
	e := ui.ButtonBase(c).Label(label).Row().AlignItems(ui.Center).Justify(ui.Center).
		Padding(8, 12).Gap(7).Radius(8).MinWidth(0).MinHeight(36)
	bg, fg := ui.Transparent, t.TextMuted
	switch tone {
	case filled:
		bg, fg = t.Accent, t.AccentText
	case tonal, selected:
		bg, fg = accentSurface(c), t.Accent
	case destructive:
		fg = t.Danger
	case negative:
		bg, fg = t.Surface.Mix(t.Danger, .12), t.Danger
	}
	if e.Pressed() {
		if tone == negative {
			bg = bg.Mix(t.Danger, .14)
		} else {
			bg = bg.Mix(t.Accent, .14)
		}
	} else if e.Hovered() {
		switch tone {
		case filled:
			bg = t.AccentHover
		case negative:
			bg = t.Surface.Mix(t.Danger, .16)
		default:
			bg = t.SurfaceHover
		}
	}
	e.Background(bg).TextColor(fg).Selected(tone == selected || tone == negative)
	e.Children(func() {
		if glyph := icons[icon]; glyph != nil {
			ui.Icon(c, glyph).Size(17, 17).Shrink(0)
		}
		ui.Text(c, label).SingleLine()
	})
	return e
}

// reaction is a compact selected-state control for like/dislike. BYR has no
// cancellation endpoint, so a selected reaction is disabled only by the caller
// when another network action is already running.
func (a *App) reaction(c *ui.Context, label, icon string, isSelected, isNegative bool, run func()) {
	a.reactionDisabled(c, label, icon, isSelected, isNegative, a.busy, run)
}

func (a *App) reactionDisabled(c *ui.Context, label, icon string, isSelected, isNegative, disabled bool, run func()) {
	tone := quiet
	switch {
	case isSelected && isNegative:
		tone = negative
	case isSelected:
		tone = selected
	}
	e := control(c, label, icon, tone).Disabled(disabled)
	if e.Clicked() {
		run()
	}
}

// textLink is a title-like button with left alignment, used by list cards.
func textLink(c *ui.Context, label string, run func(), disabled bool) {
	e := ui.ButtonBase(c).Label(label).Row().AlignItems(ui.Center).Justify(ui.Start).
		Grow(1).MinWidth(0).Padding(0).Gap(4).Radius(6).Disabled(disabled)
	if e.Hovered() {
		e.TextColor(c.Theme().Accent)
	} else {
		e.TextColor(c.Theme().Text)
	}
	e.Children(func() { ui.Text(c, label).Bold().MaxLines(2).Ellipsis("…") })
	if e.Clicked() {
		run()
	}
}

func (a *App) action(c *ui.Context, label, icon string, tone buttonTone, run func()) {
	if control(c, label, icon, tone).Disabled(a.busy).Clicked() {
		run()
	}
}

func badge(c *ui.Context, label string, color ui.Color) {
	ui.Text(c, label).FontSize(11).TextColor(color).Padding(3, 7).
		Background(c.Theme().Surface.Mix(color, .10)).Radius(5).SingleLine()
}

func (a *App) avatar(c *ui.Context, u forum.User, size float32) {
	e := ui.ButtonBase(c).Size(size, size).Shrink(0).Radius(size / 2).
		Background(accentSurface(c)).TextColor(c.Theme().Accent).
		Label("查看用户 " + u.DisplayName()).Disabled(a.busy || u.Anonymous() || u.ID == "")
	e.Children(func() {
		if u.FaceURL != "" {
			ui.Column(c).Size(size, size).Clip().Children(func() { a.image(c, u.FaceURL, size, size, false) })
		} else {
			ui.Row(c).Fill().Center().Children(func() {
				name := strings.TrimSpace(u.DisplayName())
				initial := "?"
				if r := []rune(name); len(r) > 0 {
					initial = strings.ToUpper(string(r[0]))
				}
				ui.Text(c, initial).FontSize(size * .4).Bold()
			})
		}
	})
	if e.Clicked() {
		a.navigate(Route{Page: "user", Name: string(u.ID)}, true)
	}
}

func emptyState(c *ui.Context, icon, title, detail string) {
	ui.Column(c).Grow(1).FillWidth().FillHeight().Center().Padding(32).Gap(10).Children(func() {
		if glyph := icons[icon]; glyph != nil {
			ui.Icon(c, glyph).Size(30, 30).TextColor(c.Theme().TextMuted)
		}
		ui.Text(c, title).Bold().TextColor(c.Theme().TextMuted)
		if detail != "" {
			ui.Text(c, detail).TextColor(c.Theme().TextMuted)
		}
	})
}

func (a *App) emptyRequestState(c *ui.Context, icon, title, detail string) {
	if a.busy && a.pageLoading {
		emptyState(c, "clock", "正在加载…", "请稍候，内容加载完成后会显示在这里。")
		return
	}
	if a.failed {
		emptyState(c, "alert", "加载失败", a.message)
		return
	}
	emptyState(c, icon, title, detail)
}
