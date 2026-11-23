// Package application coordinates shared business use cases. It has no
// framework types, OS calls, UI routing or background goroutine ownership.
package application

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

type Credential struct {
	Username, Token string
	ExpiresAt       int64
}

// CredentialStore persists only the session credential. Implementations belong
// to the platform boundary so the application layer remains testable.
type CredentialStore interface {
	Load(context.Context) (Credential, error)
	Save(context.Context, Credential) error
	Clear(context.Context) error
}

// Options supplies the application facade's concrete protocol and local-state
// collaborators. Local is required; Credentials may be nil for session-only use.
type Options struct {
	BaseURL     string
	HTTPClient  *http.Client
	Local       *local.Store
	Credentials CredentialStore
}

// App coordinates forum use cases while keeping storage and platform services
// behind consumer-owned interfaces.
type App struct {
	Forum       *forum.Client
	Local       *local.Store
	credentials CredentialStore
	mu          sync.RWMutex
	session     Credential
	generation  uint64
	collections map[collectionKey]bool
}

// New constructs an application facade without performing I/O.
func New(o Options) (*App, error) {
	if o.Local == nil {
		return nil, errors.New("local store is required")
	}
	a := &App{Local: o.Local, credentials: o.Credentials}
	f, err := forum.New(forum.Options{BaseURL: o.BaseURL, HTTPClient: o.HTTPClient, Token: a.Token, IPv6: func() bool { return a.Local.Preferences().IPv6 }})
	if err != nil {
		return nil, err
	}
	a.Forum = f
	return a, nil
}

// Session returns a copy of the current in-memory credential.
func (a *App) Session() Credential {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.session
}
func (a *App) Token(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	fresh := func(c Credential) (string, error) {
		if strings.TrimSpace(c.Token) == "" || c.ExpiresAt > 0 && time.Now().Unix() >= c.ExpiresAt {
			return "", forum.ErrUnauthenticated
		}
		return c.Token, nil
	}
	// A use case can bind the credential it started with so a later session
	// change cannot silently send the new account's token on the old request.
	// The bound credential obeys the same empty/expiry checks as the current
	// session; binding is not a way to bypass token validity.
	if bound, ok := ctx.Value(boundCredentialKey{}).(Credential); ok {
		return fresh(bound)
	}
	return fresh(a.Session())
}
func (a *App) Restore(ctx context.Context) error {
	if a.credentials == nil {
		return nil
	}
	s, err := a.credentials.Load(ctx)
	if err != nil {
		return err
	}
	if s.Token == "" {
		a.mu.Lock()
		a.session = Credential{}
		a.collections = nil
		a.bumpGenerationLocked()
		a.mu.Unlock()
		return nil
	}
	if s.ExpiresAt > 0 && s.ExpiresAt <= time.Now().Unix() {
		var clearErr error
		if accounts, ok := a.credentials.(AccountCredentialStore); ok {
			clearErr = accounts.DeleteAccount(ctx, s.Username)
		} else {
			clearErr = a.credentials.Clear(ctx)
		}
		a.mu.Lock()
		a.session = Credential{}
		a.collections = nil
		a.bumpGenerationLocked()
		a.mu.Unlock()
		return clearErr
	}
	a.mu.Lock()
	a.session = s
	a.bumpGenerationLocked()
	a.mu.Unlock()
	return nil
}
func (a *App) Login(ctx context.Context, username, password string, remember bool) error {
	info, err := a.Forum.Login(ctx, username, password)
	if err != nil {
		return err
	}
	s := Credential{Username: strings.TrimSpace(username), Token: info.AccessToken}
	if info.ExpiresIn > 0 {
		s.ExpiresAt = time.Now().Unix() + int64(info.ExpiresIn)
	}
	if a.credentials != nil {
		if remember {
			err = a.credentials.Save(ctx, s)
		} else if accounts, ok := a.credentials.(AccountCredentialStore); ok {
			// A non-remembered login clears only the persisted active selection;
			// other saved accounts remain available in the account manager.
			err = accounts.SetActive(ctx, "")
		} else {
			err = a.credentials.Clear(ctx)
			if errors.Is(err, errors.ErrUnsupported) {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	} else if remember {
		return errors.New("未配置系统凭据存储，请关闭记住登录")
	}
	a.mu.Lock()
	a.session = s
	a.bumpGenerationLocked()
	a.mu.Unlock()
	return nil
}
func (a *App) Logout(ctx context.Context) error {
	if a.credentials != nil {
		var err error
		if accounts, ok := a.credentials.(AccountCredentialStore); ok {
			err = accounts.SetActive(ctx, "")
		} else {
			err = a.credentials.Clear(ctx)
		}
		if err != nil && !errors.Is(err, errors.ErrUnsupported) {
			return err
		}
	}
	a.mu.Lock()
	a.session = Credential{}
	a.collections = nil
	a.bumpGenerationLocked()
	a.mu.Unlock()
	return nil
}

// Publication reports the server article and a separate local cleanup error.
// Callers must not retry a published post because disk cleanup failed.
type Publication struct {
	Article    forum.Article
	LocalError error
}

// Publish validates permissions, performs the server mutation, and removes
// local composer drafts only after the server confirms success.
func (a *App) Publish(ctx context.Context, p forum.Post, editID string) (Publication, error) {
	if strings.TrimSpace(p.Board) == "" || strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Content) == "" {
		return Publication{}, errors.New("版面、标题和内容不能为空")
	}
	account := a.Session().Username
	board, err := a.Forum.Board(ctx, p.Board, forum.Page{Size: 1})
	if err != nil {
		return Publication{}, err
	}
	if err = ValidatePost(board, p, editID); err != nil {
		return Publication{}, err
	}
	var article forum.Article
	if editID != "" {
		article, err = a.Forum.UpdateArticle(ctx, p.Board, editID, p.Title, p.Content)
	} else {
		article, err = a.Forum.PostArticle(ctx, p)
	}
	if err != nil {
		return Publication{}, err
	}
	cleanup := a.clearPublicationDrafts(ctx, account, p, editID)
	return Publication{Article: article, LocalError: cleanup}, nil
}

// clearPublicationDrafts removes both the selected-board draft and the
// boardless new-post draft used by the mobile composer. Each deletion is
// attempted so one failed cleanup does not leave another copy behind.
func (a *App) clearPublicationDrafts(ctx context.Context, account string, p forum.Post, editID string) error {
	keys := []string{local.DraftKey(p.Board, p.ReplyID, editID)}
	if p.ReplyID == "" && editID == "" && p.Board != "" {
		keys = append(keys, local.DraftKey("", "", ""))
	}
	var failures []error
	for _, key := range keys {
		if err := a.Local.SaveDraft(ctx, account, key, local.Draft{}); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// ValidatePost is shared by presentation previews and the publishing use case.
// The server remains authoritative if permissions change after this check.
func ValidatePost(board forum.Board, p forum.Post, editID string) error {
	if bool(board.IsReadOnly) || board.AllowPost != nil && !bool(*board.AllowPost) {
		return errors.New("当前版面不允许发帖")
	}
	if editID == "" && p.ReplyID != "" && bool(board.IsNoReply) {
		return errors.New("当前版面不允许回复")
	}
	if editID == "" && p.Anonymous && !bool(board.AllowAnonymous) {
		return errors.New("此版面不支持匿名发帖")
	}
	if editID == "" && p.Outgo && !bool(board.AllowOutgo) {
		return errors.New("此版面不允许转出")
	}
	return nil
}
