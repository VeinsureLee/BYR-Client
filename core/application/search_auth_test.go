package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

// controlledSearchForum supplies only the reads used by the all-board search
// use case. The embedded interface provides the rest of the contract without
// contacting a real protocol client.
type controlledSearchForum struct {
	ForumClient
	boards []string
	search func(context.Context, forum.Search) (forum.SearchResults, error)
}

func (f *controlledSearchForum) Sections(context.Context) (forum.SectionList, error) {
	return forum.SectionList{Sections: forum.List[forum.Section]{{Name: "0"}}}, nil
}

func (f *controlledSearchForum) Section(context.Context, string) (forum.Section, error) {
	boards := make(forum.List[forum.Board], len(f.boards))
	for i, board := range f.boards {
		boards[i] = forum.Board{Name: board}
	}
	return forum.Section{Name: "0", Boards: boards}, nil
}

func (f *controlledSearchForum) SearchArticles(ctx context.Context, query forum.Search) (forum.SearchResults, error) {
	return f.search(ctx, query)
}

func newControlledSearchApp(t *testing.T, client ForumClient) *App {
	t.Helper()
	store, err := local.Open(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(Options{
		Local:        store,
		ForumFactory: testForumFactory("http://127.0.0.1:1", nil),
		Credentials:  &credentials{value: Credential{Username: "alice", Token: "token"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	a.forum = client
	return a
}

func TestSearchAllBoardsPrioritizesLateUnauthenticatedError(t *testing.T) {
	failReturned := make(chan struct{})
	var failOnce sync.Once
	client := &controlledSearchForum{
		boards: []string{"Fail", "OK", "Auth"},
		search: func(ctx context.Context, query forum.Search) (forum.SearchResults, error) {
			switch query.Board {
			case "Fail":
				failOnce.Do(func() { close(failReturned) })
				return forum.SearchResults{}, errors.New("board 502")
			case "OK":
				return forum.SearchResults{
					Articles:   forum.List[forum.Article]{{ID: "42", GroupID: "42", Board: "Go", Title: "partial hit"}},
					Pagination: forum.Pagination{TotalItems: 1},
				}, nil
			case "Auth":
				select {
				case <-failReturned:
				case <-ctx.Done():
					return forum.SearchResults{}, ctx.Err()
				}
				// Let the worker record the earlier 502 before authentication
				// arrives, reproducing the late-invalid-token ordering.
				time.Sleep(20 * time.Millisecond)
				return forum.SearchResults{}, fmt.Errorf("read forum credential: %w", forum.ErrUnauthenticated)
			default:
				return forum.SearchResults{}, nil
			}
		},
	}
	a := newControlledSearchApp(t, client)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := a.Search(ctx, SearchRequest{Keyword: "needle", Size: 30})
	if !errors.Is(err, forum.ErrUnauthenticated) {
		t.Fatalf("search error = %v, want ErrUnauthenticated", err)
	}
	if len(result.Articles) != 0 {
		t.Fatalf("authentication failure returned partial results: %+v", result.Articles)
	}
}

func TestSearchAllBoardsCallerCancellationStillWins(t *testing.T) {
	started := make(chan struct{})
	var startOnce sync.Once
	client := &controlledSearchForum{
		boards: []string{"Blocked"},
		search: func(ctx context.Context, query forum.Search) (forum.SearchResults, error) {
			startOnce.Do(func() { close(started) })
			<-ctx.Done()
			return forum.SearchResults{}, ctx.Err()
		},
	}
	a := newControlledSearchApp(t, client)
	ctx, cancel := context.WithCancel(context.Background())
	completed := make(chan error, 1)
	go func() {
		_, err := a.Search(ctx, SearchRequest{Keyword: "needle", Size: 30})
		completed <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("search did not start")
	}
	cancel()
	select {
	case err := <-completed:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled search error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled search did not return")
	}
}
