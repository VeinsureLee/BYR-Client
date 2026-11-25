package application

import (
	"context"

	"github.com/VeinsureLee/BYR-Client/core/local"
)

type Preferences = local.Preferences
type PreferencePatch = local.PreferencePatch

// Preferences returns a normalized local snapshot for the current device.
func (a *App) Preferences() Preferences { return a.local.Preferences() }

// UpdatePreferences persists only fields named in the patch so independent
// settings controls cannot overwrite one another from stale snapshots.
func (a *App) UpdatePreferences(ctx context.Context, patch PreferencePatch) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.local.UpdatePreferences(ctx, patch)
}
