package application

import (
	"context"
	"errors"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestDraftUseCasesBindToActiveAccount(t *testing.T) {
	ctx := context.Background()
	creds := &accountsCredentialStore{
		accounts: []Credential{{Username: "alice", Token: "alice-token"}, {Username: "bob", Token: "bob-token"}},
		active:   "alice",
	}
	a := newLookupTestApp(t, "http://forum.test", creds)
	key := local.DraftKey("Go", "42", "")
	aliceSession := a.SessionInfo()
	aliceDraft := local.Draft{Title: "Alice draft", Content: "reply"}
	if err := a.SaveDraft(ctx, aliceSession, key, aliceDraft); err != nil {
		t.Fatal(err)
	}
	if got, found, err := a.LoadDraft(ctx, aliceSession, key); err != nil || !found || got.Title != "Alice draft" {
		t.Fatalf("Alice draft = %+v found=%v error=%v", got, found, err)
	}
	if err := a.SwitchAccount(ctx, "bob"); err != nil {
		t.Fatal(err)
	}
	bobSession := a.SessionInfo()
	if err := a.SaveDraft(ctx, aliceSession, key, local.Draft{Title: "late Alice write"}); !errors.Is(err, ErrStaleSession) {
		t.Fatalf("old-session draft write = %v, want stale", err)
	}
	if _, _, err := a.LoadDraft(ctx, aliceSession, key); !errors.Is(err, ErrStaleSession) {
		t.Fatalf("old-session draft read = %v, want stale", err)
	}
	if _, found, err := a.LoadDraft(ctx, bobSession, key); err != nil || found {
		t.Fatalf("Alice draft crossed to Bob: found=%v error=%v", found, err)
	}
	bobDraft := local.Draft{Title: "Bob draft", Content: "other"}
	if err := a.SaveDraft(ctx, bobSession, key, bobDraft); err != nil {
		t.Fatal(err)
	}
	if err := a.SwitchAccount(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	aliceSession = a.SessionInfo()
	if got, found, err := a.LoadDraft(ctx, aliceSession, key); err != nil || !found || got.Title != "Alice draft" {
		t.Fatalf("Alice draft after return = %+v found=%v error=%v", got, found, err)
	}
	if err := a.DeleteDraft(ctx, aliceSession, key); err != nil {
		t.Fatal(err)
	}
	if _, found, err := a.LoadDraft(ctx, aliceSession, key); err != nil || found {
		t.Fatalf("deleted Alice draft = found:%v error=%v", found, err)
	}
	if _, _, err := a.LoadDraft(ctx, aliceSession, key); err != nil {
		t.Fatalf("old session after switch should have been refreshed in the test: %v", err)
	}
}
