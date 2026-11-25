package desktop

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/local"
)

type desktopHistoryBackend struct {
	mu   sync.Mutex
	data []byte
	fail bool
}

func (b *desktopHistoryBackend) Load(context.Context) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.data...), nil
}

func (b *desktopHistoryBackend) Save(_ context.Context, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		return errors.New("disk full")
	}
	b.data = append([]byte(nil), data...)
	return nil
}

func (b *desktopHistoryBackend) setFailure(fail bool) {
	b.mu.Lock()
	b.fail = fail
	b.mu.Unlock()
}

func seedDesktopHistory(t *testing.T, backend *desktopHistoryBackend, account, id string) {
	t.Helper()
	store, err := local.Open(context.Background(), backend)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Visit(context.Background(), account, local.History{Board: "Go", ID: id, Title: id}); err != nil {
		t.Fatal(err)
	}
}

func openDesktopHistory(t *testing.T, backend *desktopHistoryBackend, account string) []local.History {
	t.Helper()
	store, err := local.Open(context.Background(), backend)
	if err != nil {
		t.Fatal(err)
	}
	return store.History(account)
}

func TestDesktopHistoryClearRequiresConfirmationInSettingsAndHistory(t *testing.T) {
	backend := &desktopHistoryBackend{}
	seedDesktopHistory(t, backend, "alice", "alice-old")
	seedDesktopHistory(t, backend, "bob", "bob-old")
	h := newHarnessWithStorage(t, backend)
	h.login()

	h.goTo(Route{Page: "settings"})
	h.click("清空最近阅读")
	h.has("只清空当前账号 alice 的本机最近阅读记录")
	if got := h.app.Core.RecentHistory(); len(got) != 1 || got[0].ID != "alice-old" {
		t.Fatalf("settings clear acted before confirmation: %+v", got)
	}
	h.click("取消")
	if h.app.confirm != nil || len(h.app.Core.RecentHistory()) != 1 {
		t.Fatalf("settings cancellation changed history: dialog=%v history=%+v", h.app.confirm != nil, h.app.Core.RecentHistory())
	}
	h.click("清空最近阅读")
	h.click("确认")
	if got := h.app.Core.RecentHistory(); len(got) != 0 {
		t.Fatalf("settings clear left Alice history: %+v", got)
	}
	if got := openDesktopHistory(t, backend, "bob"); len(got) != 1 || got[0].ID != "bob-old" {
		t.Fatalf("settings clear changed Bob history: %+v", got)
	}

	if err := h.app.Core.RecordThreadVisit(context.Background(), h.app.Core.HistoryReadToken(), "Go", "history-page", "History page"); err != nil {
		t.Fatal(err)
	}
	h.goTo(Route{Page: "history"})
	h.click("清空阅读历史")
	h.has("只清空当前账号 alice 的本机最近阅读记录")
	h.click("确认")
	if got := h.app.Core.RecentHistory(); len(got) != 0 {
		t.Fatalf("history-page clear left Alice history: %+v", got)
	}
}

func TestDesktopHistoryClearFailureKeepsListAndRetries(t *testing.T) {
	backend := &desktopHistoryBackend{}
	seedDesktopHistory(t, backend, "alice", "42")
	backend.setFailure(true)
	h := newHarnessWithStorage(t, backend)
	h.login()
	h.goTo(Route{Page: "history"})
	h.click("清空阅读历史")
	h.click("确认")
	if got := h.app.Core.RecentHistory(); len(got) != 1 || got[0].ID != "42" {
		t.Fatalf("failed clear discarded history: %+v", got)
	}
	if !h.app.failed || h.app.retryLoad == nil {
		t.Fatalf("failed clear has no visible retry path: failed=%v retry=%v message=%q", h.app.failed, h.app.retryLoad != nil, h.app.message)
	}
	backend.setFailure(false)
	h.click("重试")
	if got := h.app.Core.RecentHistory(); len(got) != 0 {
		t.Fatalf("history after retry = %+v", got)
	}
	if h.app.failed || h.app.retryLoad != nil {
		t.Fatalf("successful retry retained error state: failed=%v retry=%v message=%q", h.app.failed, h.app.retryLoad != nil, h.app.message)
	}
}
