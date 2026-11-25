package ui

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

type toggleHistoryBackend struct {
	mu   sync.Mutex
	data []byte
	fail bool
}

func (b *toggleHistoryBackend) Load(context.Context) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.data...), nil
}

func (b *toggleHistoryBackend) Save(_ context.Context, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		return errors.New("disk full")
	}
	b.data = append([]byte(nil), data...)
	return nil
}

func (b *toggleHistoryBackend) setFailure(fail bool) {
	b.mu.Lock()
	b.fail = fail
	b.mu.Unlock()
}

func seedMobileHistory(t *testing.T, backend local.Backend, account, id string) {
	t.Helper()
	store, err := local.Open(context.Background(), backend)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Visit(context.Background(), account, local.History{Board: "Go", ID: id, Title: id}); err != nil {
		t.Fatal(err)
	}
}

func storedMobileHistory(t *testing.T, backend local.Backend, account string) []local.History {
	t.Helper()
	store, err := local.Open(context.Background(), backend)
	if err != nil {
		t.Fatal(err)
	}
	return store.History(account)
}

func newHistoryPageApp(t *testing.T, baseURL string, backend local.Backend) *App {
	t.Helper()
	credentials := &testCredentials{session: application.Credential{
		Username: "alice", Token: "fixture-token", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}}
	shared := newUIFixtureCore(t, baseURL, http.DefaultClient, backend, credentials)
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.ready = true
	a.core = shared
	a.stack = []Route{{Page: PageHistory}}
	t.Cleanup(a.Close)
	return a
}

func TestHistoryClearRequiresConfirmationAndOnlyClearsActiveAccount(t *testing.T) {
	server := newInteractionServer(t)
	backend := &toggleHistoryBackend{}
	seedMobileHistory(t, backend, "alice", "alice-old")
	seedMobileHistory(t, backend, "bob", "bob-old")
	a := newHistoryPageApp(t, server.server.URL, backend)
	ctx := context.Background()

	renderAppAt(t, a, 420, 700, 1)
	a.click("history-clear").Click()
	renderAppAt(t, a, 420, 700, 1)
	if a.confirmation == nil || len(a.core.RecentHistory()) != 1 {
		t.Fatalf("clear acted before confirmation: dialog=%v history=%+v", a.confirmation != nil, a.core.RecentHistory())
	}
	a.click("confirmation-cancel").Click()
	renderAppAt(t, a, 420, 700, 1)
	if a.confirmation != nil || len(a.core.RecentHistory()) != 1 {
		t.Fatalf("cancel changed history: dialog=%v history=%+v", a.confirmation != nil, a.core.RecentHistory())
	}

	a.click("history-clear").Click()
	renderAppAt(t, a, 420, 700, 1)
	a.click("confirmation-accept").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	if got := a.core.RecentHistory(); len(got) != 0 {
		t.Fatalf("Alice history after confirmed clear = %+v", got)
	}
	if got := storedMobileHistory(t, backend, "bob"); len(got) != 1 || got[0].ID != "bob-old" {
		t.Fatalf("Bob history after Alice clear = %+v", got)
	}

	if err := a.core.RecordThreadVisit(ctx, a.core.HistoryReadToken(), "Go", "settings", "Settings clear"); err != nil {
		t.Fatal(err)
	}
	a.stack = []Route{{Page: PageSettings}}
	renderAppAt(t, a, 420, 700, 1)
	a.auxList.Position.First = 2
	renderAppAt(t, a, 420, 700, 1)
	a.click("settings-clear-history").Click()
	renderAppAt(t, a, 420, 700, 1)
	if a.confirmation == nil || len(a.core.RecentHistory()) != 1 {
		t.Fatalf("settings clear acted before confirmation: dialog=%v history=%+v", a.confirmation != nil, a.core.RecentHistory())
	}
	a.click("confirmation-accept").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	if got := a.core.RecentHistory(); len(got) != 0 {
		t.Fatalf("settings clear left Alice history: %+v", got)
	}
}

func TestHistoryClearFailureRetainsEntriesAndCanRetry(t *testing.T) {
	server := newInteractionServer(t)
	backend := &toggleHistoryBackend{}
	seedMobileHistory(t, backend, "alice", "42")
	backend.setFailure(true)
	a := newHistoryPageApp(t, server.server.URL, backend)

	renderAppAt(t, a, 420, 700, 1)
	a.click("history-clear").Click()
	renderAppAt(t, a, 420, 700, 1)
	a.click("confirmation-accept").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	if got := a.core.RecentHistory(); len(got) != 1 || got[0].ID != "42" {
		t.Fatalf("failed clear discarded history: %+v", got)
	}
	if !a.failed || a.retryAction == nil {
		t.Fatalf("failed clear lacks an error/retry action: failed=%v retry=%v message=%q", a.failed, a.retryAction != nil, a.message)
	}

	backend.setFailure(false)
	a.click("retry-status").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	if got := a.core.RecentHistory(); len(got) != 0 {
		t.Fatalf("history after successful retry = %+v", got)
	}
	if a.failed || a.retryAction != nil {
		t.Fatalf("successful retry retained error state: failed=%v retry=%v message=%q", a.failed, a.retryAction != nil, a.message)
	}
}
