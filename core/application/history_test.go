package application

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestClearHistoryIsCurrentAccountScopedAndInvalidatesQueuedVisits(t *testing.T) {
	ctx := context.Background()
	store, err := local.Open(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	creds := &accountsCredentialStore{accounts: []Credential{
		{Username: "alice", Token: "alice-token"},
		{Username: "bob", Token: "bob-token"},
	}, active: "alice"}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) }, store, creds)

	if err := store.Visit(ctx, "alice", local.History{Board: "Go", ID: "alice-old", Title: "Alice old"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Visit(ctx, "bob", local.History{Board: "Go", ID: "bob-old", Title: "Bob old"}); err != nil {
		t.Fatal(err)
	}
	queued := a.HistoryReadToken()
	if err := a.ClearHistory(ctx, queued.Generation); err != nil {
		t.Fatal(err)
	}
	if got := store.History("alice"); len(got) != 0 {
		t.Fatalf("Alice history after clear = %+v, want empty", got)
	}
	if got := store.History("bob"); len(got) != 1 || got[0].ID != "bob-old" {
		t.Fatalf("Bob history after Alice clear = %+v, want unchanged", got)
	}
	// This represents a thread request queued before the confirmed clear. It may
	// finish afterward, but its old epoch is discarded instead of repopulating history.
	if err := a.RecordThreadVisit(ctx, queued, "Go", "late", "Late visit"); err != nil {
		t.Fatalf("stale pre-clear visit should be dropped quietly: %v", err)
	}
	if got := store.History("alice"); len(got) != 0 {
		t.Fatalf("pre-clear visit repopulated Alice history: %+v", got)
	}
	current := a.HistoryReadToken()
	if err := a.RecordThreadVisit(ctx, current, "Go", "new", "New visit"); err != nil {
		t.Fatal(err)
	}
	if got := store.History("alice"); len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("post-clear visit = %+v, want only the new record", got)
	}
}

func TestClearHistoryRejectsOldAccountGeneration(t *testing.T) {
	ctx := context.Background()
	store, err := local.Open(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Visit(ctx, "alice", local.History{Board: "Go", ID: "a", Title: "Alice"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Visit(ctx, "bob", local.History{Board: "Go", ID: "b", Title: "Bob"}); err != nil {
		t.Fatal(err)
	}
	creds := &accountsCredentialStore{accounts: []Credential{
		{Username: "alice", Token: "alice-token"},
		{Username: "bob", Token: "bob-token"},
	}, active: "alice"}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) }, store, creds)
	aliceGeneration := a.SessionInfo().Generation
	if err := a.SwitchAccount(ctx, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := a.ClearHistory(ctx, aliceGeneration); !errors.Is(err, ErrStaleSession) {
		t.Fatalf("clear with stale account generation = %v, want ErrStaleSession", err)
	}
	if len(store.History("alice")) != 1 || len(store.History("bob")) != 1 {
		t.Fatalf("stale clear changed histories: Alice=%+v Bob=%+v", store.History("alice"), store.History("bob"))
	}
}

func TestClearHistorySerializesConcurrentPreClearVisit(t *testing.T) {
	ctx := context.Background()
	store, err := local.Open(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) }, store,
		&credentials{value: Credential{Username: "alice", Token: "token"}})
	token := a.HistoryReadToken()
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	var visitErr, clearErr error
	go func() {
		defer wg.Done()
		<-start
		visitErr = a.RecordThreadVisit(ctx, token, "Go", "late", "Late visit")
	}()
	go func() {
		defer wg.Done()
		<-start
		clearErr = a.ClearHistory(ctx, token.Generation)
	}()
	close(start)
	wg.Wait()
	if visitErr != nil || clearErr != nil {
		t.Fatalf("concurrent visit/clear errors = %v/%v", visitErr, clearErr)
	}
	if got := store.History("alice"); len(got) != 0 {
		t.Fatalf("history after concurrent clear = %+v, want empty", got)
	}
	fresh := a.HistoryReadToken()
	if err := a.RecordThreadVisit(ctx, fresh, "Go", "after", "After clear"); err != nil {
		t.Fatal(err)
	}
	if got := store.History("alice"); len(got) != 1 || got[0].ID != "after" {
		t.Fatalf("post-clear history = %+v, want new visit only", got)
	}
}

func TestFailedClearDoesNotAdvanceHistoryEpoch(t *testing.T) {
	ctx := context.Background()
	store, err := local.Open(ctx, failingDisk{})
	if err != nil {
		t.Fatal(err)
	}
	a := appFor(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) }, store,
		&credentials{value: Credential{Username: "alice", Token: "token"}})
	token := a.HistoryReadToken()
	if err := a.ClearHistory(ctx, token.Generation); err == nil {
		t.Fatal("failed disk clear reported success")
	}
	if got := a.historyEpoch[historyAccountKey("alice")]; got != 0 {
		t.Fatalf("history epoch after failed clear = %d, want 0", got)
	}
}
