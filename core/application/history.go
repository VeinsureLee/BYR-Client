package application

import (
	"context"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/local"
)

// HistoryReadToken binds a deferred visit write to the account session and
// history-clear epoch observed when its route read began.
type HistoryReadToken struct {
	Username   string
	Generation uint64
	Epoch      uint64
}

func historyAccountKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// HistoryReadToken captures the currently authenticated account and its
// per-account clear epoch before a route starts asynchronous work.
func (a *App) HistoryReadToken() HistoryReadToken {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	a.mu.RLock()
	session, generation := a.session, a.generation
	a.mu.RUnlock()
	account := historyAccountKey(session.Username)
	return HistoryReadToken{Username: session.Username, Generation: generation, Epoch: a.historyEpoch[account]}
}

// ClearHistory clears only the active account and only if the confirmation was
// created for the session generation that is still active. The per-account epoch
// invalidates route visits queued before the clear, so they cannot repopulate it.
func (a *App) ClearHistory(ctx context.Context, generation uint64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	a.mu.RLock()
	defer a.mu.RUnlock()
	if generation == 0 || a.generation != generation || a.session.Token == "" || historyAccountKey(a.session.Username) == "" {
		return ErrStaleSession
	}
	if err := a.local.ClearHistory(ctx, a.session.Username); err != nil {
		return err
	}
	account := historyAccountKey(a.session.Username)
	if a.historyEpoch == nil {
		a.historyEpoch = make(map[string]uint64)
	}
	a.historyEpoch[account]++
	return nil
}

// RecentHistory reads only the active account's local reading records. It
// returns an immutable slice snapshot suitable for synchronous page rendering.
func (a *App) RecentHistory() []local.History {
	info := a.SessionInfo()
	if !info.Authenticated {
		return nil
	}
	return a.local.History(info.Username)
}

// RecordThreadVisit writes a successful route visit only if the request still
// belongs to the same session and predates no successful history clear. A stale
// pre-clear visit is intentionally dropped without turning the visible thread
// into an error state.
func (a *App) RecordThreadVisit(ctx context.Context, token HistoryReadToken, board, id, title string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	a.mu.RLock()
	defer a.mu.RUnlock()
	account := historyAccountKey(a.session.Username)
	if token.Generation == 0 || token.Generation != a.generation || a.session.Token == "" ||
		account == "" || account != historyAccountKey(token.Username) {
		return ErrStaleSession
	}
	if token.Epoch != a.historyEpoch[account] {
		return nil
	}
	return a.local.Visit(ctx, a.session.Username, local.History{Board: board, ID: id, Title: title})
}
