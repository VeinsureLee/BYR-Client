package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func TestHomeSnapshotReloadsBoardPageAndRestoresStableAnchorAfterCacheMiss(t *testing.T) {
	var gate chan struct{}
	started := make(chan string, 1)
	var mu sync.Mutex
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/open/board/Go.json" {
			query := r.URL.Query().Get("page")
			mu.Lock()
			queries = append(queries, query)
			mu.Unlock()
			started <- query
			<-gate
			articles := make(forum.List[forum.Article], 0, 21)
			articles = append(articles, forum.Article{ID: "inserted", Position: 1, Title: "new floor", Content: "new body"})
			for i := 0; i < 20; i++ {
				articles = append(articles, forum.Article{
					ID: forum.Scalar(fmt.Sprintf("topic-%02d", i)), Position: forum.Number(i + 2),
					Title: fmt.Sprintf("Topic %02d", i), Content: "body", Author: forum.User{ID: "alice"},
				})
			}
			_ = json.NewEncoder(w).Encode(forum.Board{
				Name: "Go", Description: "Go 讨论", Articles: articles,
				Pagination: forum.Pagination{CurrentPage: 2, TotalPages: 4, TotalItems: 121},
			})
			return
		}
		_, _ = w.Write([]byte(`{"code":"0000"}`))
	}))
	t.Cleanup(server.Close)
	gate = make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gate) }) }
	t.Cleanup(release)

	a := interactionApp(t, server.URL)
	defer a.Close()
	articles := make(forum.List[forum.Article], 20)
	for i := range articles {
		articles[i] = forum.Article{
			ID: forum.Scalar(fmt.Sprintf("topic-%02d", i)), Position: forum.Number(i + 1),
			Title: fmt.Sprintf("Topic %02d", i), Content: "body", Author: forum.User{ID: "alice"},
		}
	}
	board := forum.Board{
		Name: "Go", Description: "Go 讨论", Articles: articles,
		Pagination: forum.Pagination{CurrentPage: 2, TotalPages: 4, TotalItems: 120},
	}
	a.sections = []forum.Section{{Name: "0", Description: "校园", Subsections: []forum.Section{{Name: "1", Description: "院系"}}}}
	a.section = forum.Section{Name: "1", Description: "院系", Parent: "0", Boards: []forum.Board{board}}
	a.boards = []forum.Board{board}
	a.board = board
	a.sectionTab = "all"
	a.stack = []Route{{Page: PageHome}, {Page: PageSection, Name: "1"}, {Page: PageBoard, Name: "Go", Number: 2}}
	key := listKey{Page: PageBoard, Name: "Go", Number: 2}
	state := a.mobileList(key)
	state.Position.First = 5
	state.Position.Offset = 17
	renderAppAt(t, a, 420, 700, 1)
	if got := string(a.board.Articles[state.Position.First].ID); got != "topic-05" {
		t.Fatalf("anchor fixture = %q, want topic-05", got)
	}

	a.setTab(PageSearch)
	snapshot := a.homeSnapshots["all"]
	if snapshot == nil {
		t.Fatal("Home browsing snapshot was not captured")
	}
	snapshot.ready = false // simulate that page data must be reread on return
	a.setTab(PageHome)
	select {
	case page := <-started:
		if page != "2" {
			t.Fatalf("reloaded board page query = %q, want page=2", page)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("return did not reload the cached-missing board page")
	}
	if a.current().Page != PageBoard || a.current().Number != 2 || !a.busy {
		t.Fatalf("pending restored route = %+v busy=%t", a.current(), a.busy)
	}
	release()
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	mu.Lock()
	gotQueries := append([]string(nil), queries...)
	mu.Unlock()
	if len(gotQueries) != 1 || gotQueries[0] != "2" {
		t.Fatalf("board reload queries = %v, want exactly [2]", gotQueries)
	}
	state = a.mobileList(key)
	if state.Position.First != 6 || state.Position.Offset != 17 ||
		string(a.board.Articles[state.Position.First].ID) != "topic-05" {
		t.Fatalf("stable board anchor after insertion = row %d offset %d item=%q, want row 6 offset 17 topic-05",
			state.Position.First, state.Position.Offset, a.board.Articles[state.Position.First].ID)
	}
}
