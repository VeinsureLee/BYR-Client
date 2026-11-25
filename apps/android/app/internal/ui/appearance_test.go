package ui

import (
	"context"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/appearance"
	"github.com/VeinsureLee/BYR-Client/core/application"
)

func setThemePreferenceForTest(t *testing.T, a *App, mode int) {
	t.Helper()
	if a.core == nil {
		t.Fatal("theme test app has no shared core")
	}
	if err := a.core.UpdatePreferences(context.Background(), application.PreferencePatch{Theme: &mode}); err != nil {
		t.Fatal(err)
	}
}

func TestAndroidPaletteChoiceUpdatesLiveThemeWithoutResettingPageState(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	theme := 2
	scheme := string(appearance.BYRBlue)
	fontScale := float32(1.2)
	ipv6 := true
	if err := a.core.UpdatePreferences(context.Background(), application.PreferencePatch{
		Theme: &theme, ColorScheme: &scheme, FontScale: &fontScale, IPv6: &ipv6,
	}); err != nil {
		t.Fatal(err)
	}
	a.stack = []Route{{Page: PageHome}}
	a.openSettings()
	renderAppAt(t, a, 420, 700, 1)
	for _, scheme := range appearance.Schemes() {
		if a.clicks["settings-scheme:"+string(scheme)] == nil {
			t.Fatalf("settings omitted the %s palette option", scheme)
		}
	}
	originalStack := append([]Route(nil), a.stack...)

	a.click("settings-scheme:" + string(appearance.Pine)).Click()
	renderAppAt(t, a, 420, 700, 1)
	renderAppAt(t, a, 420, 700, 1)
	got := a.core.Preferences()
	if got.ColorScheme != string(appearance.Pine) || got.Theme != 2 || got.FontScale != 1.2 || !got.IPv6 {
		t.Fatalf("pine selection changed unrelated preferences: %+v", got)
	}
	want := appearance.Resolve(appearance.Pine, true)
	if !a.dark || a.colorScheme != string(appearance.Pine) || colorAccent != want.Accent || colorAccentSoft != want.Selection || a.theme.ContrastFg != want.AccentText {
		t.Fatalf("live pine palette mismatch: dark=%v scheme=%q accent=%v selection=%v contrast=%v want=%+v", a.dark, a.colorScheme, colorAccent, colorAccentSoft, a.theme.ContrastFg, want)
	}
	if len(a.stack) != len(originalStack) || a.stack[0] != originalStack[0] {
		t.Fatalf("palette change navigated away from settings: %+v", a.stack)
	}

	a.click("settings-scheme:" + string(appearance.Plum)).Click()
	renderAppAt(t, a, 420, 700, 1)
	renderAppAt(t, a, 420, 700, 1)
	if got := a.core.Preferences().ColorScheme; got != string(appearance.Plum) {
		t.Fatalf("selected scheme after switching again = %q", got)
	}
	if want := appearance.Resolve(appearance.Plum, true); colorAccent != want.Accent || colorAccentSoft != want.Selection {
		t.Fatalf("plum palette = accent:%v selection:%v, want %+v", colorAccent, colorAccentSoft, want)
	}
}
