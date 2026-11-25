package application

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func newLookupTestApp(t *testing.T, baseURL string, credentials CredentialStore) *App {
	t.Helper()
	ctx := context.Background()
	store, err := local.Open(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(Options{Local: store, Credentials: credentials, ForumFactory: testForumFactory(baseURL, nil)})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestLookupUserTrimsIdentityAndUsesSharedQuery(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/open/user/query/alice.json" {
			t.Errorf("lookup request = %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("oauth_token") != "token" {
			t.Errorf("lookup token = %q", r.URL.Query().Get("oauth_token"))
		}
		io.WriteString(w, `{"id":"alice","user_name":"Alice","post_count":"12"}`)
	}))
	defer server.Close()
	a := newLookupTestApp(t, server.URL, &credentials{value: Credential{Username: "alice", Token: "token"}})

	user, err := a.LookupUser(context.Background(), "  alice  ")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != forum.Scalar("alice") || user.UserName != "Alice" || calls != 1 {
		t.Fatalf("lookup user=%+v calls=%d", user, calls)
	}
	if _, err := a.LookupUser(context.Background(), " \t "); err == nil {
		t.Fatal("blank identity was accepted")
	}
	if calls != 1 {
		t.Fatalf("blank lookup made a request, calls=%d", calls)
	}
}

func TestLookupUserDropsResponseAfterAccountSwitch(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("oauth_token"); got != "alice-token" {
			t.Errorf("in-flight lookup token = %q, want original Alice token", got)
		}
		close(started)
		<-release
		io.WriteString(w, `{"id":"alice","user_name":"Alice"}`)
	}))
	defer server.Close()
	creds := &accountsCredentialStore{
		accounts: []Credential{{Username: "alice", Token: "alice-token"}, {Username: "bob", Token: "bob-token"}},
		active:   "alice",
	}
	a := newLookupTestApp(t, server.URL, creds)

	type result struct {
		user forum.User
		err  error
	}
	completed := make(chan result, 1)
	go func() {
		user, err := a.LookupUser(context.Background(), "alice")
		completed <- result{user: user, err: err}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("lookup request did not start")
	}
	if err := a.SwitchAccount(context.Background(), "bob"); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case got := <-completed:
		if !errors.Is(got.err, ErrStaleSession) || got.user.ID != "" {
			t.Fatalf("old-account lookup result = user:%+v error:%v", got.user, got.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("lookup did not complete after release")
	}
	if info := a.SessionInfo(); info.Username != "bob" || !info.Authenticated {
		t.Fatalf("session after switch = %+v", info)
	}
}
