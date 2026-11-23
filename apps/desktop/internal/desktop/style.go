package desktop

import (
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/resources/fonts"
	"github.com/egoist/mygo/ui"
)

// The reference client's Material seed is #0A5CBF. These tokens keep every
// native page consistent; the shared business core has no colors or UI types.
func forumTheme(dark bool, scale float32) *ui.Theme {
	t := *ui.LightTheme()
	if dark {
		t = *ui.DarkTheme()
		t.Background, t.Surface = ui.Hex("#111418"), ui.Hex("#191E25")
		t.SurfaceHover, t.SurfacePressed = ui.Hex("#222A35"), ui.Hex("#2B3746")
		t.Border, t.Text, t.TextMuted = ui.Hex("#303946"), ui.Hex("#EFF2F6"), ui.Hex("#BCC7D5")
		t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#9CC5FF"), ui.Hex("#B1D0FF"), ui.Hex("#7EAFE9")
		t.AccentText = ui.Hex("#10243D")
		t.Selection = ui.Hex("#202F43")
		t.Danger, t.Warning, t.Success = ui.Hex("#FFB4AA"), ui.Hex("#F3CB80"), ui.Hex("#8BD6B3")
	} else {
		t.Background, t.Surface = ui.Hex("#F6F7F9"), ui.Hex("#FFFFFF")
		t.SurfaceHover, t.SurfacePressed = ui.Hex("#EFF3F8"), ui.Hex("#E7ECF3")
		t.Border, t.Text, t.TextMuted = ui.Hex("#DDE3EA"), ui.Hex("#18202A"), ui.Hex("#475569")
		t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#0A5CBF"), ui.Hex("#084FA5"), ui.Hex("#073F87")
		t.AccentText = ui.Hex("#FFFFFF")
		t.Selection = ui.Hex("#E7F0FB")
		t.Danger, t.Warning, t.Success = ui.Hex("#A32D29"), ui.Hex("#855300"), ui.Hex("#226456")
	}
	t.Font, t.FontSize, t.Radius, t.Spacing = fonts.Family, 14*scale, 8, 4
	t.Focus, t.InactiveAccent = t.Accent, t.Accent
	return &t
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
	return c.Theme().Surface.Mix(c.Theme().Accent, .10)
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
	ui.Column(c).FillWidth().Padding(32).Gap(10).AlignItems(ui.Center).Children(func() {
		if glyph := icons[icon]; glyph != nil {
			ui.Icon(c, glyph).Size(30, 30).TextColor(c.Theme().TextMuted)
		}
		ui.Text(c, title).Bold().TextColor(c.Theme().TextMuted)
		if detail != "" {
			ui.Text(c, detail).TextColor(c.Theme().TextMuted)
		}
	})
}
