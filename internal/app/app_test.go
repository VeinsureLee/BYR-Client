package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/ports"
	"BYR-Client/internal/source/demo"
	"BYR-Client/internal/store/memory"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func newTestApp(t *testing.T) (*App, fixedClock) {
	t.Helper()
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	clock := fixedClock{now: now}
	source := demo.New(now)
	store := memory.New()
	a := New(source, store, clock, 10*time.Minute)
	if err := a.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	a.Start()
	t.Cleanup(a.Close)
	return a, clock
}

func waitRun(t *testing.T, a *App, id string) RunSnapshot {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		run, ok := a.Run(id)
		if !ok {
			t.Fatalf("run %q not found", id)
		}
		if run.Status != StatusQueued && run.Status != StatusRunning {
			return run
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("run %q did not finish", id)
	return RunSnapshot{}
}

func TestBrowseAndThreadAreLoadedOnDemand(t *testing.T) {
	a, _ := newTestApp(t)
	if _, ok := a.Board("b-go"); ok {
		t.Fatal("board was loaded during initialization")
	}
	if err := a.OpenBoard(context.Background(), "b-go"); err != nil {
		t.Fatal(err)
	}
	board, ok := a.Board("b-go")
	if !ok || len(board.Articles) != domain.DefaultBoardBatchSize {
		t.Fatalf("board window = %+v", board)
	}
	if board.End {
		t.Fatal("first board window should have another page")
	}
	key := board.Articles[0].Key
	if err := a.OpenThread(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	thread, ok := a.Thread(key)
	if !ok || len(thread.Replies) == 0 || !thread.Article.BodyLoaded {
		t.Fatalf("thread = %+v", thread)
	}
	if _, ok := a.Board("b-front"); ok {
		t.Fatal("opening one board loaded another")
	}
}

func TestKeywordSearchRespectsLimitsAndPinnedPosts(t *testing.T) {
	a, _ := newTestApp(t)
	q := domain.SearchQuery{
		Keywords:    []string{"Go"},
		Time:        domain.TimeRange{Kind: domain.TimeAll},
		PerBoard:    2,
		Total:       3,
		ThreadPages: 2,
		Concurrency: 2,
	}
	id, err := a.StartSearch(q)
	if err != nil {
		t.Fatal(err)
	}
	run := waitRun(t, a, id)
	if run.Status != StatusCompleted {
		t.Fatalf("status = %s, error = %s", run.Status, run.Error)
	}
	if run.ArticleCount != 3 {
		t.Fatalf("article count = %d, want 3", run.ArticleCount)
	}
	if run.StopReason != StopResultLimit {
		t.Fatalf("stop reason = %s, want result limit", run.StopReason)
	}
	for _, board := range run.Boards {
		if board.Accepted > 2 {
			t.Fatalf("board %s exceeded the per-board limit", board.BoardID)
		}
		for _, article := range board.Articles {
			if article.Article.Pinned {
				t.Fatalf("pinned article %s entered keyword results", article.Article.Title)
			}
			if len(article.Replies) == 0 {
				t.Fatalf("article %s has no loaded replies", article.Article.Title)
			}
			if article.ThreadPages > 2 {
				t.Fatalf("article %s exceeded the thread page budget", article.Article.Title)
			}
		}
	}
	stats := a.Stats()
	if stats.Articles == 0 || stats.Replies == 0 {
		t.Fatalf("store stats = %+v", stats)
	}
}

func TestHistoryPinDeleteUndo(t *testing.T) {
	a, _ := newTestApp(t)
	id, err := a.StartSearch(domain.SearchQuery{
		Keywords: []string{"Go"},
		Time:     domain.TimeRange{Kind: domain.TimeAll},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, a, id)
	if !a.PinHistory(id, true) {
		t.Fatal("pin failed")
	}
	pinned, recent := a.History()
	if len(pinned) != 1 || len(recent) != 0 {
		t.Fatalf("history pinned=%d recent=%d", len(pinned), len(recent))
	}
	if !a.DeleteHistory(id) {
		t.Fatal("delete failed")
	}
	if _, ok := a.HistoryEntry(id); ok {
		t.Fatal("deleted entry still visible")
	}
	if !a.UndoDelete() {
		t.Fatal("undo failed")
	}
	if _, ok := a.HistoryEntry(id); !ok {
		t.Fatal("entry not restored")
	}
}

func TestSearchRequiresKeywordOrAuthor(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.StartSearch(domain.SearchQuery{}); err == nil {
		t.Fatal("empty search accepted")
	}
}

type countingSource struct {
	base        ports.ForumSource
	boardCalls  atomic.Int32
	threadCalls atomic.Int32
	searchCalls atomic.Int32
}

func (s *countingSource) ReadCatalog(ctx context.Context) (domain.Catalog, error) {
	return s.base.ReadCatalog(ctx)
}

func (s *countingSource) ReadBoardPage(ctx context.Context, scope domain.Scope, board domain.BoardID, cursor domain.Cursor, limit int) (domain.BoardPage, error) {
	s.boardCalls.Add(1)
	return s.base.ReadBoardPage(ctx, scope, board, cursor, limit)
}

func (s *countingSource) ReadThreadPage(ctx context.Context, scope domain.Scope, key domain.ArticleKey, cursor domain.Cursor, limit int) (domain.ThreadPage, error) {
	s.threadCalls.Add(1)
	return s.base.ReadThreadPage(ctx, scope, key, cursor, limit)
}

func (s *countingSource) SearchBoardPage(ctx context.Context, scope domain.Scope, board domain.BoardID, query domain.BoardQuery, cursor domain.Cursor, limit int) (domain.BoardPage, error) {
	s.searchCalls.Add(1)
	return s.base.SearchBoardPage(ctx, scope, board, query, cursor, limit)
}

func TestInitializationReadsOnlyCatalog(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	source := &countingSource{base: demo.New(now)}
	application := New(source, memory.New(), fixedClock{now: now}, 10*time.Minute)
	t.Cleanup(application.Close)
	if err := application.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if source.boardCalls.Load() != 0 || source.threadCalls.Load() != 0 || source.searchCalls.Load() != 0 {
		t.Fatalf("initialization read content: board=%d thread=%d search=%d",
			source.boardCalls.Load(), source.threadCalls.Load(), source.searchCalls.Load())
	}
	if err := application.OpenBoard(context.Background(), "b-go"); err != nil {
		t.Fatal(err)
	}
	if source.boardCalls.Load() != 1 || source.threadCalls.Load() != 0 {
		t.Fatalf("opening a board read other resources: board=%d thread=%d", source.boardCalls.Load(), source.threadCalls.Load())
	}
	board, _ := application.Board("b-go")
	if err := application.OpenThread(context.Background(), board.Articles[0].Key); err != nil {
		t.Fatal(err)
	}
	if source.threadCalls.Load() != 1 {
		t.Fatalf("opening a thread made %d thread requests", source.threadCalls.Load())
	}
}

func TestSearchKeepsPagingUntilPerBoardLimit(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	source := &countingSource{base: demo.New(now)}
	application := New(source, memory.New(), fixedClock{now: now}, 10*time.Minute)
	t.Cleanup(application.Close)
	if err := application.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	id, err := application.StartSearch(domain.SearchQuery{
		Keywords:    []string{"Go"},
		Time:        domain.TimeRange{Kind: domain.TimeAll},
		ScopeNode:   "b-go",
		PerBoard:    25,
		Total:       100,
		ThreadPages: 1,
		Concurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	run := waitRun(t, application, id)
	if run.ArticleCount != 25 {
		t.Fatalf("article count = %d, want 25", run.ArticleCount)
	}
	if run.StopReason != StopPerBoard {
		t.Fatalf("stop reason = %s, want per-board limit", run.StopReason)
	}
	if source.searchCalls.Load() < 2 {
		t.Fatalf("search stopped after %d candidate page(s)", source.searchCalls.Load())
	}
}

func TestRepeatedSearchFetchesAgainWithoutDuplicatingEntities(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	source := &countingSource{base: demo.New(now)}
	application := New(source, memory.New(), fixedClock{now: now}, 10*time.Minute)
	t.Cleanup(application.Close)
	if err := application.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	query := domain.SearchQuery{
		Keywords:    []string{"Go"},
		Time:        domain.TimeRange{Kind: domain.TimeAll},
		ScopeNode:   "b-go",
		PerBoard:    5,
		Total:       5,
		ThreadPages: 1,
		Concurrency: 1,
	}
	firstID, err := application.StartSearch(query)
	if err != nil {
		t.Fatal(err)
	}
	first := waitRun(t, application, firstID)
	firstStats := application.Stats()
	firstCalls := source.searchCalls.Load()

	secondID, err := application.StartSearch(query)
	if err != nil {
		t.Fatal(err)
	}
	second := waitRun(t, application, secondID)
	secondStats := application.Stats()
	if first.ArticleCount != second.ArticleCount {
		t.Fatalf("second run returned %d topics, first returned %d", second.ArticleCount, first.ArticleCount)
	}
	if source.searchCalls.Load() <= firstCalls {
		t.Fatal("second search reused the first network result")
	}
	if secondStats.Articles != firstStats.Articles || secondStats.Replies != firstStats.Replies {
		t.Fatalf("entities were duplicated: first=%+v second=%+v", firstStats, secondStats)
	}
}

type fakeAuth struct {
	session ports.Session
}

func (a *fakeAuth) Login(_ context.Context, username, password string) (ports.Session, error) {
	if password != "secret" {
		return ports.Session{}, errors.New("密码错误")
	}
	a.session = ports.Session{SiteID: "byr", AccountID: username, DisplayName: username, Epoch: 1, LoggedIn: true}
	return a.session, nil
}

func (a *fakeAuth) Restore(context.Context) (ports.Session, bool, error) {
	return a.session, a.session.LoggedIn, nil
}

func (a *fakeAuth) Logout(context.Context) error {
	a.session = ports.Session{SiteID: "byr"}
	return nil
}

func (a *fakeAuth) Session() ports.Session { return a.session }

func TestAppRequiresLoginBeforeCatalogInitialization(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	auth := &fakeAuth{}
	application := NewWithAuth(demo.New(now), memory.New(), auth, fixedClock{now: now}, 10*time.Minute)
	t.Cleanup(application.Close)
	if err := application.Initialize(context.Background()); !errors.Is(err, ports.ErrAuthRequired) {
		t.Fatalf("initialize without login = %v", err)
	}
	if _, err := application.Login(context.Background(), "alice", "wrong"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if _, err := application.Login(context.Background(), "alice", "secret"); err != nil {
		t.Fatal(err)
	}
	if err := application.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	stats := application.CatalogStats()
	if !stats.Ready || stats.Forums != 10 || stats.Boards == 0 {
		t.Fatalf("catalog stats = %+v", stats)
	}
	if err := application.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if application.Authenticated() {
		t.Fatal("logout kept the session")
	}
	if application.Navigation() != nil {
		t.Fatal("logout kept the catalog")
	}
	if _, err := application.StartSearch(domain.SearchQuery{Keywords: []string{"Go"}}); !errors.Is(err, ports.ErrAuthRequired) {
		t.Fatalf("search after logout = %v", err)
	}
}

func TestRestoreSessionThenInitialize(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	auth := &fakeAuth{session: ports.Session{SiteID: "byr", AccountID: "alice", DisplayName: "Alice", Epoch: 3, LoggedIn: true}}
	application := NewWithAuth(demo.New(now), memory.New(), auth, fixedClock{now: now}, 10*time.Minute)
	t.Cleanup(application.Close)
	session, ok, err := application.RestoreSession(context.Background())
	if err != nil || !ok || session.AccountID != "alice" {
		t.Fatalf("restore = %+v ok=%v err=%v", session, ok, err)
	}
	if err := application.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !application.CatalogStats().Ready {
		t.Fatal("catalog not ready after restore")
	}
}

func TestDemoImagesAreFetchedThroughApplication(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.OpenBoard(context.Background(), "b-go"); err != nil {
		t.Fatal(err)
	}
	board, _ := a.Board("b-go")
	var key domain.ArticleKey
	for _, article := range board.Articles {
		if len(article.Images) > 0 {
			key = article.Key
			break
		}
	}
	if key.ArticleID == "" {
		t.Fatal("no demo article with an image")
	}
	data, err := a.FetchImage(context.Background(), board.Articles[0].Images[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 {
		t.Fatalf("image data = %d bytes", len(data))
	}
}

func TestTrafficLoadsForDirectoryAndBoard(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.LoadTraffic(context.Background(), "f-tech"); err != nil {
		t.Fatal(err)
	}
	aggregate, ok := a.TrafficForNode("f-tech")
	if !ok || aggregate.Posts == 0 || aggregate.TodayPosts == 0 {
		t.Fatalf("aggregate = %+v ok=%v", aggregate, ok)
	}
	board, ok := a.BoardTraffic("b-go")
	if !ok || board.Threads == 0 {
		t.Fatalf("board traffic = %+v ok=%v", board, ok)
	}
	if a.TrafficLoading("f-tech") {
		t.Fatal("traffic still loading")
	}
}

func TestDirectoryCountsUseDescendantBoardsAndSubForums(t *testing.T) {
	a, _ := newTestApp(t)
	boards, subForums := a.DirectoryCounts("f-tech")
	if boards != 4 || subForums != 1 {
		t.Fatalf("f-tech counts = %d boards, %d sub forums", boards, subForums)
	}
	boards, subForums = a.DirectoryCounts("s-prog")
	if boards != 3 || subForums != 0 {
		t.Fatalf("s-prog counts = %d boards, %d sub forums", boards, subForums)
	}
	boards, subForums = a.DirectoryCounts("")
	if boards == 0 || subForums == 0 {
		t.Fatalf("site counts = %d boards, %d sub forums", boards, subForums)
	}
}

func TestLatestTopicSkipsPinnedPosts(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.LoadLatestTopic(context.Background(), "f-tech"); err != nil {
		t.Fatal(err)
	}
	article, board, ok := a.LatestTopic("f-tech")
	if !ok || article.Key.ArticleID == "" || board == "" {
		t.Fatalf("latest = %+v board=%s ok=%v", article, board, ok)
	}
	if article.Pinned {
		t.Fatalf("latest topic is pinned: %s", article.Title)
	}
}
