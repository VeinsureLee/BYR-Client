package application

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

var (
	ErrAccountStoreUnsupported = errors.New("当前凭据存储不支持多账号")
	ErrSavedAccountNotFound    = errors.New("已保存的账号不存在")
)

// AccountCredentialStore extends the legacy single-session store without
// coupling the shared core to any platform keychain or file format.
type AccountCredentialStore interface {
	Accounts(context.Context) ([]Credential, error)
	LoadAccount(context.Context, string) (Credential, error)
	SetActive(context.Context, string) error
	DeleteAccount(context.Context, string) error
}

// SavedAccount exposes only non-secret account metadata to the UI.
type SavedAccount struct {
	Username   string
	Active     bool
	Remembered bool
}

func accountKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// SavedAccounts returns the known accounts without exposing access tokens. A
// non-remembered active session is represented with Remembered=false.
func (a *App) SavedAccounts(ctx context.Context) ([]SavedAccount, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	session := a.Session()
	store, supportsAccounts := a.credentials.(AccountCredentialStore)
	var saved []Credential
	if supportsAccounts {
		var err error
		saved, err = store.Accounts(ctx)
		if err != nil {
			return nil, err
		}
	} else if a.credentials != nil {
		credential, err := a.credentials.Load(ctx)
		if err != nil {
			return nil, err
		}
		if credential.Username != "" && credential.Token != "" {
			saved = []Credential{credential}
		}
	}

	accounts := make([]SavedAccount, 0, len(saved)+1)
	known := make(map[string]bool, len(saved)+1)
	for _, credential := range saved {
		key := accountKey(credential.Username)
		if key == "" || credential.Token == "" || known[key] {
			continue
		}
		known[key] = true
		accounts = append(accounts, SavedAccount{
			Username:   credential.Username,
			Active:     accountKey(session.Username) == key,
			Remembered: true,
		})
	}
	if key := accountKey(session.Username); key != "" && !known[key] {
		accounts = append(accounts, SavedAccount{Username: session.Username, Active: true})
	}
	sort.SliceStable(accounts, func(i, j int) bool {
		if accounts[i].Active != accounts[j].Active {
			return accounts[i].Active
		}
		return strings.ToLower(accounts[i].Username) < strings.ToLower(accounts[j].Username)
	})
	return accounts, nil
}

// SwitchAccount activates a saved credential only after it is found, still
// valid, and the platform store successfully persists the selected identity.
func (a *App) SwitchAccount(ctx context.Context, username string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store, ok := a.credentials.(AccountCredentialStore)
	if !ok {
		return ErrAccountStoreUnsupported
	}
	username = strings.TrimSpace(username)
	if accountKey(username) == "" {
		return ErrSavedAccountNotFound
	}
	credential, err := store.LoadAccount(ctx, username)
	if err != nil {
		return err
	}
	if credential.Token == "" || credential.ExpiresAt > 0 && credential.ExpiresAt <= time.Now().Unix() {
		return forum.ErrUnauthenticated
	}
	if err := store.SetActive(ctx, credential.Username); err != nil {
		return err
	}
	a.mu.Lock()
	if accountKey(a.session.Username) != accountKey(credential.Username) || a.session.Token != credential.Token {
		a.collections = nil
		a.bumpGenerationLocked()
	}
	a.session = credential
	a.mu.Unlock()
	return nil
}

// ForgetAccount removes only the selected saved account. Forgetting the active
// one also clears the in-memory session after persistent deletion succeeds.
func (a *App) ForgetAccount(ctx context.Context, username string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store, ok := a.credentials.(AccountCredentialStore)
	if !ok {
		return ErrAccountStoreUnsupported
	}
	username = strings.TrimSpace(username)
	if accountKey(username) == "" {
		return ErrSavedAccountNotFound
	}
	if err := store.DeleteAccount(ctx, username); err != nil {
		return err
	}
	a.mu.Lock()
	if accountKey(a.session.Username) == accountKey(username) {
		a.session = Credential{}
		a.collections = nil
		a.bumpGenerationLocked()
	}
	a.mu.Unlock()
	return nil
}
