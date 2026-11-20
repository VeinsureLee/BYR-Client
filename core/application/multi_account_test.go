package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

type accountCredentials struct {
	accounts map[string]Credential
	active   string
	fail     error
}

func (s *accountCredentials) Load(context.Context) (Credential, error) {
	return s.accounts[accountKey(s.active)], nil
}

func (s *accountCredentials) Save(_ context.Context, credential Credential) error {
	if s.fail != nil {
		return s.fail
	}
	if s.accounts == nil {
		s.accounts = make(map[string]Credential)
	}
	key := accountKey(credential.Username)
	s.accounts[key] = credential
	s.active = key
	return nil
}

func (s *accountCredentials) Clear(context.Context) error {
	if s.fail != nil {
		return s.fail
	}
	s.active = ""
	return nil
}

func (s *accountCredentials) Accounts(context.Context) ([]Credential, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	keys := make([]string, 0, len(s.accounts))
	for key := range s.accounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Credential, 0, len(keys))
	for _, key := range keys {
		out = append(out, s.accounts[key])
	}
	return out, nil
}

func (s *accountCredentials) LoadAccount(_ context.Context, username string) (Credential, error) {
	if s.fail != nil {
		return Credential{}, s.fail
	}
	credential, ok := s.accounts[accountKey(username)]
	if !ok {
		return Credential{}, ErrSavedAccountNotFound
	}
	return credential, nil
}

func (s *accountCredentials) SetActive(_ context.Context, username string) error {
	if s.fail != nil {
		return s.fail
	}
	key := accountKey(username)
	if key != "" {
		if _, ok := s.accounts[key]; !ok {
			return ErrSavedAccountNotFound
		}
	}
	s.active = key
	return nil
}

func (s *accountCredentials) DeleteAccount(_ context.Context, username string) error {
	if s.fail != nil {
		return s.fail
	}
	key := accountKey(username)
	if _, ok := s.accounts[key]; !ok {
		return ErrSavedAccountNotFound
	}
	delete(s.accounts, key)
	if s.active == key {
		s.active = ""
	}
	return nil
}

func TestMultiAccountSwitchPreservesCredentialsAndMetadata(t *testing.T) {
	ctx := context.Background()
	store := &accountCredentials{accounts: map[string]Credential{
		"alice": {Username: "Alice", Token: "alice-token"},
		"bob":   {Username: "bob", Token: "bob-token"},
	}, active: "alice"}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{}`) }, nil, store)

	if err := a.SwitchAccount(ctx, "BOB"); err != nil {
		t.Fatal(err)
	}
	if got := a.Session(); got.Username != "bob" || got.Token != "bob-token" {
		t.Fatalf("switched session = %+v", got)
	}
	if err := a.SwitchAccount(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if got := a.Session(); got.Username != "Alice" || got.Token != "alice-token" {
		t.Fatalf("switched back session = %+v", got)
	}

	accounts, err := a.SavedAccounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || !accounts[0].Active || accounts[0].Username != "Alice" {
		t.Fatalf("saved account summaries = %+v", accounts)
	}
	if strings.Contains(strings.ToLower(fmt.Sprintf("%+v", accounts)), "token") {
		t.Fatalf("account summaries exposed credentials: %+v", accounts)
	}
}

func TestNonRememberedLoginDoesNotForgetOtherAccounts(t *testing.T) {
	ctx := context.Background()
	store := &accountCredentials{accounts: map[string]Credential{
		"alice": {Username: "alice", Token: "alice-token"},
	}, active: "alice"}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"access_token":"bob-token","expires_in":3600}`)
	}, nil, store)
	if err := a.Login(ctx, "bob", "pw", false); err != nil {
		t.Fatal(err)
	}
	if len(store.accounts) != 1 || store.accounts["alice"].Token != "alice-token" || store.active != "" {
		t.Fatalf("non-remembered login changed saved accounts: accounts=%+v active=%q", store.accounts, store.active)
	}
	if got := a.Session(); got.Username != "bob" || got.Token != "bob-token" {
		t.Fatalf("active session = %+v", got)
	}
}

func TestSwitchAccountFailureLeavesCurrentSessionUnchanged(t *testing.T) {
	ctx := context.Background()
	store := &accountCredentials{accounts: map[string]Credential{
		"alice": {Username: "alice", Token: "alice-token"},
		"bob":   {Username: "bob", Token: "bob-token"},
	}, active: "alice"}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{}`) }, nil, store)
	store.fail = errors.New("keyring unavailable")
	if err := a.SwitchAccount(ctx, "bob"); err == nil {
		t.Fatal("switch succeeded despite keyring error")
	}
	if got := a.Session(); got.Username != "alice" || got.Token != "alice-token" {
		t.Fatalf("failed switch changed session: %+v", got)
	}
}

func TestExpiredAccountCannotBeActivatedAndForgetClearsActiveOnly(t *testing.T) {
	ctx := context.Background()
	store := &accountCredentials{accounts: map[string]Credential{
		"alice": {Username: "alice", Token: "alice-token"},
		"bob":   {Username: "bob", Token: "expired", ExpiresAt: time.Now().Unix() - 1},
	}, active: "alice"}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{}`) }, nil, store)
	if err := a.SwitchAccount(ctx, "bob"); !errors.Is(err, forum.ErrUnauthenticated) {
		t.Fatalf("expired account switch error = %v, want unauthenticated", err)
	}
	if got := a.Session().Username; got != "alice" {
		t.Fatalf("expired account replaced active session with %q", got)
	}
	if err := a.ForgetAccount(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if a.Session().Token != "" || len(store.accounts) != 1 || store.accounts["bob"].Token != "expired" {
		t.Fatalf("forget active result session=%+v accounts=%+v", a.Session(), store.accounts)
	}
}

func TestLogoutClearsActiveSelectionButRetainsSavedAccounts(t *testing.T) {
	ctx := context.Background()
	store := &accountCredentials{accounts: map[string]Credential{
		"alice": {Username: "alice", Token: "alice-token"},
		"bob":   {Username: "bob", Token: "bob-token"},
	}, active: "alice"}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{}`) }, nil, store)
	if err := a.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if a.Session().Token != "" || store.active != "" || len(store.accounts) != 2 {
		t.Fatalf("logout did not preserve saved accounts: session=%+v store=%+v", a.Session(), store)
	}
}
