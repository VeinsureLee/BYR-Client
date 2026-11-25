package application

import (
	"context"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func (a *App) verifySessionInfo(ctx context.Context, session SessionInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if session.Generation == 0 || !session.Authenticated {
		return forum.ErrUnauthenticated
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.generation != session.Generation || a.session.Token == "" ||
		historyAccountKey(a.session.Username) != historyAccountKey(session.Username) {
		return ErrStaleSession
	}
	return nil
}

func (a *App) LoadDraft(ctx context.Context, session SessionInfo, key string) (local.Draft, bool, error) {
	if err := a.verifySessionInfo(ctx, session); err != nil {
		return local.Draft{}, false, err
	}
	draft, found := a.local.Draft(session.Username, key)
	if err := a.verifySessionInfo(ctx, session); err != nil {
		return local.Draft{}, false, err
	}
	return draft, found, nil
}

func (a *App) SaveDraft(ctx context.Context, session SessionInfo, key string, draft local.Draft) error {
	if err := a.verifySessionInfo(ctx, session); err != nil {
		return err
	}
	if err := a.local.SaveDraft(ctx, session.Username, key, draft); err != nil {
		return err
	}
	return a.verifySessionInfo(ctx, session)
}

func (a *App) DeleteDraft(ctx context.Context, session SessionInfo, key string) error {
	return a.SaveDraft(ctx, session, key, local.Draft{})
}
