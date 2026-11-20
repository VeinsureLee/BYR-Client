// Package platform adapts Android app-private storage and clipboard access.
// The shared core imports none of these Gio or filesystem APIs.
package platform

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gioui.org/app"

	"github.com/VeinsureLee/BYR-Client/core/application"
)

// DataDir resolves the app-private directory. On Android Gio maps this to
// Context.getFilesDir(); it must be called after the app backend is ready.
func DataDir() (string, error) {
	if p := os.Getenv("BYR_DATA_DIR"); p != "" {
		return filepath.Abs(p)
	}
	dir, err := app.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "BYR-Client"), nil
}

// FileState is the local.Store backend. The file lives in app-private storage
// and is written atomically with owner-only permissions.
type FileState struct{ Path string }

func (f FileState) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

func (f FileState) Save(ctx context.Context, b []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Dir(f.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	t, err := os.CreateTemp(dir, ".state-*")
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

// FileCredentials keeps the forum token in app-private storage. Unlike the
// desktop credential store this is not hardware-backed; the Android app
// sandbox is the protection boundary. The data file never contains the token.
type FileCredentials struct{ Path string }

type fileCredentialDocument struct {
	Version  int                               `json:"version"`
	Active   string                            `json:"active,omitempty"`
	Accounts map[string]application.Credential `json:"accounts"`
}

const fileCredentialVersion = 1

var fileCredentialMu sync.Mutex

func credentialKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func (f FileCredentials) loadDocument(ctx context.Context) (fileCredentialDocument, bool, error) {
	doc := fileCredentialDocument{Version: fileCredentialVersion, Accounts: map[string]application.Credential{}}
	if err := ctx.Err(); err != nil {
		return doc, false, err
	}
	b, err := os.ReadFile(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return doc, false, nil
	}
	if err != nil {
		return doc, false, err
	}
	if len(b) == 0 {
		return doc, false, nil
	}
	doc = fileCredentialDocument{}
	if err := json.Unmarshal(b, &doc); err == nil && doc.Version == fileCredentialVersion {
		if doc.Accounts == nil {
			doc.Accounts = map[string]application.Credential{}
		}
		return doc, false, nil
	}
	var legacy application.Credential
	if err := json.Unmarshal(b, &legacy); err != nil {
		return doc, false, err
	}
	if legacy.Username == "" && legacy.Token == "" {
		return doc, false, errors.New("不支持的账号凭据版本")
	}
	key := credentialKey(legacy.Username)
	if key == "" || legacy.Token == "" {
		return doc, false, errors.New("旧版账号凭据无效")
	}
	doc.Version = fileCredentialVersion
	doc.Active = key
	doc.Accounts = map[string]application.Credential{key: legacy}
	return doc, true, nil
}

func (f FileCredentials) saveDocument(ctx context.Context, doc fileCredentialDocument) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	doc.Version = fileCredentialVersion
	if doc.Accounts == nil {
		doc.Accounts = map[string]application.Credential{}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	dir := filepath.Dir(f.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	t, err := os.CreateTemp(dir, ".session-*")
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

func (f FileCredentials) Load(ctx context.Context) (application.Credential, error) {
	fileCredentialMu.Lock()
	defer fileCredentialMu.Unlock()
	doc, legacy, err := f.loadDocument(ctx)
	if err != nil {
		return application.Credential{}, err
	}
	if legacy {
		if err := f.saveDocument(ctx, doc); err != nil {
			return application.Credential{}, err
		}
	}
	return doc.Accounts[credentialKey(doc.Active)], nil
}

func (f FileCredentials) Save(ctx context.Context, c application.Credential) error {
	fileCredentialMu.Lock()
	defer fileCredentialMu.Unlock()
	key := credentialKey(c.Username)
	if key == "" || c.Token == "" {
		return errors.New("账号凭据无效")
	}
	doc, _, err := f.loadDocument(ctx)
	if err != nil {
		return err
	}
	doc.Accounts[key] = c
	doc.Active = key
	return f.saveDocument(ctx, doc)
}

func (f FileCredentials) Clear(ctx context.Context) error {
	return f.SetActive(ctx, "")
}

func (f FileCredentials) Accounts(ctx context.Context) ([]application.Credential, error) {
	fileCredentialMu.Lock()
	defer fileCredentialMu.Unlock()
	doc, legacy, err := f.loadDocument(ctx)
	if err != nil {
		return nil, err
	}
	if legacy {
		if err := f.saveDocument(ctx, doc); err != nil {
			return nil, err
		}
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

func (f FileCredentials) LoadAccount(ctx context.Context, username string) (application.Credential, error) {
	fileCredentialMu.Lock()
	defer fileCredentialMu.Unlock()
	doc, legacy, err := f.loadDocument(ctx)
	if err != nil {
		return application.Credential{}, err
	}
	if legacy {
		if err := f.saveDocument(ctx, doc); err != nil {
			return application.Credential{}, err
		}
	}
	credential, ok := doc.Accounts[credentialKey(username)]
	if !ok {
		return application.Credential{}, application.ErrSavedAccountNotFound
	}
	return credential, nil
}

func (f FileCredentials) SetActive(ctx context.Context, username string) error {
	fileCredentialMu.Lock()
	defer fileCredentialMu.Unlock()
	doc, _, err := f.loadDocument(ctx)
	if err != nil {
		return err
	}
	key := credentialKey(username)
	if key != "" {
		if _, ok := doc.Accounts[key]; !ok {
			return application.ErrSavedAccountNotFound
		}
	}
	doc.Active = key
	return f.saveDocument(ctx, doc)
}

func (f FileCredentials) DeleteAccount(ctx context.Context, username string) error {
	fileCredentialMu.Lock()
	defer fileCredentialMu.Unlock()
	doc, _, err := f.loadDocument(ctx)
	if err != nil {
		return err
	}
	key := credentialKey(username)
	if _, ok := doc.Accounts[key]; !ok {
		return application.ErrSavedAccountNotFound
	}
	delete(doc.Accounts, key)
	if doc.Active == key {
		doc.Active = ""
	}
	if len(doc.Accounts) == 0 {
		err := os.Remove(f.Path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return f.saveDocument(ctx, doc)
}

// SaveImage writes into this app's external Pictures directory. Android's
// scoped-storage rules allow access to an app-specific external directory
// without broad storage permissions; the returned path is shown to the user.
func SaveImage(ctx context.Context, name string, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("图片内容为空")
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "", errors.New("图片文件名无效")
	}
	dir := os.Getenv("BYR_IMAGE_EXPORT_DIR")
	if dir == "" {
		external := os.Getenv("EXTERNAL_STORAGE")
		if external == "" {
			external = "/storage/emulated/0"
		}
		dir = filepath.Join(external, "Android", "data", "cn.byr.bbs.byrclient", "files", "Pictures", "BYR-Client")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, ".image-*")
	if err != nil {
		return "", err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err = file.Chmod(0644); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	path := filepath.Join(dir, name)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.Rename(temp, path); err != nil {
		return "", err
	}
	return path, nil
}
