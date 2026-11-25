package desktop

import (
	"math"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/appearance"
	"github.com/egoist/mygo/ui"
)

func TestDesktopThemeMapsSharedAppearancePalettes(t *testing.T) {
	for _, scheme := range appearance.Schemes() {
		for _, dark := range []bool{false, true} {
			palette := appearance.Resolve(scheme, dark)
			theme := forumTheme(scheme, dark, 1.2)
			if theme.Accent != ui.Hex(appearance.Hex(palette.Accent)) ||
				theme.AccentHover != ui.Hex(appearance.Hex(palette.AccentHover)) ||
				theme.AccentPressed != ui.Hex(appearance.Hex(palette.AccentPressed)) ||
				theme.AccentText != ui.Hex(appearance.Hex(palette.AccentText)) ||
				theme.Selection != ui.Hex(appearance.Hex(palette.Selection)) {
				t.Errorf("%s dark=%v desktop theme did not map appearance tokens", scheme, dark)
			}
			if theme.Background != ui.Hex(appearance.Hex(palette.Background)) ||
				theme.Surface != ui.Hex(appearance.Hex(palette.Surface)) ||
				theme.Text != ui.Hex(appearance.Hex(palette.Text)) {
				t.Errorf("%s dark=%v desktop theme did not map neutral tokens", scheme, dark)
			}
			if theme.Focus != theme.Accent || math.Abs(float64(theme.FontSize)-16.8) > 0.0001 {
				t.Errorf("%s dark=%v focus/font = %v/%v", scheme, dark, theme.Focus, theme.FontSize)
			}
		}
	}
}
