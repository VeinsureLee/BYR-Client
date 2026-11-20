package platform

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/application"
)

func TestFileCredentialsMigratesLegacyAndKeepsMultipleAccounts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	legacy, err := json.Marshal(application.Credential{Username: "alice", Token: "alice-token"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	store := FileCredentials{Path: path}
	ctx := context.Background()
	if got, err := store.Load(ctx); err != nil || got.Username != "alice" || got.Token != "alice-token" {
		t.Fatalf("legacy restore = %+v, %v", got, err)
	}

	if err := store.Save(ctx, application.Credential{Username: "Bob", Token: "bob-token"}); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(ctx); err != nil || got.Username != "Bob" {
		t.Fatalf("active account after save = %+v, %v", got, err)
	}
	accounts, err := store.Accounts(ctx)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("accounts after second login = %+v, %v", accounts, err)
	}
	if err := store.SetActive(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(ctx); err != nil || got.Token != "alice-token" {
		t.Fatalf("switch active account = %+v, %v", got, err)
	}
	if err := store.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(ctx); err != nil || got.Token != "" {
		t.Fatalf("cleared active selection = %+v, %v", got, err)
	}
	accounts, err = store.Accounts(ctx)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("logout deleted saved accounts: %+v, %v", accounts, err)
	}
	if err := store.DeleteAccount(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	accounts, err = store.Accounts(ctx)
	if err != nil || len(accounts) != 1 || accounts[0].Username != "Bob" {
		t.Fatalf("delete inactive account = %+v, %v", accounts, err)
	}
}
