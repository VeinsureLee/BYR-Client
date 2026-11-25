package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/appearance"
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

func TestColorSchemeDefaultsAndPreferencePatchPreserveStateV1(t *testing.T) {
	ctx := context.Background()
	legacy := []byte(`{"version":1,"preferences":{"theme":2,"fontScale":1.2,"ipv6":true},"accounts":{"alice":{"drafts":{},"history":[{"Board":"Go","ID":"42","Title":"old"}]}}}`)
	disk := &memoryDisk{data: append([]byte(nil), legacy...)}
	store, err := Open(ctx, disk)
	if err != nil {
		t.Fatal(err)
	}
	prefs := store.Preferences()
	if prefs.Theme != 2 || prefs.FontScale != 1.2 || !prefs.IPv6 || prefs.ColorScheme != string(appearance.BYRBlue) {
		t.Fatalf("legacy v1 preferences = %+v", prefs)
	}

	pine := string(appearance.Pine)
	if err := store.UpdatePreferences(ctx, PreferencePatch{ColorScheme: &pine}); err != nil {
		t.Fatal(err)
	}
	ipv6 := false
	if err := store.UpdatePreferences(ctx, PreferencePatch{IPv6: &ipv6}); err != nil {
		t.Fatal(err)
	}
	prefs = store.Preferences()
	if prefs.ColorScheme != pine || prefs.Theme != 2 || prefs.FontScale != 1.2 || prefs.IPv6 {
		t.Fatalf("targeted patches overwrote unrelated preferences: %+v", prefs)
	}
	var saved document
	if err := json.Unmarshal(disk.data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Version != 1 || saved.Accounts["alice"].History[0].ID != "42" {
		t.Fatalf("preference patch changed the v1 document/history: %+v", saved)
	}
	restored, err := Open(ctx, disk)
	if err != nil {
		t.Fatal(err)
	}
	if got := restored.Preferences(); got.ColorScheme != pine || got.Theme != 2 || got.FontScale != 1.2 || got.IPv6 {
		t.Fatalf("reopened v1 preferences = %+v", got)
	}
}

func TestVersionOneDraftAttachmentFixtureRoundTrips(t *testing.T) {
	ctx := context.Background()
	fixture := []byte(`{"version":1,"preferences":{"theme":1,"fontScale":1,"ipv6":false},"accounts":{"alice":{"drafts":{"composeDraft.Go.new":{"Title":"包含附件","Content":"正文","attachments":[{"file":[{"name":"diagram.png","url":"/api/attachment/Go/99","size":"123","thumbnail_small":"/thumb.png","thumbnail_middle":"/middle.png"}],"remain_space":"456","remain_count":3}]}},"history":[]}}}`)
	disk := &memoryDisk{data: fixture}
	store, err := Open(ctx, disk)
	if err != nil {
		t.Fatal(err)
	}
	key := DraftKey("Go", "", "")
	draft, found := store.Draft("alice", key)
	if !found || draft.Title != "包含附件" || len(draft.Attachments) != 1 || len(draft.Attachments[0].Files) != 1 {
		t.Fatalf("legacy attachment draft = %+v found=%v", draft, found)
	}
	file := draft.Attachments[0].Files[0]
	if file.Name != "diagram.png" || file.URL != "/api/attachment/Go/99" || file.Size != "123" ||
		file.ThumbnailSmall != "/thumb.png" || file.ThumbnailMiddle != "/middle.png" ||
		draft.Attachments[0].RemainSpace != "456" || draft.Attachments[0].RemainCount != 3 {
		t.Fatalf("legacy attachment metadata = %+v / %+v", file, draft.Attachments[0])
	}
	plum := string(appearance.Plum)
	if err := store.UpdatePreferences(ctx, PreferencePatch{ColorScheme: &plum}); err != nil {
		t.Fatal(err)
	}
	var saved document
	if err := json.Unmarshal(disk.data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Version != 1 {
		t.Fatalf("attachment preference update changed state version to %d", saved.Version)
	}
	restored, err := Open(ctx, disk)
	if err != nil {
		t.Fatal(err)
	}
	got, found := restored.Draft("alice", key)
	if !found || len(got.Attachments) != 1 || got.Attachments[0].Files[0].URL != file.URL ||
		got.Attachments[0].RemainSpace != "456" || got.Attachments[0].RemainCount != 3 {
		t.Fatalf("v1 attachment draft after rewrite = %+v found=%v", got, found)
	}
	if prefs := restored.Preferences(); prefs.ColorScheme != plum || prefs.Theme != 1 || prefs.FontScale != 1 {
		t.Fatalf("v1 preferences after attachment roundtrip = %+v", prefs)
	}
}
