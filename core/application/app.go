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
type CredentialStore interface {
	Load(context.Context) (Credential, error)
	Save(context.Context, Credential) error
	Clear(context.Context) error
}
type Options struct {
	BaseURL     string
	HTTPClient  *http.Client
	Local       *local.Store
	Credentials CredentialStore
}
type App struct {
	Forum       *forum.Client
	Local       *local.Store
	credentials CredentialStore
	mu          sync.RWMutex
	session     Credential
	collections map[collectionKey]bool
}

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
func (a *App) Session() Credential { a.mu.RLock(); defer a.mu.RUnlock(); return a.session }
func (a *App) Token(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s := a.Session()
	if s.Token == "" || s.ExpiresAt > 0 && time.Now().Unix() >= s.ExpiresAt {
		return "", forum.ErrUnauthenticated
	}
	return s.Token, nil
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
		return nil
	}
	if s.ExpiresAt > 0 && s.ExpiresAt <= time.Now().Unix() {
		return a.credentials.Clear(ctx)
	}
	a.mu.Lock()
	a.session = s
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
	a.mu.Unlock()
	return nil
}
func (a *App) Logout(ctx context.Context) error {
	if a.credentials != nil {
		if err := a.credentials.Clear(ctx); err != nil && !errors.Is(err, errors.ErrUnsupported) {
			return err
		}
	}
	a.mu.Lock()
	a.session = Credential{}
	a.mu.Unlock()
	return nil
}

// Publish removes the draft only after server success. A local cleanup error is
// separate: callers must not retry a published post because disk cleanup failed.
type Publication struct {
	Article    forum.Article
	LocalError error
}

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
	cleanup := a.Local.SaveDraft(ctx, account, local.DraftKey(p.Board, p.ReplyID, editID), local.Draft{})
	return Publication{Article: article, LocalError: cleanup}, nil
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
