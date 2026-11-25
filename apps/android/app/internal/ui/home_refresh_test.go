package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func TestTopRootIsSavedAndRestoredAsThePartitionModule(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageTop, Number: 1}}
	a.topTab = "top"
	a.articles = make([]forum.Article, 10)
	for i := range a.articles {
		a.articles[i] = forum.Article{ID: forum.Scalar(fmt.Sprintf("top-%d", i)), Title: fmt.Sprintf("Top %d", i)}
	}
	renderAppAt(t, a, 420, 700, 1)
	a.topList.Position.First = 3
	a.topList.Position.Offset = 7
	renderAppAt(t, a, 420, 700, 1)

	a.setTab(PageSearch)
	renderAppAt(t, a, 420, 700, 1)
	a.setTab(PageHome)
	renderAppAt(t, a, 420, 700, 1)

	if got := a.current(); got != (Route{Page: PageTop, Number: 1}) {
		t.Fatalf("partition tab restored route %+v, want today's top root", got)
	}
	if a.topTab != "top" || a.navPage(a.current().Page) != PageHome || len(a.articles) != 10 {
		t.Fatalf("partition module state: topTab=%q nav=%v articles=%d", a.topTab, a.navPage(a.current().Page), len(a.articles))
	}
	if a.topList.Position.First != 3 || a.topList.Position.Offset != 7 {
		t.Fatalf("top list position = %+v, want 3/7", a.topList.Position)
	}
	if len(a.stack) != 1 {
		t.Fatalf("top root stack = %+v, want one root route", a.stack)
	}
}

func TestFavoritesRefreshUsesFavoriteEndpointAndKeepsListUntilSuccess(t *testing.T) {
	var mu sync.Mutex
	requests := []string{}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	fail := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.Path)
		mu.Unlock()
		if r.URL.Path != "/open/favorite/0.json" {
			if r.URL.Path == "/open/board/Fresh.json" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"name":"Fresh","description":"新收藏版面"}`))
				return
			}
			http.Error(w, fmt.Sprintf("unexpected %s", r.URL.Path), http.StatusNotFound)
			return
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		if fail {
			http.Error(w, "favorite unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"section":[],"board":[{"name":"Fresh","description":"新收藏版面"}]}`))
	}))
	t.Cleanup(server.Close)
	t.Cleanup(unblock)
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome, Number: 1}}
	a.sectionTab = "favorites"
	a.favoriteTab = "boards"
	a.favoriteBoards = []forum.Board{{Name: "Go", Description: "旧收藏版面"}}
	a.favorite = forum.Favorite{Boards: a.favoriteBoards}
	renderAppAt(t, a, 420, 700, 1)

	a.click("top-refresh").Click()
	renderAppAt(t, a, 420, 700, 1)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("favorites refresh did not request the favorite endpoint")
	}
	if got := a.favoriteBoards; len(got) != 1 || got[0].Name != "Go" {
		t.Fatalf("waiting refresh replaced favorites: %+v", got)
	}
	unblock()
	waitForIdle(t, a)
	mu.Lock()
	gotRequests := append([]string(nil), requests...)
	mu.Unlock()
	if len(gotRequests) != 1 || gotRequests[0] != "/open/favorite/0.json" {
		t.Fatalf("refresh requests = %v, want only the favorite boards endpoint", gotRequests)
	}
	if !a.failed || len(a.favoriteBoards) != 1 || a.favoriteBoards[0].Name != "Go" {
		t.Fatalf("failed refresh lost the list: failed=%t favorites=%+v", a.failed, a.favoriteBoards)
	}

	fail = false
	a.reloadCurrent()
	waitForIdle(t, a)
	if a.failed || len(a.favoriteBoards) != 1 || a.favoriteBoards[0].Name != "Fresh" {
		t.Fatalf("successful retry = failed:%t favorites:%+v", a.failed, a.favoriteBoards)
	}
	if a.message != "刷新成功" {
		t.Fatalf("successful favorites refresh message = %q", a.message)
	}
	mu.Lock()
	gotRequests = append([]string(nil), requests...)
	mu.Unlock()
	favoriteCalls := 0
	for _, path := range gotRequests {
		if path == "/open/favorite/0.json" {
			favoriteCalls++
		}
	}
	if favoriteCalls != 2 {
		t.Fatalf("retry requests = %v, want two favorite-board reads", gotRequests)
	}
}
