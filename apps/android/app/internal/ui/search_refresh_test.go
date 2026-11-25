package ui

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func TestSameSearchRefreshKeepsOldResultsPageAndPositionUntilFailure(t *testing.T) {
	var gate chan struct{}
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/section.json":
			_, _ = w.Write([]byte(`{"section":[{"name":"0"}]}`))
		case "/open/section/0.json":
			_, _ = w.Write([]byte(`{"name":"0","board":[{"name":"Go"}]}`))
		case "/open/search/article.json":
			started <- struct{}{}
			<-gate
			http.Error(w, "search failed", http.StatusBadGateway)
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	// Register server shutdown before the gate release. testing.T runs cleanups
	// in reverse order, so a failed assertion cannot leave the handler blocked.
	t.Cleanup(server.Close)
	gate = make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gate) }) }
	t.Cleanup(release)

	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 2}}
	a.searchMode = "all"
	a.searchHasRun = true
	a.searchListQuery = "needle"
	a.searchEdit.SetText("needle")
	a.searchResults = []forum.Article{{ID: "old", Title: "old search hit"}}
	a.searchPagination = forum.Pagination{CurrentPage: 2, TotalPages: 3, TotalItems: 31}
	key := searchListKey(a.current(), "needle")
	list := a.mobileList(key)
	list.Position.First = 3
	list.Position.Offset = 17

	a.loadSearch()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("same-query refresh did not reach the search endpoint")
	}
	if !a.busy || len(a.searchResults) != 1 || a.searchResults[0].ID != "old" ||
		a.current().Number != 2 || a.searchPagination.CurrentPage != 2 ||
		list.Position.First != 3 || list.Position.Offset != 17 {
		t.Fatalf("loading refresh changed visible search state: busy=%t results=%+v page=%d pagination=%+v position=%d/%d",
			a.busy, a.searchResults, a.current().Number, a.searchPagination, list.Position.First, list.Position.Offset)
	}
	release()
	waitForIdle(t, a)
	if !a.failed || len(a.searchResults) != 1 || a.searchResults[0].ID != "old" ||
		a.current().Number != 2 || list.Position.First != 3 || list.Position.Offset != 17 {
		t.Fatalf("failed refresh replaced the successful snapshot: failed=%t results=%+v page=%d position=%d/%d message=%q",
			a.failed, a.searchResults, a.current().Number, list.Position.First, list.Position.Offset, a.message)
	}
}
