package application

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"

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
		BaseURL: server.URL,
		Local:   store,
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
		BaseURL: server.URL,
		Local:   store,
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
		BaseURL: server.URL,
		Local:   store,
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
	if !result.Truncated {
		t.Fatal("partial board failure was not reported as truncated")
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
