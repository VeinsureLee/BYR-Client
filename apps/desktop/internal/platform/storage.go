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
