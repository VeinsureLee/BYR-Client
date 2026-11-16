package platform

import (
	"context"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/application"
)

func TestFileCredentialsClearIsIdempotent(t *testing.T) {
	store := FileCredentials{Path: t.TempDir() + "/session.json"}
	ctx := context.Background()
	if err := store.Save(ctx, application.Credential{Username: "alice", Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(ctx); err != nil {
		t.Fatalf("second clear failed: %v", err)
	}
	got, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != (application.Credential{}) {
		t.Fatalf("credential remains after clear: %+v", got)
	}
}
