// Package platform adapts desktop OS facilities; the shared core imports none
// of these MyGo or filesystem APIs.
package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/egoist/mygo"
)

type FileState struct{ Path string }

func (f FileState) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(f.Path)
	if err == nil && len(b) > 0 && json.Valid(b) {
		return b, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// A direct write can be interrupted by shutdown. Keep the previous valid
	// state in a sidecar and recover it rather than starting from scratch.
	if backup, backupErr := os.ReadFile(f.Path + ".bak"); backupErr == nil && len(backup) > 0 && json.Valid(backup) {
		return backup, nil
	}
	if err == nil && len(b) > 0 {
		return nil, errors.New("state file is invalid")
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return b, err
}
func (f FileState) Save(ctx context.Context, b []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0700); err != nil {
		return err
	}
	if preferDirectStateWrite() {
		// Windows security products may deny creating the hidden temporary
		// file used by the atomic path. Update the established files directly,
		// keeping a valid previous copy for crash recovery.
		return f.saveDirectWithBackup(b)
	}
	var last error
	for attempt := 0; attempt < 6; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		last = f.saveOnce(b)
		if last == nil {
			return nil
		}
		// Windows antivirus/indexers can briefly open the freshly written
		// temporary file or the destination. Back off and retry instead of
		// surfacing a spurious "Access is denied" for reading history.
		if !retryableFileError(last) {
			return last
		}
		time.Sleep(time.Duration(20*(attempt+1)) * time.Millisecond)
	}
	if retryableFileError(last) {
		// Some security suites deny creating hidden temporary files but allow
		// updating the established state file. Prefer keeping preferences and
		// history working over failing every save; this fallback is not atomic,
		// so it is used only after the atomic path was denied repeatedly.
		if err := f.saveDirectWithBackup(b); err == nil {
			return nil
		} else {
			return err
		}
	}
	return last
}

func (f FileState) saveDirectWithBackup(b []byte) error {
	old, err := os.ReadFile(f.Path)
	if err == nil && len(old) > 0 && json.Valid(old) {
		if err := writeDirect(f.Path+".bak", old); err != nil {
			return fmt.Errorf("backup state: %w", err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read previous state: %w", err)
	}
	return writeDirect(f.Path, b)
}

func writeDirect(path string, b []byte) error {
	t, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err = t.Write(b); err == nil {
		err = t.Sync()
	}
	closeErr := t.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func (f FileState) saveOnce(b []byte) error {
	t, err := os.CreateTemp(filepath.Dir(f.Path), ".state-*")
	if err != nil {
		return err
	}
	name := t.Name()
	defer os.Remove(name)
	if err = t.Chmod(0600); err == nil {
		_, err = t.Write(b)
	}
	if err == nil {
		err = t.Sync()
	}
	closeErr := t.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, f.Path)
}

type credentialSecretStore interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Delete(string) error
}

type Credentials struct{ store credentialSecretStore }

func (s Credentials) secretStore() credentialSecretStore {
	if s.store != nil {
		return s.store
	}
	return mygo.Secrets
}

type desktopCredentialDocument struct {
	Version  int                               `json:"version"`
	Active   string                            `json:"active,omitempty"`
	Accounts map[string]application.Credential `json:"accounts"`
}

const desktopCredentialVersion = 1

var desktopCredentialMu sync.Mutex

func desktopCredentialKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func (s Credentials) loadDocument(ctx context.Context) (desktopCredentialDocument, bool, error) {
	doc := desktopCredentialDocument{Version: desktopCredentialVersion, Accounts: map[string]application.Credential{}}
	if err := ctx.Err(); err != nil {
		return doc, false, err
	}
	b, err := s.secretStore().Get("forum-accounts")
	if errors.Is(err, mygo.ErrSecretNotFound) {
		legacy, legacyErr := s.secretStore().Get("forum-session")
		if errors.Is(legacyErr, mygo.ErrSecretNotFound) {
			return doc, false, nil
		}
		if legacyErr != nil {
			return doc, false, legacyErr
		}
		var credential application.Credential
		if err := json.Unmarshal(legacy, &credential); err != nil {
			return doc, false, err
		}
		key := desktopCredentialKey(credential.Username)
		if key == "" || credential.Token == "" {
			return doc, false, errors.New("旧版账号凭据无效")
		}
		doc.Active = key
		doc.Accounts[key] = credential
		return doc, true, nil
	}
	if err != nil {
		return doc, false, err
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return doc, false, err
	}
	if doc.Version != desktopCredentialVersion {
		return doc, false, errors.New("不支持的账号凭据版本")
	}
	if doc.Accounts == nil {
		doc.Accounts = map[string]application.Credential{}
	}
	return doc, false, nil
}

func (s Credentials) saveDocument(ctx context.Context, doc desktopCredentialDocument) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	doc.Version = desktopCredentialVersion
	if doc.Accounts == nil {
		doc.Accounts = map[string]application.Credential{}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return s.secretStore().Set("forum-accounts", b)
}

func (s Credentials) migrate(ctx context.Context, doc desktopCredentialDocument, legacy bool) error {
	if !legacy {
		return nil
	}
	if err := s.saveDocument(ctx, doc); err != nil {
		return err
	}
	_ = s.secretStore().Delete("forum-session")
	return nil
}

func (s Credentials) Load(ctx context.Context) (application.Credential, error) {
	desktopCredentialMu.Lock()
	defer desktopCredentialMu.Unlock()
	doc, legacy, err := s.loadDocument(ctx)
	if err != nil {
		return application.Credential{}, err
	}
	if err := s.migrate(ctx, doc, legacy); err != nil {
		return application.Credential{}, err
	}
	return doc.Accounts[desktopCredentialKey(doc.Active)], nil
}

func (s Credentials) Save(ctx context.Context, credential application.Credential) error {
	desktopCredentialMu.Lock()
	defer desktopCredentialMu.Unlock()
	key := desktopCredentialKey(credential.Username)
	if key == "" || credential.Token == "" {
		return errors.New("账号凭据无效")
	}
	doc, _, err := s.loadDocument(ctx)
	if err != nil {
		return err
	}
	doc.Accounts[key] = credential
	doc.Active = key
	return s.saveDocument(ctx, doc)
}

func (s Credentials) Clear(ctx context.Context) error { return s.SetActive(ctx, "") }

func (s Credentials) Accounts(ctx context.Context) ([]application.Credential, error) {
	desktopCredentialMu.Lock()
	defer desktopCredentialMu.Unlock()
	doc, legacy, err := s.loadDocument(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.migrate(ctx, doc, legacy); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(doc.Accounts))
	for key := range doc.Accounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]application.Credential, 0, len(keys))
	for _, key := range keys {
		out = append(out, doc.Accounts[key])
	}
	return out, nil
}

