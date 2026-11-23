package application

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestReadThreadCanonicalBoardAndCollectionMerge(t *testing.T) {
	creds := &credentials{value: Credential{Username: "alice", Token: "token"}}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/threads/悄悄话/42.json":
			io.WriteString(w, `{"id":42,"title":"匿名主题","collect":0,"article":[
				{"id":42,"position":1,"board_name":"IWhisper","content":"正文","user":{"id":"IWhisper#1"}},
				{"id":44,"position":2,"board_name":"IWhisper","content":"回复","user":{"id":"bob"}}]}`)
		case "/open/threads/Go/43.json":
			io.WriteString(w, `{"id":43,"title":"无版面响应的主题","article":[{"id":43,"position":1,"content":"正文"}]}`)
		default:
			io.WriteString(w, `{"code":"0000"}`)
		}
	}, nil, creds)
	a.RememberCollection("IWhisper", "42")

	detail, err := a.ReadThread(context.Background(), "悄悄话", "42", forum.Page{Number: 1})
	if err != nil {
		t.Fatal(err)
	}
	if detail.CanonicalBoard != "IWhisper" {
		t.Fatalf("canonical board = %q, want IWhisper", detail.CanonicalBoard)
	}
	if !bool(detail.Thread.Collect) {
		t.Fatal("collection state was not merged into the shared thread result")
	}
	if detail.Thread.Board != "IWhisper" {
		t.Fatalf("thread board = %q, want the canonical board", detail.Thread.Board)
	}

	// The requested board is the last fallback when the response and floors
	// carry no board identity.
	fallback, err := a.ReadThread(context.Background(), "Go", "43", forum.Page{Number: 1})
	if err != nil {
		t.Fatal(err)
	}
	if fallback.CanonicalBoard != "Go" {
		t.Fatalf("requested board fallback = %q, want Go", fallback.CanonicalBoard)
	}
}

func TestRecordThreadVisitRejectsStaleGeneration(t *testing.T) {
	ctx := context.Background()
	creds := &credentials{value: Credential{Username: "alice", Token: "token"}}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) }, nil, creds)
	info := a.SessionInfo()
	if !info.Authenticated || info.Generation == 0 {
		t.Fatalf("session info = %+v", info)
	}
	if err := a.RecordThreadVisit(ctx, info.Generation-1, "Go", "42", "旧主题"); !errors.Is(err, ErrStaleSession) {
		t.Fatalf("stale history error = %v, want ErrStaleSession", err)
	}
	if got := a.Local.History("alice"); len(got) != 0 {
		t.Fatalf("stale history was written: %+v", got)
	}
	if err := a.RecordThreadVisit(ctx, info.Generation, "Go", "42", "主题"); err != nil {
		t.Fatal(err)
	}
	if got := a.Local.History("alice"); len(got) != 1 || got[0].ID != "42" || got[0].Title != "主题" {
		t.Fatalf("history = %+v", got)
	}
}

func TestRecordThreadVisitReportsLocalFailureWithoutLosingThread(t *testing.T) {
	ctx := context.Background()
	store, err := local.Open(ctx, failingDisk{})
	if err != nil {
		t.Fatal(err)
	}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) },
		store, &credentials{value: Credential{Username: "alice", Token: "token"}})
	info := a.SessionInfo()
	if err := a.RecordThreadVisit(ctx, info.Generation, "Go", "42", "主题"); err == nil {
		t.Fatal("failing disk reported a successful history write")
	}
	// The read path is independent: the same account can still read a thread
	// when only history persistence fails.
	if a.SessionInfo().Username != "alice" || !a.SessionInfo().Authenticated {
		t.Fatal("history failure changed the session")
	}
}

func TestSessionGenerationChangesWithAccountBoundary(t *testing.T) {
	ctx := context.Background()
	creds := &accountsCredentialStore{accounts: []Credential{
		{Username: "alice", Token: "a"},
		{Username: "bob", Token: "b"},
	}, active: "alice"}
	store, err := local.Open(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) }, store, creds)
	first := a.SessionInfo().Generation
	if err := a.SwitchAccount(ctx, "bob"); err != nil {
		t.Fatal(err)
	}
	second := a.SessionInfo().Generation
	if second == first {
		t.Fatal("switching accounts did not advance the session generation")
	}
	if err := a.RecordThreadVisit(ctx, first, "Go", "42", "旧账号主题"); !errors.Is(err, ErrStaleSession) {
		t.Fatalf("old generation history = %v, want stale", err)
	}
	if err := a.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if a.SessionInfo().Generation == second {
		t.Fatal("logout did not advance the session generation")
	}
}

// accountsCredentialStore is a minimal multi-account store for generation tests.
type accountsCredentialStore struct {
	accounts []Credential
	active   string
}

