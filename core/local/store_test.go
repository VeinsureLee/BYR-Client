package local

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

type memoryDisk struct {
	data []byte
	fail bool
}

func (m *memoryDisk) Load(context.Context) ([]byte, error) {
	return append([]byte(nil), m.data...), nil
}
func (m *memoryDisk) Save(_ context.Context, b []byte) error {
	if m.fail {
		return errors.New("disk full")
	}
	m.data = append([]byte(nil), b...)
	return nil
}
func TestAccountIsolationRoundTripAndHistoryBound(t *testing.T) {
	ctx := context.Background()
	disk := &memoryDisk{}
	s, err := Open(ctx, disk)
	if err != nil {
		t.Fatal(err)
	}
	key := DraftKey("Go", "42", "")
	if err = s.SaveDraft(ctx, "Alice", key, Draft{Title: "回复", Content: "草稿"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Draft("bob", key); ok {
		t.Fatal("draft crossed account boundary")
	}
	for i := 0; i < 105; i++ {
		if err = s.Visit(ctx, "alice", History{Board: "Go", ID: fmt.Sprint(i), VisitedAt: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Visit(ctx, "ALICE", History{Board: "Go", ID: "50"}); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(ctx, disk)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := restored.Draft("alice", key)
	if !ok || v.Content != "草稿" {
		t.Fatalf("%+v %t", v, ok)
	}
	h := restored.History("alice")
	if len(h) != 100 || h[0].ID != "50" || h[1].ID != "104" {
		t.Fatal(h)
	}
	if len(restored.History("bob")) != 0 {
		t.Fatal("history leaked")
	}
	if err = restored.SaveDraft(ctx, "alice", key, Draft{}); err != nil {
		t.Fatal(err)
	}
	if _, ok = restored.Draft("alice", key); ok {
		t.Fatal("empty draft retained")
	}
}

func TestReplyAndEditDraftsHaveDistinctKeys(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	replyKey := DraftKey("Go", "42", "")
	editKey := DraftKey("Go", "", "42")
	if replyKey == editKey {
		t.Fatalf("reply and edit drafts share key %q", replyKey)
	}
	if err := s.SaveDraft(ctx, "alice", replyKey, Draft{Content: "reply"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveDraft(ctx, "alice", editKey, Draft{Content: "edit"}); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.Draft("alice", replyKey); !ok || got.Content != "reply" {
		t.Fatalf("reply draft = %+v, ok=%v", got, ok)
	}
	if got, ok := s.Draft("alice", editKey); !ok || got.Content != "edit" {
		t.Fatalf("edit draft = %+v, ok=%v", got, ok)
	}
}

func TestWriteFailureDoesNotPublishUnpersistedState(t *testing.T) {
	ctx := context.Background()
	disk := &memoryDisk{}
	s, _ := Open(ctx, disk)
	disk.fail = true
	if err := s.SetPreferences(ctx, Preferences{Theme: 2}); err == nil {
		t.Fatal("missing failure")
	}
	if s.Preferences().Theme != 0 {
		t.Fatal("failed write changed state")
	}
}
func TestConcurrentDraftAndHistoryWrites(t *testing.T) {
	ctx := context.Background()
	disk := &memoryDisk{}
	s, _ := Open(ctx, disk)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := s.SaveDraft(ctx, "a", fmt.Sprint(i), Draft{Content: fmt.Sprint(i)}); err != nil {
				t.Error(err)
			}
			if err := s.Visit(ctx, "a", History{Board: "Go", ID: fmt.Sprint(i)}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	restored, err := Open(ctx, disk)
	if err != nil || len(restored.History("a")) != 20 {
		t.Fatalf("history lost: %v", err)
	}
	for i := 0; i < 20; i++ {
		if _, ok := restored.Draft("a", fmt.Sprint(i)); !ok {
			t.Fatalf("draft %d lost", i)
		}
	}
}
func TestUnsupportedOrCorruptDataIsNotOverwritten(t *testing.T) {
	for _, data := range []string{`{bad`, `{"version":2}`} {
		disk := &memoryDisk{data: []byte(data)}
		if _, err := Open(context.Background(), disk); err == nil {
			t.Fatal("accepted incompatible data")
		}
		if string(disk.data) != data {
			t.Fatal("input overwritten")
		}
	}
}
