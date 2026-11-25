package desktop

import (
	"context"
	"image"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/appearance"
	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/egoist/mygo/ui"
)

func systemAccentPreferences() ui.Preferences {
	return ui.Preferences{
		Accent:       ui.Hex("#C43154"),
		HighContrast: true,
		ReduceMotion: true,
		TextScale:    1.2,
	}
}

func countRGB(img *image.RGBA, target ui.Color) int {
	n := 0
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == target.R && uint8(g>>8) == target.G && uint8(b>>8) == target.B {
				n++
			}
		}
	}
	return n
}

func TestDesktopThemeKeepsPaletteUnderSystemAccent(t *testing.T) {
	prefs := systemAccentPreferences()
	for _, scheme := range appearance.Schemes() {
		for _, dark := range []bool{false, true} {
			palette := appearance.Resolve(scheme, dark)
			theme := forumThemeWithPreferences(scheme, dark, 1.2, prefs)
			if theme.Accent != ui.Hex(appearance.Hex(palette.Accent)) ||
				theme.AccentHover != ui.Hex(appearance.Hex(palette.AccentHover)) ||
				theme.AccentPressed != ui.Hex(appearance.Hex(palette.AccentPressed)) ||
				theme.AccentText != ui.Hex(appearance.Hex(palette.AccentText)) ||
				theme.Selection != ui.Hex(appearance.Hex(palette.Selection)) {
				t.Errorf("%s dark=%v system accent replaced the selected palette", scheme, dark)
			}
			if theme.Focus != theme.Accent || theme.Focus.A != 255 {
				t.Errorf("%s dark=%v focus = %+v, want the opaque palette accent", scheme, dark, theme.Focus)
			}
			wantFont := float32(14 * 1.2 * 1.2)
			if diff := theme.FontSize - wantFont; diff < -0.001 || diff > 0.001 {
				t.Errorf("%s dark=%v font size = %v, want system text scale folded in (%v)", scheme, dark, theme.FontSize, wantFont)
			}
		}
	}
}

func TestDesktopSystemAccentDoesNotOverrideRenderedSchemeOrSavedPreference(t *testing.T) {
	for _, scheme := range []appearance.Scheme{appearance.BYRBlue, appearance.Pine, appearance.Plum} {
		for _, themeMode := range []int{1, 2} {
			name := string(scheme) + "-theme" + string(rune('0'+themeMode))
			t.Run(name, func(t *testing.T) {
				h := newHarness(t)
				h.view.SetPreferences(systemAccentPreferences())
				h.login()
				schemeValue := string(scheme)
				if err := h.app.Core.UpdatePreferences(context.Background(), application.PreferencePatch{
					ColorScheme: &schemeValue, Theme: &themeMode,
				}); err != nil {
					t.Fatal(err)
				}
				h.goTo(Route{Page: "search"})
				h.view.Frame()
				if !h.app.reduceMotion {
					t.Fatal("ReduceMotion system preference was not applied")
				}
				if h.app.fontScale < 1.19 {
					t.Fatalf("system TextScale was not folded into fontScale: %v", h.app.fontScale)
				}
				palette := appearance.Resolve(scheme, themeMode == 2)
				img := h.view.Image()
				if got := countRGB(img, ui.Hex(appearance.Hex(palette.Accent))); got == 0 {
					t.Fatalf("rendered UI does not use %s accent %s", scheme, appearance.Hex(palette.Accent))
				}
				if got := countRGB(img, systemAccentPreferences().Accent); got != 0 {
					t.Fatalf("rendered UI used the system accent instead of %s: %d pixels", scheme, got)
				}
			})
		}
	}
}

func TestDesktopSelectedPaletteSurvivesRestartWithSystemAccent(t *testing.T) {
	backend := &desktopHistoryBackend{}
	first := newHarnessWithStorage(t, backend)
	first.login()
	scheme := string(appearance.Plum)
	themeMode := 2
	if err := first.app.Core.UpdatePreferences(context.Background(), application.PreferencePatch{
		ColorScheme: &scheme, Theme: &themeMode,
	}); err != nil {
		t.Fatal(err)
	}
	first.app.Close()

	second := newHarnessWithStorage(t, backend)
	second.view.SetPreferences(systemAccentPreferences())
	second.login()
	if got := second.app.Core.Preferences(); got.ColorScheme != scheme || got.Theme != themeMode {
		t.Fatalf("restored preferences = %+v", got)
	}
	second.goTo(Route{Page: "search"})
	second.view.Frame()
	palette := appearance.Resolve(appearance.Plum, true)
	img := second.view.Image()
	if got := countRGB(img, ui.Hex(appearance.Hex(palette.Accent))); got == 0 {
		t.Fatalf("restart did not render the plum accent (want %s)", appearance.Hex(palette.Accent))
	}
	if got := countRGB(img, systemAccentPreferences().Accent); got != 0 {
		t.Fatalf("restart rendered the system accent: %d pixels", got)
	}
}
