package desktop

import (
	"context"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/appearance"
	"github.com/VeinsureLee/BYR-Client/core/application"
)

func TestDesktopPaletteSettingsPersistSchemeAndKeepOtherPreferences(t *testing.T) {
	h := newHarness(t)
	h.login()
	theme := 2
	scheme := string(appearance.BYRBlue)
	fontScale := float32(1.2)
	ipv6 := true
	if err := h.app.Core.UpdatePreferences(context.Background(), application.PreferencePatch{
		Theme: &theme, ColorScheme: &scheme, FontScale: &fontScale, IPv6: &ipv6,
	}); err != nil {
		t.Fatal(err)
	}
	h.goTo(Route{Page: "settings"})
	h.has("北邮蓝")
	h.has("松针绿")
	h.has("暮紫")
	h.click("选择配色 松针绿")
	got := h.app.Core.Preferences()
	if got.ColorScheme != string(appearance.Pine) || got.Theme != 2 || got.FontScale != 1.2 || !got.IPv6 {
		t.Fatalf("selecting pine changed unrelated preferences: %+v", got)
	}
	h.click("选择配色 暮紫")
	got = h.app.Core.Preferences()
	if got.ColorScheme != string(appearance.Plum) || got.Theme != 2 || got.FontScale != 1.2 || !got.IPv6 {
		t.Fatalf("selecting plum changed unrelated preferences: %+v", got)
	}
	if h.app.route.Page != "settings" {
		t.Fatalf("palette change navigated away from settings: %+v", h.app.route)
	}
}
