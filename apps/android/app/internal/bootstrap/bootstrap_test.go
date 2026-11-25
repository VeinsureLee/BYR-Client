package bootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/application"
)

type testCredentials struct{ value application.Credential }

func (s *testCredentials) Load(context.Context) (application.Credential, error) { return s.value, nil }
func (s *testCredentials) Save(_ context.Context, value application.Credential) error {
	s.value = value
	return nil
}
func (s *testCredentials) Clear(context.Context) error {
	s.value = application.Credential{}
	return nil
}

func TestBootstrapOpensV1StateAndRestoresSessionWithoutNetwork(t *testing.T) {
	dir := t.TempDir()
	state := []byte(`{"version":1,"preferences":{"theme":2,"fontScale":1,"ipv6":false},"accounts":{"alice":{"drafts":{},"history":[{"Board":"Go","ID":"42","Title":"最近主题"}]}}}`)
	if err := os.WriteFile(filepath.Join(dir, "state.json"), state, 0600); err != nil {
		t.Fatal(err)
	}
	credentials := &testCredentials{value: application.Credential{Username: "alice", Token: "fixture-token"}}
	services, err := New(context.Background(), Config{BaseURL: "http://127.0.0.1", DataDir: dir, Credentials: credentials})
	if err != nil {
		t.Fatal(err)
	}
	if services.App == nil || services.DataDir != dir {
		t.Fatalf("services = %+v", services)
	}
	if info := services.App.SessionInfo(); !info.Authenticated || info.Username != "alice" {
		t.Fatalf("restored session = %+v", info)
	}
	if prefs := services.App.Preferences(); prefs.Theme != 2 || prefs.ColorScheme != "byr-blue" {
		t.Fatalf("restored v1 preferences = %+v", prefs)
	}
	if entries := services.App.RecentHistory(); len(entries) != 1 || entries[0].ID != "42" {
		t.Fatalf("restored v1 history = %+v", entries)
	}
}

func TestBootstrapReturnsRecoverableStateError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{bad`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := New(context.Background(), Config{BaseURL: "http://127.0.0.1", DataDir: dir, Credentials: &testCredentials{}})
	if err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("bootstrap state error = %v", err)
	}
}
