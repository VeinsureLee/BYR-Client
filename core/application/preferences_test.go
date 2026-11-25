package application

import (
	"context"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/appearance"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestPreferenceUseCasesApplyOnlyExplicitFields(t *testing.T) {
	a := appFor(t, nil, nil, &credentials{value: Credential{Username: "alice", Token: "token"}})
	initial := local.Preferences{Theme: 2, ColorScheme: string(appearance.BYRBlue), FontScale: 1.2, IPv6: true}
	if err := a.UpdatePreferences(context.Background(), local.PreferencePatch{
		Theme: &initial.Theme, ColorScheme: &initial.ColorScheme, FontScale: &initial.FontScale, IPv6: &initial.IPv6,
	}); err != nil {
		t.Fatal(err)
	}
	pine := string(appearance.Pine)
	if err := a.UpdatePreferences(context.Background(), local.PreferencePatch{ColorScheme: &pine}); err != nil {
		t.Fatal(err)
	}
	got := a.Preferences()
	if got.Theme != 2 || got.ColorScheme != pine || got.FontScale != 1.2 || !got.IPv6 {
		t.Fatalf("preference snapshot = %+v", got)
	}
}
