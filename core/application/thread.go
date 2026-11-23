package application

import (
	"context"
	"errors"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

// ErrStaleSession reports a result or history write bound to a session that has
// already been replaced. Callers drop it instead of applying it to the new one.
var ErrStaleSession = errors.New("账号会话已切换")

// SessionInfo is the non-sensitive session view UI code may keep. Generation
// changes on login, logout, restore, account switch and account removal so an
// in-flight result can be matched to the account that started it.
type SessionInfo struct {
	Username      string
	Authenticated bool
	Generation    uint64
}

// SessionInfo returns a snapshot of the current session without its token.
func (a *App) SessionInfo() SessionInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return SessionInfo{
		Username:      a.session.Username,
		Authenticated: a.session.Token != "",
		Generation:    a.generation,
	}
}

// bumpGenerationLocked must be called with a.mu held whenever the active
// session identity changes. Results carry the generation observed before their
// first request; a later comparison rejects results from the old session.
func (a *App) bumpGenerationLocked() { a.generation++ }

// ThreadDetail is the shared result of opening a thread. CanonicalBoard is the
// board identity both UIs should use for links, reactions and history.
type ThreadDetail struct {
	Thread         forum.Thread
	CanonicalBoard string
}

// boundCredentialKey carries the credential snapshot of one use case through
// the forum transport without changing the shared client.
type boundCredentialKey struct{}

func withCredential(ctx context.Context, credential Credential) context.Context {
	return context.WithValue(ctx, boundCredentialKey{}, credential)
}

// ReadThread fetches one page of a thread and merges the current account's
// collection state. The credential and session generation are captured before
// the first request, and the cache is checked and updated under one lock after
// the response so a logout/login cannot populate the new session's cache.
func (a *App) ReadThread(ctx context.Context, board, id string, page forum.Page) (ThreadDetail, error) {
	a.mu.RLock()
	credential := a.session
	generation := a.generation
	a.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return ThreadDetail{}, err
	}
	ctx = withCredential(ctx, credential)
	v, err := a.Forum.Thread(ctx, board, id, page)
	if err != nil {
		return ThreadDetail{}, err
	}
	if err := ctx.Err(); err != nil {
		return ThreadDetail{}, err
	}
	canonical := canonicalThreadBoard(v, board)
	if strings.TrimSpace(v.Board) == "" {
		v.Board = canonical
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return ThreadDetail{}, err
	}
	if a.generation != generation || a.session.Token != credential.Token {
		return ThreadDetail{}, ErrStaleSession
	}
	key := collectionIdentity(credential.Username, canonical, id)
	fallback := bool(v.Collect)
	if fallback {
		// A thread read from a collection page is collected even when the
		// response omits the flag. Remember it for the account that asked.
		if a.collections == nil {
			a.collections = make(map[collectionKey]bool)
		}
		a.collections[key] = true
	}
	collected := fallback
	if saved, ok := a.collections[key]; ok {
		collected = saved
	}
	v.Collect = forum.Flag(collected)
	return ThreadDetail{Thread: v, CanonicalBoard: canonical}, nil
}

// RecordThreadVisit appends a visit for the session generation that loaded the
// thread. Stale generations are rejected so switching accounts cannot write one
// account's reading history into another's state.
func (a *App) RecordThreadVisit(ctx context.Context, generation uint64, board, id, title string) error {
	info := a.SessionInfo()
	if generation == 0 || info.Generation != generation || !info.Authenticated {
		return ErrStaleSession
	}
	return a.Local.Visit(ctx, info.Username, local.History{Board: board, ID: id, Title: title})
}

// canonicalThreadBoard follows the response thread board, then the first floor
// that carries a board, then the requested board. Keeping this rule here avoids
// desktop and Android disagreeing about reaction and collection endpoints.
func canonicalThreadBoard(v forum.Thread, requested string) string {
	if board := strings.TrimSpace(v.Board); board != "" {
		return board
	}
	for _, article := range v.Articles {
		if board := strings.TrimSpace(article.Board); board != "" {
			return board
		}
	}
	return strings.TrimSpace(requested)
}

// collectionState reads a successful state remembered for a specific account.
// Reading through the captured account keeps a request started before an
// account switch from reading the new account's map.
func (a *App) collectionState(account, board, id string, fallback bool) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if value, known := a.collections[collectionIdentity(account, board, id)]; known {
		return value
	}
	return fallback
}
