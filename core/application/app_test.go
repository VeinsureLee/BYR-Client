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

type credentials struct {
	value Credential
	fail  bool
}

func (c *credentials) Load(context.Context) (Credential, error) { return c.value, nil }
func (c *credentials) Save(_ context.Context, v Credential) error {
	if c.fail {
		return errors.New("keyring locked")
	}
	c.value = v
	return nil
}
func (c *credentials) Clear(context.Context) error {
	if c.fail {
		return errors.New("keyring locked")
	}
	c.value = Credential{}
	return nil
}

type failingDisk struct{}

func (failingDisk) Load(context.Context) ([]byte, error) { return nil, nil }
func (failingDisk) Save(context.Context, []byte) error   { return errors.New("disk full") }
func appFor(t *testing.T, handler http.HandlerFunc, store *local.Store, creds *credentials) *App {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	if store == nil {
		store, _ = local.Open(context.Background(), nil)
	}
	a, err := New(Options{BaseURL: s.URL, Local: store, Credentials: creds})
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	return a
}
func TestLoginPersistenceExpiryAndLogout(t *testing.T) {
	ctx := context.Background()
	creds := &credentials{}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"access_token":"oauth-token","expires_in":3600}`)
	}, nil, creds)
	if err := a.Login(ctx, "alice", "pw", true); err != nil {
		t.Fatal(err)
	}
	if creds.value.Token != "oauth-token" || a.Session().Username != "alice" {
		t.Fatal("session not persisted")
	}
	creds.fail = true
	if err := a.Logout(ctx); err == nil || a.Session().Token == "" {
		t.Fatal("failed credential clear pretended to logout")
	}
	creds.fail = false
	if err := a.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Token(ctx); !errors.Is(err, forum.ErrUnauthenticated) {
		t.Fatal(err)
	}
	creds.value = Credential{Username: "alice", Token: "expired", ExpiresAt: time.Now().Unix() - 1}
	if err := a.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	if creds.value.Token != "" {
		t.Fatal("expired credential retained")
	}
}
func TestPublishKeepsFailedDraftAndSeparatesCleanupFailure(t *testing.T) {
	ctx := context.Background()
	creds := &credentials{value: Credential{Username: "a", Token: "t"}}
	store, _ := local.Open(ctx, nil)
	key := local.DraftKey("Go", "", "")
	store.SaveDraft(ctx, "a", key, local.Draft{Title: "draft", Content: "body"})
	fail := true
	posts := 0
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open/board/Go.json" {
			io.WriteString(w, `{"allow_post":true}`)
			return
		}
		posts++
		if fail {
			io.WriteString(w, `{"code":"1234","msg":"failed"}`)
		} else {
			io.WriteString(w, `{"id":42}`)
		}
	}, store, creds)
	p := forum.Post{Board: "Go", Title: "draft", Content: "body"}
	if _, err := a.Publish(ctx, p, ""); err == nil {
		t.Fatal("expected failure")
	}
	if _, ok := store.Draft("a", key); !ok {
		t.Fatal("failed publish removed draft")
	}
	fail = false
	if _, err := a.Publish(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Draft("a", key); ok {
		t.Fatal("successful publish retained draft")
	}
	if posts != 2 {
		t.Fatalf("retried mutation: %d", posts)
	}
	broken, _ := local.Open(ctx, failingDisk{})
	a.Local = broken
	result, err := a.Publish(ctx, p, "")
	if err != nil || result.LocalError == nil || result.Article.ID != "42" {
		t.Fatalf("must report published post separately from cleanup failure: %+v %v", result, err)
	}
}
func TestReactionIsOneWayAndCanSwitch(t *testing.T) {
	calls := 0
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		io.WriteString(w, `{"like_sum":2,"votedown_sum":0}`)
	}, nil, &credentials{value: Credential{Username: "a", Token: "t"}})
	v := forum.Article{ID: "42", IsVotedown: true, VotedownSum: 1}
	v, err := a.React(context.Background(), "Go", v, Like)
	if err != nil || !bool(v.IsLiked) || bool(v.IsVotedown) || v.LikeSum != 2 || v.VotedownSum != 0 {
		t.Fatalf("%+v %v", v, err)
	}
	a.React(context.Background(), "Go", v, Like)
	if calls != 1 {
		t.Fatal("clicking liked post called cancellation endpoint")
	}
}

func TestDuplicateLikeConfirmsStateWithoutCounting(t *testing.T) {
	calls := 0
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		io.WriteString(w, `{"code":"1603","msg":"已经赞过"}`)
	}, nil, &credentials{value: Credential{Username: "a", Token: "t"}})
	v := forum.Article{ID: "42", LikeSum: 2}
	v, err := a.React(context.Background(), "Go", v, Like)
	if err != nil || !bool(v.IsLiked) || v.LikeSum != 2 {
		t.Fatalf("duplicate like must confirm without incrementing: %+v %v", v, err)
	}
	if calls != 1 {
		t.Fatalf("like calls = %d", calls)
	}
}

func TestDuplicateLikeOverHTTPStatusIsConfirmed(t *testing.T) {
	calls := 0
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"code":"1603","msg":"已经赞过"}`)
	}, nil, &credentials{value: Credential{Username: "a", Token: "t"}})
	v := forum.Article{ID: "42", LikeSum: 2}
	v, err := a.React(context.Background(), "Go", v, Like)
	if err != nil || !bool(v.IsLiked) || v.LikeSum != 2 {
		t.Fatalf("duplicate like over HTTP status must confirm without incrementing: %+v %v", v, err)
	}
	if calls != 1 {
		t.Fatalf("like calls = %d", calls)
	}
}

