package application

import (
	"context"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

// bindSession pins protocol requests to the credential observed at use-case
// start. A later login/account switch cannot silently swap identity mid-request.
func (a *App) bindSession(ctx context.Context) (context.Context, Credential, uint64, error) {
	if err := ctx.Err(); err != nil {
		return ctx, Credential{}, 0, err
	}
	a.mu.RLock()
	credential, generation := a.session, a.generation
	a.mu.RUnlock()
	if credential.Token == "" {
		return ctx, credential, generation, forum.ErrUnauthenticated
	}
	return withCredential(ctx, credential), credential, generation, nil
}

// ensureSessionCurrent rejects a response or write whose starting account
// session has been replaced while the use case was in flight.
func (a *App) ensureSessionCurrent(ctx context.Context, credential Credential, generation uint64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if generation == 0 || a.generation != generation || a.session.Token != credential.Token || a.session.Username != credential.Username {
		return ErrStaleSession
	}
	return nil
}

func runBound[T any](a *App, ctx context.Context, work func(context.Context, Credential) (T, error)) (T, error) {
	requestCtx, credential, generation, err := a.bindSession(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	result, err := work(requestCtx, credential)
	if sessionErr := a.ensureSessionCurrent(ctx, credential, generation); sessionErr != nil {
		var zero T
		return zero, sessionErr
	}
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}
