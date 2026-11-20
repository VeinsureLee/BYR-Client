package platform

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/egoist/mygo"
)

type memorySecrets map[string][]byte

func (s memorySecrets) Get(key string) ([]byte, error) {
	value, ok := s[key]
	if !ok {
		return nil, mygo.ErrSecretNotFound
	}
	return append([]byte(nil), value...), nil
}

func (s memorySecrets) Set(key string, value []byte) error {
	s[key] = append([]byte(nil), value...)
	return nil
}

func (s memorySecrets) Delete(key string) error {
	if _, ok := s[key]; !ok {
		return mygo.ErrSecretNotFound
	}
	delete(s, key)
	return nil
}

func TestDesktopCredentialsMigrateLegacyAndManageMultipleAccounts(t *testing.T) {
	secrets := memorySecrets{}
	legacy, err := json.Marshal(application.Credential{Username: "alice", Token: "alice-token"})
	if err != nil {
		t.Fatal(err)
	}
	secrets["forum-session"] = legacy
	store := Credentials{store: secrets}
	ctx := context.Background()

	if got, err := store.Load(ctx); err != nil || got.Username != "alice" || got.Token != "alice-token" {
		t.Fatalf("legacy credential = %+v, %v", got, err)
	}
	if _, ok := secrets["forum-session"]; ok {
		t.Fatal("legacy secret was not retired after migration")
	}
	if _, ok := secrets["forum-accounts"]; !ok {
		t.Fatal("migrated credential document missing from secret store")
	}
	if err := store.Save(ctx, application.Credential{Username: "Bob", Token: "bob-token"}); err != nil {
		t.Fatal(err)
	}
	accounts, err := store.Accounts(ctx)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("accounts after second login = %+v, %v", accounts, err)
	}
	if err := store.SetActive(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(ctx); err != nil || got.Token != "alice-token" {
		t.Fatalf("selected account = %+v, %v", got, err)
	}
	if err := store.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(ctx); err != nil || got.Token != "" {
		t.Fatalf("cleared active session = %+v, %v", got, err)
	}
	accounts, err = store.Accounts(ctx)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("logout removed saved accounts = %+v, %v", accounts, err)
	}
	if err := store.DeleteAccount(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	accounts, err = store.Accounts(ctx)
	if err != nil || len(accounts) != 1 || accounts[0].Username != "Bob" {
		t.Fatalf("delete account result = %+v, %v", accounts, err)
	}
}

func TestDesktopCredentialStorageErrorDoesNotMutateSessionStore(t *testing.T) {
	store := Credentials{store: failingSecrets{}}
	if err := store.Save(context.Background(), application.Credential{Username: "alice", Token: "token"}); err == nil {
		t.Fatal("save succeeded with failing keychain")
	}
}

type failingSecrets struct{}

func (failingSecrets) Get(string) ([]byte, error) { return nil, errors.New("keychain locked") }
func (failingSecrets) Set(string, []byte) error   { return errors.New("keychain locked") }
func (failingSecrets) Delete(string) error        { return errors.New("keychain locked") }
