package application

import (
	"context"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestRecentHistoryFollowsActiveAccountWithoutExposingCredential(t *testing.T) {
	ctx := context.Background()
	store, err := local.Open(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct{ account, id string }{{"alice", "alice-1"}, {"bob", "bob-1"}} {
		if err := store.Visit(ctx, entry.account, local.History{Board: "Go", ID: entry.id, Title: entry.id}); err != nil {
			t.Fatal(err)
		}
	}
	creds := &accountsCredentialStore{
		accounts: []Credential{{Username: "alice", Token: "alice-token"}, {Username: "bob", Token: "bob-token"}},
		active:   "alice",
	}
	a := appFor(t, nil, store, creds)
	if got := a.RecentHistory(); len(got) != 1 || got[0].ID != "alice-1" {
		t.Fatalf("active Alice history = %+v", got)
	}
	if err := a.SwitchAccount(ctx, "bob"); err != nil {
		t.Fatal(err)
	}
	if got := a.RecentHistory(); len(got) != 1 || got[0].ID != "bob-1" {
		t.Fatalf("active Bob history = %+v", got)
	}
}