func TestReactionAcceptsBooleanStatusFromLiveServer(t *testing.T) {
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status":true,"like_sum":3,"votedown_sum":0}`)
	}, nil, &credentials{value: Credential{Username: "a", Token: "t"}})
	v := forum.Article{ID: "42", LikeSum: 2}
	v, err := a.React(context.Background(), "Go", v, Like)
	if err != nil || !bool(v.IsLiked) || v.LikeSum != 3 {
		t.Fatalf("like with boolean status must succeed and use the server count: %+v %v", v, err)
	}
}

func TestPublishChecksBoardPermissionsBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name, metadata string
		post           forum.Post
	}{{"read only", `{"is_read_only":true}`, forum.Post{}}, {"anonymous", `{"allow_anonymous":false}`, forum.Post{Anonymous: true}}, {"no reply", `{"is_no_reply":true}`, forum.Post{ReplyID: "42"}}, {"no outgo", `{"allow_outgo":false}`, forum.Post{Outgo: true}}} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					writes++
				}
				io.WriteString(w, tc.metadata)
			}, nil, &credentials{value: Credential{Username: "a", Token: "t"}})
			p := tc.post
			p.Board, p.Title, p.Content = "Go", "标题", "正文"
			if _, err := a.Publish(context.Background(), p, ""); err == nil {
				t.Fatal("disallowed publish succeeded")
			}
			if writes != 0 {
				t.Fatal("permission check happened after mutation")
			}
		})
	}
}
func TestConversationLoadsBodiesAndPreservesDirection(t *testing.T) {
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/mail/inbox.json":
			io.WriteString(w, `{"mail":[{"index":1,"user":{"id":"bob"},"post_time":2},{"index":9,"user":{"id":"other"}}]}`)
		case "/open/mail/outbox.json":
			io.WriteString(w, `{"mail":[{"index":3,"user":{"id":"bob"},"post_time":1}]}`)
		case "/open/mail/inbox/1.json":
			io.WriteString(w, `{"index":1,"content":"received body","post_time":2}`)
		case "/open/mail/outbox/3.json":
			io.WriteString(w, `{"index":3,"content":"sent body","post_time":1}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}, nil, &credentials{value: Credential{Username: "alice", Token: "t"}})
	chat, err := a.Conversation(context.Background(), "bob")
	if err != nil || len(chat) != 2 || !chat[0].Mine || chat[0].Mail.Content != "sent body" || chat[1].Mine || chat[1].Mail.Content != "received body" {
		t.Fatalf("%+v %v", chat, err)
	}
}
