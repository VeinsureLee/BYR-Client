package fyneproto

import (
	"bytes"
	"context"
	"image"
	_ "image/png"
	"strings"
	"sync"
	"testing"
	"time"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/source/demo"
)

func awaitSnapshot(t *testing.T, c *Controller, accept func(Snapshot) bool) Snapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s := c.Snapshot()
		if accept(s) {
			return s
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("controller condition not reached: %+v", c.Snapshot())
	return Snapshot{}
}

func TestDemoCanReadPagesAndLoadEntireStressThread(t *testing.T) {
	c := NewDemo(Options{StressReplies: 1200})
	t.Cleanup(c.Close)
	c.Start()
	s := awaitSnapshot(t, c, func(s Snapshot) bool { return s.Ready && !s.Thread.Loading && s.Thread.Article.BodyLoaded })
	if len(s.Board.Articles) != domain.DefaultBoardBatchSize || s.Thread.Key != StressKey() {
		t.Fatalf("unexpected initial selection: board=%d thread=%v", len(s.Board.Articles), s.Thread.Key)
	}
	if len(s.Thread.Replies) != domain.DefaultRepliesPerPage || s.Thread.End {
		t.Fatal("opening the thread must read only the first reply page")
	}
	c.LoadMoreBoard()
	awaitSnapshot(t, c, func(s Snapshot) bool { return !s.Board.Loading && len(s.Board.Articles) == 40 })
	c.LoadMoreReplies()
	awaitSnapshot(t, c, func(s Snapshot) bool { return !s.Thread.Loading && len(s.Thread.Replies) == 20 })
	c.LoadAllReplies()
	s = awaitSnapshot(t, c, func(s Snapshot) bool { return !s.Thread.Loading && s.Thread.End })
	if len(s.Thread.Replies) != 1200 {
		t.Fatalf("got %d replies, want 1200", len(s.Thread.Replies))
	}
	seen := make(map[domain.ReplyID]bool)
	quotes, images := 0, 0
	for _, r := range s.Thread.Replies {
		if seen[r.Key.ReplyID] {
			t.Fatalf("duplicate reply after pagination: %s", r.Key.ReplyID)
		}
		seen[r.Key.ReplyID] = true
		if r.QuoteAuthor != "" {
			quotes++
		}
		images += len(r.Images)
	}
	if quotes == 0 || images == 0 {
		t.Fatal("stress fixture must exercise quotes and images")
	}
	data, err := c.ReadImage(context.Background(), s.Thread.Article.Images[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || img.Width != 800 || img.Height != 450 {
		t.Fatalf("local image cannot be decoded: %+v %v", img, err)
	}
}

type delayedSource struct {
	*demo.Source
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *delayedSource) ReadThreadPage(ctx context.Context, scope domain.Scope, key domain.ArticleKey, cursor domain.Cursor, limit int) (domain.ThreadPage, error) {
	if key.ArticleID == "b-campus-a001" {
		s.once.Do(func() { close(s.started) })
		<-s.release // Deliberately ignore cancellation, as a non-cooperative adapter might.
		return s.Source.ReadThreadPage(context.Background(), scope, key, cursor, limit)
	}
	return s.Source.ReadThreadPage(ctx, scope, key, cursor, limit)
}

func TestOldThreadResponseDoesNotReplaceNewSelection(t *testing.T) {
	source := &delayedSource{Source: demo.New(time.Now()), started: make(chan struct{}), release: make(chan struct{})}
	c := New(source)
	t.Cleanup(c.Close)
	c.Start()
	select {
	case <-source.started:
	case <-time.After(5 * time.Second):
		t.Fatal("initial delayed thread was not requested")
	}
	newKey := domain.ArticleKey{SiteID: "byr", BoardID: "b-campus", ArticleID: "b-campus-a002"}
	c.OpenThread(newKey)
	awaitSnapshot(t, c, func(s Snapshot) bool {
		return s.Thread.Key == newKey && s.Thread.Article.BodyLoaded && !s.Thread.Loading
	})
	close(source.release)
	c.Close() // Join both reads before inspecting the final visible selection.
	s := c.Snapshot()
	if s.SelectedThread != newKey || s.Thread.Key != newKey || s.Thread.Article.Key != newKey {
		t.Fatalf("late first request replaced newer thread: %+v", s.Thread)
	}
}

func TestSimulationStopsWithoutDiscardingOutputAndCanRestart(t *testing.T) {
	c := NewDemo(Options{StreamInterval: time.Millisecond})
	t.Cleanup(c.Close)
	c.Start()
	awaitSnapshot(t, c, func(s Snapshot) bool { return s.Ready })
	c.StartChat("第一轮问题", nil)
	awaitSnapshot(t, c, func(s Snapshot) bool { return s.Chat.Running && len(s.Chat.Answer) > 0 })
	c.StopChat()
	stopped := c.Snapshot().Chat
	if stopped.Running || !stopped.Simulated || stopped.Answer == "" {
		t.Fatalf("stop lost partial simulated output: %+v", stopped)
	}
	c.wg.Wait()
	if current := c.Snapshot().Chat; current.Answer != stopped.Answer || current.Status != stopped.Status {
		t.Fatal("late stream delta changed a stopped response")
	}
	c.StartChat("第二轮问题", nil)
	s := awaitSnapshot(t, c, func(s Snapshot) bool { return !s.Chat.Running && strings.Contains(s.Chat.Status, "模拟完成") })
	if !strings.Contains(s.Chat.Answer, "第二轮问题") || strings.Contains(s.Chat.Answer, "第一轮问题") {
		t.Fatal("new chat was contaminated by the stopped response")
	}
}

func TestKeywordSearchUsesExistingApplicationService(t *testing.T) {
	c := NewDemo(Options{})
	t.Cleanup(c.Close)
	c.Start()
	awaitSnapshot(t, c, func(s Snapshot) bool { return s.Ready })
	c.Search("Fyne")
	s := awaitSnapshot(t, c, func(s Snapshot) bool { return s.SearchID != "" && s.Search.Status == "completed" })
	if s.Search.ArticleCount != 1 {
		t.Fatalf("unexpected Fyne result count: %d", s.Search.ArticleCount)
	}
	found := false
	for _, board := range s.Search.Boards {
		for _, result := range board.Articles {
			found = found || result.Article.Key == StressKey()
		}
	}
	if !found {
		t.Fatal("search did not locate the prototype stress thread")
	}
}
