package application

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestSearchUsesSharedRemoteContract(t *testing.T) {
	var got url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/open/search/article.json" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		got = r.URL.Query()
		io.WriteString(w, `{"article":[{"id":42,"group_id":42,"board_name":"Go","title":"北邮搜索结果","user":{"id":"alice"}}],"pagination":{"page_all_count":2,"page_current_count":2,"item_all_count":31}}`)
	}))
	defer server.Close()

	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(Options{
		Local:        store,
		ForumFactory: testForumFactory(server.URL, nil),
		Credentials: &credentials{value: Credential{
			Username: "alice",
			Token:    "token",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}

	result, err := a.Search(context.Background(), SearchRequest{
		Keyword: "  北邮  ",
		Board:   " Go ",
		Page:    2,
		Size:    20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != SearchSourceRemote || len(result.Articles) != 1 || result.Articles[0].Title != "北邮搜索结果" {
		t.Fatalf("result = %+v", result)
	}
	if result.Pagination.TotalPages != 2 || result.Pagination.TotalItems != 31 {
		t.Fatalf("pagination = %+v", result.Pagination)
	}
	want := url.Values{
		"oauth_token": {"token"}, "board": {"Go"}, "title1": {"北邮"},
		"title2": {""}, "titlen": {""}, "author": {""}, "day": {""},
		"m": {""}, "a": {""}, "o": {"1"}, "page": {"2"}, "count": {"20"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("query = %v, want %v", got, want)
	}
}

func TestSearchExpandsAllBoardsWhenTheAPINeedsABoard(t *testing.T) {
	var mu sync.Mutex
	requested := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/section.json":
			io.WriteString(w, `{"section":[{"name":"0"}]}`)
		case "/open/section/0.json":
			io.WriteString(w, `{"name":"0","sub_section":[{"name":"1"}],"board":[{"name":"Go"}]}`)
		case "/open/section/1.json":
			io.WriteString(w, `{"name":"1","board":[{"name":"Advice"}]}`)
		case "/open/search/article.json":
			q := r.URL.Query()
			board := q.Get("board")
			if q.Get("title1") != "北邮" || q.Get("count") != "30" {
				t.Errorf("search query = %v", q)
			}
			mu.Lock()
			requested[board]++
			mu.Unlock()
			switch board {
			case "Go":
				io.WriteString(w, `{"article":[{"id":1,"group_id":1,"board_name":"Go","title":"Go 主题","post_time":100}],"pagination":{"page_all_count":1,"item_all_count":1}}`)
			case "Advice":
				io.WriteString(w, `{"article":[{"id":2,"group_id":2,"board_name":"Advice","title":"Advice 主题","post_time":200}],"pagination":{"page_all_count":1,"item_all_count":1}}`)
			default:
				t.Errorf("unexpected board filter %q", board)
			}
		default:
			t.Errorf("unexpected search request %s", r.URL.Path)
		}
	}))
	defer server.Close()

	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(Options{
		Local:        store,
		ForumFactory: testForumFactory(server.URL, nil),
		Credentials: &credentials{value: Credential{
			Username: "alice",
			Token:    "token",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := a.Search(context.Background(), SearchRequest{Keyword: "北邮", Size: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Articles) != 2 || result.Articles[0].Board != "Advice" || result.Articles[1].Board != "Go" {
		t.Fatalf("all-board results = %+v", result.Articles)
	}
	if result.Pagination.TotalItems != 2 || result.Pagination.TotalPages != 1 || result.Truncated {
		t.Fatalf("all-board pagination = %+v truncated=%t", result.Pagination, result.Truncated)
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(requested, map[string]int{"Go": 1, "Advice": 1}) {
		t.Fatalf("board requests = %+v", requested)
	}
}

func TestSearchAllBoardsKeepsPartialResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/section.json":
			io.WriteString(w, `{"section":[{"name":"0"}]}`)
		case "/open/section/0.json":
			io.WriteString(w, `{"name":"0","board":[{"name":"Go"},{"name":"Advice"}]}`)
		case "/open/search/article.json":
			if r.URL.Query().Get("board") == "Advice" {
				http.Error(w, "board unavailable", http.StatusInternalServerError)
				return
			}
			io.WriteString(w, `{"article":[{"id":1,"group_id":1,"board_name":"Go","title":"Go 主题","post_time":100}],"pagination":{"page_all_count":1,"item_all_count":1}}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()

	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(Options{
		Local:        store,
		ForumFactory: testForumFactory(server.URL, nil),
		Credentials: &credentials{value: Credential{
			Username: "alice",
			Token:    "token",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := a.Search(context.Background(), SearchRequest{Keyword: "北邮", Size: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Articles) != 1 || result.Articles[0].Board != "Go" {
		t.Fatalf("partial results = %+v", result.Articles)
	}
	if !result.Incomplete || result.FailedBoards != 1 || result.Truncated {
		t.Fatalf("partial board failure marks incomplete=%t failed=%d truncated=%t", result.Incomplete, result.FailedBoards, result.Truncated)
	}
}

func TestSearchAllBoardsPartialFailureWithNoHitsIsIncomplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/section.json":
			io.WriteString(w, `{"section":[{"name":"0"}]}`)
		case "/open/section/0.json":
			io.WriteString(w, `{"name":"0","board":[{"name":"Go"},{"name":"Advice"}]}`)
		case "/open/search/article.json":
			if r.URL.Query().Get("board") == "Advice" {
				http.Error(w, "board unavailable", http.StatusBadGateway)
				return
			}
			io.WriteString(w, `{"article":[],"pagination":{"page_all_count":0,"item_all_count":0}}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()

	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(Options{
		Local:        store,
		ForumFactory: testForumFactory(server.URL, nil),
		Credentials:  &credentials{value: Credential{Username: "alice", Token: "token"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := a.Search(context.Background(), SearchRequest{Keyword: "missing", Size: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Articles) != 0 || !result.Incomplete || result.FailedBoards != 1 || result.Truncated {
		t.Fatalf("partial failure with no hits = %+v", result)
	}
}

func TestSearchAllBoardsCompleteEmptyAndTotalFailureAreDistinct(t *testing.T) {
	for _, tc := range []struct {
		name       string
		failBoard  bool
		wantErr    bool
		wantEmpty  bool
		incomplete bool
	}{
		{name: "all successful empty", wantEmpty: true},
		{name: "all boards failed", failBoard: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/open/section.json":
					io.WriteString(w, `{"section":[{"name":"0"}]}`)
				case "/open/section/0.json":
					io.WriteString(w, `{"name":"0","board":[{"name":"Go"}]}`)
				case "/open/search/article.json":
					if tc.failBoard {
						http.Error(w, "board unavailable", http.StatusBadGateway)
						return
					}
					io.WriteString(w, `{"article":[],"pagination":{"page_all_count":0,"item_all_count":0}}`)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
			}))
			defer server.Close()
			store, err := local.Open(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			a, err := New(Options{
				Local:        store,
				ForumFactory: testForumFactory(server.URL, nil),
				Credentials:  &credentials{value: Credential{Username: "alice", Token: "token"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := a.Restore(context.Background()); err != nil {
				t.Fatal(err)
			}
			result, err := a.Search(context.Background(), SearchRequest{Keyword: "missing", Size: 30})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("all-board failure returned result %+v", result)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantEmpty && len(result.Articles) != 0 {
				t.Fatalf("successful empty search returned %+v", result.Articles)
			}
			if result.Incomplete || result.Truncated || result.FailedBoards != 0 {
				t.Fatalf("successful empty search state = %+v", result)
			}
		})
	}
}

func TestSearchPinsStartingAccountAndRejectsAfterSwitch(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/search/article.json" {
			t.Errorf("search request path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("oauth_token"); got != "alice-token" {
			t.Errorf("in-flight search token = %q, want original Alice token", got)
		}
		close(started)
		<-release
		io.WriteString(w, `{"article":[{"id":42,"group_id":42,"board_name":"Go","title":"Alice result"}]}`)
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	creds := &accountsCredentialStore{
		accounts: []Credential{{Username: "alice", Token: "alice-token"}, {Username: "bob", Token: "bob-token"}},
		active:   "alice",
	}
	a, err := New(Options{Local: store, Credentials: creds, ForumFactory: testForumFactory(server.URL, nil)})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	type searchResult struct {
		result SearchResult
		err    error
	}
	completed := make(chan searchResult, 1)
	go func() {
		result, err := a.Search(context.Background(), SearchRequest{Keyword: "北邮", Board: "Go"})
		completed <- searchResult{result: result, err: err}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("search request did not start")
	}
	if err := a.SwitchAccount(context.Background(), "bob"); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case got := <-completed:
		if !errors.Is(got.err, ErrStaleSession) || len(got.result.Articles) != 0 {
			t.Fatalf("old-account search result = %+v error=%v", got.result, got.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("search did not complete after response release")
	}
}

func TestSearchRejectsUnboundedRequest(t *testing.T) {
	calls := 0
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		io.WriteString(w, `{}`)
	}, nil, &credentials{value: Credential{Username: "alice", Token: "token"}})

	if _, err := a.Search(context.Background(), SearchRequest{}); err == nil {
		t.Fatal("empty search request succeeded")
	}
	if calls != 0 {
		t.Fatalf("empty search request made %d calls", calls)
	}
	if _, err := a.Search(context.Background(), SearchRequest{Keyword: "x", Size: 101}); err == nil {
		t.Fatal("oversized search request succeeded")
	}
	if calls != 0 {
		t.Fatalf("invalid search request made %d calls", calls)
	}
}
