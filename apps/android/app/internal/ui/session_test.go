package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/apps/android/app/internal/platform"
	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestCurrentRouteRepairsEmptyStack(t *testing.T) {
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.stack = nil
	if got := a.current().Page; got != PageLogin {
		t.Fatalf("current page = %v, want login", got)
	}
	if len(a.stack) != 1 {
		t.Fatalf("route stack was not repaired: %+v", a.stack)
	}
}

func TestMissingCredentialsStartAtLogin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BYR_DATA_DIR", dir)
	a := startAppForTest(t)
	defer a.Close()
	if !a.ready || a.initErr != "" {
		t.Fatalf("app did not become ready: ready=%v initErr=%q", a.ready, a.initErr)
	}
	if got := a.current().Page; got != PageLogin {
		t.Fatalf("current page = %v, want login", got)
	}
}

func TestExpiredCredentialsStartAtLogin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BYR_DATA_DIR", dir)
	credentials := platform.FileCredentials{Path: filepath.Join(dir, "session.json")}
	if err := credentials.Save(context.Background(), application.Credential{
		Username:  "alice",
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	a := startAppForTest(t)
	defer a.Close()
	if got := a.current().Page; got != PageLogin {
		t.Fatalf("current page = %v, want login", got)
	}
	if _, err := os.Stat(credentials.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired credential file still exists: %v", err)
	}
}

func TestLogoutThenNewAppStartsAtLogin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BYR_DATA_DIR", dir)
	credentials := platform.FileCredentials{Path: filepath.Join(dir, "session.json")}
	if err := credentials.Save(context.Background(), application.Credential{
		Username:  "alice",
		Token:     "remembered-token",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	store, err := local.Open(context.Background(), platform.FileState{Path: filepath.Join(dir, "state.json")})
	if err != nil {
		t.Fatal(err)
	}
	core, err := application.New(application.Options{Local: store, Credentials: credentials})
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if core.Session().Token == "" {
		t.Fatal("fixture session was not restored")
	}

	first, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	first.started = true
	first.ready = true
	first.core = core
	first.dir = dir
	first.stack = []Route{{Page: PageSelf}}
	first.username.SetText("alice")
	first.logout()
	waitForIdle(t, first)
	if got := first.current().Page; got != PageLogin {
		t.Fatalf("logout page = %v, want login", got)
	}
	if core.Session().Token != "" {
		t.Fatal("logout retained the in-memory token")
	}
	if _, err := os.Stat(credentials.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("logout retained credential file: %v", err)
	}
	first.Close()

	second := startAppForTest(t)
	defer second.Close()
	if got := second.current().Page; got != PageLogin {
		t.Fatalf("restarted page = %v, want login", got)
	}
	if !second.ready || second.initErr != "" {
		t.Fatalf("restarted app is not usable: ready=%v initErr=%q", second.ready, second.initErr)
	}
}

func startAppForTest(t *testing.T) *App {
	t.Helper()
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.start()
	a.drain()
	return a
}

func waitForIdle(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		a.drain()
		if !a.busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("app remained busy: message=%q", a.message)
}
