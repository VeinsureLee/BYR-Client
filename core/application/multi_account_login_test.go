package application

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestRememberedLoginAddsAccountWithoutReplacingOthers(t *testing.T) {
	ctx := context.Background()
	store := &accountCredentials{accounts: map[string]Credential{
		"alice": {Username: "alice", Token: "alice-token"},
	}, active: "alice"}
	handler := func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"access_token":"bob-token","expires_in":3600}`)
	}
	a := appFor(t, handler, nil, store)
	if err := a.Login(ctx, "bob", "pw", true); err != nil {
		t.Fatal(err)
	}
	if len(store.accounts) != 2 || store.accounts["alice"].Token != "alice-token" || store.accounts["bob"].Token != "bob-token" {
		t.Fatalf("remembered login replaced saved accounts: %+v", store.accounts)
	}
	if store.active != "bob" {
		t.Fatalf("active account = %q, want bob", store.active)
	}
	restarted := appFor(t, handler, nil, store)
	if got := restarted.sessionSnapshot(); got.Username != "bob" || got.Token != "bob-token" {
		t.Fatalf("restored active account = %+v", got)
	}
}