func (s *accountsCredentialStore) Load(context.Context) (Credential, error) {
	for _, c := range s.accounts {
		if c.Username == s.active {
			return c, nil
		}
	}
	return Credential{}, nil
}
func (s *accountsCredentialStore) Save(_ context.Context, c Credential) error {
	s.accounts = append(s.accounts, c)
	s.active = c.Username
	return nil
}
func (s *accountsCredentialStore) Clear(context.Context) error { s.active = ""; return nil }
func (s *accountsCredentialStore) Accounts(context.Context) ([]Credential, error) {
	return append([]Credential(nil), s.accounts...), nil
}
func (s *accountsCredentialStore) LoadAccount(_ context.Context, username string) (Credential, error) {
	for _, c := range s.accounts {
		if accountKey(c.Username) == accountKey(username) {
			return c, nil
		}
	}
	return Credential{}, ErrSavedAccountNotFound
}
func (s *accountsCredentialStore) SetActive(_ context.Context, username string) error {
	s.active = username
	return nil
}
func (s *accountsCredentialStore) DeleteAccount(_ context.Context, username string) error {
	for i, c := range s.accounts {
		if accountKey(c.Username) == accountKey(username) {
			s.accounts = append(s.accounts[:i], s.accounts[i+1:]...)
			return nil
		}
	}
	return nil
}

func TestReadThreadRejectsOldSessionCacheWrite(t *testing.T) {
	started, gate := make(chan struct{}), make(chan struct{})
	requestToken := make(chan string, 1)
	creds := &credentials{value: Credential{Username: "alice", Token: "old-token"}}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		requestToken <- r.URL.Query().Get("oauth_token")
		close(started)
		<-gate
		io.WriteString(w, `{"id":42,"board_name":"Go","collect":1,"article":[{"id":42,"position":1,"content":"旧会话正文"}]}`)
	}, nil, creds)
	defer func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	}()
	type readResult struct {
		detail ThreadDetail
		err    error
	}
	done := make(chan readResult, 1)
	go func() {
		detail, err := a.ReadThread(context.Background(), "Go", "42", forum.Page{Number: 1})
		done <- readResult{detail: detail, err: err}
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("thread request did not start")
	}
	if token := <-requestToken; token != "old-token" {
		t.Fatalf("request token = %q, want the credential bound at entry", token)
	}
	if err := a.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	creds.value = Credential{Username: "alice", Token: "new-token"}
	if err := a.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(gate)
	select {
	case result := <-done:
		if !errors.Is(result.err, ErrStaleSession) {
			t.Fatalf("old read error = %v, want ErrStaleSession", result.err)
		}
		if result.detail.Thread.ID != "" || result.detail.CanonicalBoard != "" {
			t.Fatalf("old result was returned for application: %+v", result.detail)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("thread request did not finish")
	}
	if a.CollectionState("Go", "42", false) {
		t.Fatal("old session response modified the new session collection cache")
	}
}

func TestReadThreadRejectsExpiredBoundCredentialBeforeHTTP(t *testing.T) {
	var calls atomic.Int32
	creds := &credentials{value: Credential{Username: "alice", Token: "fixture-token", ExpiresAt: time.Now().Add(time.Minute).Unix()}}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.WriteString(w, `{"id":42,"board_name":"Go"}`)
	}, nil, creds)
	a.mu.Lock()
	a.session.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	a.mu.Unlock()
	if _, err := a.Token(context.Background()); !errors.Is(err, forum.ErrUnauthenticated) {
		t.Fatalf("normal expired token = %v, want ErrUnauthenticated", err)
	}
	if _, err := a.ReadThread(context.Background(), "Go", "42", forum.Page{Number: 1}); !errors.Is(err, forum.ErrUnauthenticated) {
		t.Fatalf("ReadThread expired token = %v, want ErrUnauthenticated", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("expired ReadThread issued %d HTTP requests", calls.Load())
	}
}

func TestTokenValidityForCurrentAndBoundCredentials(t *testing.T) {
	ctx := context.Background()
	creds := &credentials{value: Credential{Username: "alice", Token: "no-expiry"}}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"code":"0000"}`) }, nil, creds)
	if token, err := a.Token(ctx); err != nil || token != "no-expiry" {
		t.Fatalf("no-expiry token = %q, %v", token, err)
	}
	if token, err := a.Token(withCredential(ctx, Credential{Username: "alice", Token: "bound-no-expiry"})); err != nil || token != "bound-no-expiry" {
		t.Fatalf("bound no-expiry token = %q, %v", token, err)
	}
	future := time.Now().Add(time.Minute).Unix()
	if token, err := a.Token(withCredential(ctx, Credential{Username: "alice", Token: "bound-future", ExpiresAt: future})); err != nil || token != "bound-future" {
		t.Fatalf("bound valid token = %q, %v", token, err)
	}
	if _, err := a.Token(withCredential(ctx, Credential{Username: "alice", Token: "bound-expired", ExpiresAt: time.Now().Add(-time.Minute).Unix()})); !errors.Is(err, forum.ErrUnauthenticated) {
		t.Fatalf("bound expired token = %v, want ErrUnauthenticated", err)
	}
	if _, err := a.Token(withCredential(ctx, Credential{Username: "alice", Token: ""})); !errors.Is(err, forum.ErrUnauthenticated) {
		t.Fatalf("bound empty token = %v, want ErrUnauthenticated", err)
	}
}
