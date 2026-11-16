// Package platform adapts Android app-private storage and clipboard access.
// The shared core imports none of these Gio or filesystem APIs.
package platform

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

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

func (f FileCredentials) Load(ctx context.Context) (application.Credential, error) {
	var c application.Credential
	if err := ctx.Err(); err != nil {
		return c, err
	}
	b, err := os.ReadFile(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if len(b) == 0 {
		return c, nil
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

func (f FileCredentials) Save(ctx context.Context, c application.Credential) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, err := json.Marshal(c)
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

func (f FileCredentials) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := os.Remove(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
