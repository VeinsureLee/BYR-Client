package ui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

type recordedRequest struct {
	Method string
	Path   string
	Form   string
}

type interactionServer struct {
	mu       sync.Mutex
	requests []recordedRequest
	server   *httptest.Server
}

func newInteractionServer(t *testing.T) *interactionServer {
	t.Helper()
	s := &interactionServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.requests = append(s.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Form: string(body)})
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/like/42.json"):
			_, _ = w.Write([]byte(`{"status":true,"like_sum":3,"votedown_sum":0}`))
		case strings.HasSuffix(r.URL.Path, "/votedown/44.json"):
			_, _ = w.Write([]byte(`{"status":true,"like_sum":0,"votedown_sum":6}`))
		case strings.HasSuffix(r.URL.Path, "/thread/42.json"):
			_, _ = w.Write([]byte(`{"id":42,"title":"主题","article":[{"id":42,"position":1,"content":"正文","like_sum":3}]}`))
		case r.URL.Path == "/open/collection.json":
			_, _ = w.Write([]byte(`{"article":[{"gid":42,"bname":"Go","title":"收藏文章","num":1,"user":{"id":"alice","user_name":"Alice"}}]}`))
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *interactionServer) find(method, path string) []recordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []recordedRequest
	for _, r := range s.requests {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

func interactionApp(t *testing.T, baseURL string) *App {
	t.Helper()
	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	credentials := &testCredentials{session: application.Credential{
		Username: "alice", Token: "token", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}}
	core, err := application.New(application.Options{
		BaseURL:     baseURL,
		HTTPClient:  http.DefaultClient,
		Local:       store,
		Credentials: credentials,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.ready = true
	a.core = core
	a.stack = []Route{{Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	a.thread = sampleThread()
	return a
}

func TestThreadReactionUpdatesServerCounts(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.likeThread()
	waitForIdle(t, a)
	if len(s.find("GET", "/open/article/Go/like/42.json")) != 1 {
		t.Fatalf("like request missing: %+v", s.requests)
	}
	if got := a.thread.LikeSum; got != 3 {
		t.Fatalf("like count = %d, want 3", got)
	}
	if !bool(a.thread.IsLiked) {
		t.Fatal("thread was not marked liked")
	}
	if len(a.thread.Articles) == 0 || !bool(a.thread.Articles[0].IsLiked) {
		t.Fatal("root post was not updated")
	}

	// A second like must not call the server again.
	a.likeThread()
	waitForIdle(t, a)
	if len(s.find("GET", "/open/article/Go/like/42.json")) != 1 {
		t.Fatalf("duplicate like reached the server: %+v", s.requests)
	}
}

func TestPostDownvoteUpdatesServerCounts(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.react(1, a.thread.Articles[1], application.Downvote)
	waitForIdle(t, a)
	if len(s.find("GET", "/open/article/Go/votedown/44.json")) != 1 {
		t.Fatalf("downvote request missing: %+v", s.requests)
	}
	got := a.thread.Articles[1]
	if got.VotedownSum != 6 || !bool(got.IsVotedown) {
		t.Fatalf("downvote state = %+v", got)
	}
}

func TestPostReplyOpensComposer(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.openCompose("Go", "44", "reply")
	if got := a.current().Page; got != PageCompose {
		t.Fatalf("page = %v, want compose", got)
	}
	if a.compose.replyID != "44" || !a.compose.ready {
		t.Fatalf("reply composer state = %+v", a.compose)
	}
}

func TestCollectThreadTogglesServerState(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.collectThread()
	waitForIdle(t, a)
	if len(s.find("POST", "/open/collection/add.json")) != 1 {
		t.Fatalf("collect request missing: %+v", s.requests)
	}
	if !bool(a.thread.Collect) {
		t.Fatal("thread was not marked collected")
	}
	a.collectThread()
	waitForIdle(t, a)
	if len(s.find("POST", "/open/collection/delete.json")) != 1 {
		t.Fatalf("uncollect request missing: %+v", s.requests)
	}
	if bool(a.thread.Collect) {
		t.Fatal("thread stayed collected")
	}
}

func TestFavoritesTabSwitchLoadsCollection(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageFavorites, Number: 1}}
	a.favoriteTab = "boards"
	a.loadFavorites()
	waitForIdle(t, a)

	a.selectFavoriteTab("collection")
	waitForIdle(t, a)
	if a.favoriteTab != "collection" {
		t.Fatalf("favourite tab = %q, want collection", a.favoriteTab)
	}
	if len(s.find("GET", "/open/collection.json")) != 1 {
		t.Fatalf("collection request missing: %+v", s.requests)
	}
	if len(a.collect.Entries) != 1 || a.collect.Entries[0].ID != "42" {
		t.Fatalf("collection entries = %+v", a.collect.Entries)
	}
	if a.stack[0].Number != 1 {
		t.Fatalf("collection page number = %d, want 1", a.stack[0].Number)
	}
}

func TestFavoritesTabSwitchIgnoredWhileBusy(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageFavorites, Number: 1}}
	a.favoriteTab = "boards"
	a.busy = true
	a.selectFavoriteTab("collection")
	if a.favoriteTab != "boards" {
		t.Fatalf("busy tab switch changed tab to %q", a.favoriteTab)
	}
	a.busy = false
}

func TestCollectionCardOpensThread(t *testing.T) {
	a := interactionApp(t, "http://127.0.0.1:1")
	defer a.Close()
	a.stack = []Route{{Page: PageFavorites, Number: 1}}
	a.favoriteTab = "collection"
	entry := forum.CollectionEntry{
		ID: "42", Board: "Go", Title: "收藏文章", ReplyCount: 1,
		Author: forum.User{ID: "alice", UserName: "Alice"},
	}
	a.openCollectionEntry(entry)
	if got := a.current().Page; got != PageThread {
		t.Fatalf("page = %v, want thread", got)
	}
	if got := a.current().ID; got != "42" {
		t.Fatalf("thread id = %q, want 42", got)
	}
	if !a.core.CollectionState("Go", "42", true) {
		t.Fatal("collection state was not remembered for the opened thread")
	}
}

func TestCollectionRemoveUsesBoardQualifiedRequest(t *testing.T) {
	s := newInteractionServer(t)
	a := interactionApp(t, s.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageFavorites, Number: 1}}
	a.favoriteTab = "collection"

	entry := forum.Article{ID: "42", Board: "Go"}
	a.run("取消收藏", func(ctx context.Context) (func(), error) {
		_, err := a.core.SetCollected(ctx, entry.Board, string(entry.ID), false)
		return func() { a.loadFavorites() }, err
	})
	waitForIdle(t, a)
	requests := s.find("POST", "/open/collection/delete.json")
	if len(requests) != 1 {
		t.Fatalf("uncollect request missing: %+v", s.requests)
	}
	if !strings.Contains(requests[0].Form, "board=Go") || !strings.Contains(requests[0].Form, "id=42") {
		t.Fatalf("uncollect form = %q", requests[0].Form)
	}
}
