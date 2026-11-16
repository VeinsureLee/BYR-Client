// Package platform adapts desktop OS facilities; the shared core imports none
// of these MyGo or filesystem APIs.
package platform

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return b, err
}
func (f FileState) Save(ctx context.Context, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(f.Path), 0700); err != nil {
		return err
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
		if err := writeDirect(f.Path, b); err == nil {
			return nil
		}
	}
	return last
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

type Credentials struct{}

func (Credentials) Load(ctx context.Context) (application.Credential, error) {
	var c application.Credential
	if err := ctx.Err(); err != nil {
		return c, err
	}
	b, err := mygo.Secrets.Get("forum-session")
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}
func (Credentials) Save(ctx context.Context, c application.Credential) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return mygo.Secrets.Set("forum-session", b)
}
func (Credentials) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return mygo.Secrets.Delete("forum-session")
}
func DataDir() (string, error) {
	if p := os.Getenv("BYR_DATA_DIR"); p != "" {
		return filepath.Abs(p)
	}
	p, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(p, "BYR-Client"), nil
}