func (s Credentials) LoadAccount(ctx context.Context, username string) (application.Credential, error) {
	desktopCredentialMu.Lock()
	defer desktopCredentialMu.Unlock()
	doc, legacy, err := s.loadDocument(ctx)
	if err != nil {
		return application.Credential{}, err
	}
	if err := s.migrate(ctx, doc, legacy); err != nil {
		return application.Credential{}, err
	}
	credential, ok := doc.Accounts[desktopCredentialKey(username)]
	if !ok {
		return application.Credential{}, application.ErrSavedAccountNotFound
	}
	return credential, nil
}

func (s Credentials) SetActive(ctx context.Context, username string) error {
	desktopCredentialMu.Lock()
	defer desktopCredentialMu.Unlock()
	doc, _, err := s.loadDocument(ctx)
	if err != nil {
		return err
	}
	key := desktopCredentialKey(username)
	if key != "" {
		if _, ok := doc.Accounts[key]; !ok {
			return application.ErrSavedAccountNotFound
		}
	}
	doc.Active = key
	return s.saveDocument(ctx, doc)
}

func (s Credentials) DeleteAccount(ctx context.Context, username string) error {
	desktopCredentialMu.Lock()
	defer desktopCredentialMu.Unlock()
	doc, _, err := s.loadDocument(ctx)
	if err != nil {
		return err
	}
	key := desktopCredentialKey(username)
	if _, ok := doc.Accounts[key]; !ok {
		return application.ErrSavedAccountNotFound
	}
	delete(doc.Accounts, key)
	if doc.Active == key {
		doc.Active = ""
	}
	return s.saveDocument(ctx, doc)
}
func DataDir() (string, error) {
	if p := os.Getenv("BYR_DATA_DIR"); p != "" {
		return filepath.Abs(p)
	}
	p, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	preferred := filepath.Join(p, "BYR-Client")
	if dirWritable(preferred) {
		return preferred, nil
	}
	// Some sandboxed or restricted launch contexts grant read-only access to
	// %APPDATA%. Fall back to a writable directory next to the executable,
	// then to the user temp directory, carrying the existing state over once.
	if exe, err := os.Executable(); err == nil {
		fallback := filepath.Join(filepath.Dir(exe), "data")
		if dirWritable(fallback) {
			migrateState(preferred, fallback)
			return fallback, nil
		}
	}
	fallback := filepath.Join(os.TempDir(), "BYR-Client")
	if dirWritable(fallback) {
		migrateState(preferred, fallback)
		return fallback, nil
	}
	return preferred, nil
}

func dirWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false
	}
	probe, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	closeErr := probe.Close()
	_ = os.Remove(name)
	return closeErr == nil
}

func migrateState(from, to string) {
	for _, name := range []string{"state.json", "state.json.bak"} {
		target := filepath.Join(to, name)
		if _, err := os.Stat(target); err == nil {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(from, name)); err == nil && len(b) > 0 {
			_ = os.WriteFile(target, b, 0600)
		}
	}
}
