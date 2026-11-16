//go:build windows

package platform

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectStateSaveKeepsRecoverableBackup(t *testing.T) {
	dir := t.TempDir()
	file := FileState{Path: filepath.Join(dir, "state.json")}
	ctx := context.Background()
	if err := file.Save(ctx, []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := file.Save(ctx, []byte(`{"version":2}`)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".state-") {
			t.Fatalf("direct save created a blocked temporary file: %s", entry.Name())
		}
	}
	if err := os.WriteFile(file.Path, []byte("truncated"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := file.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"version":1`) {
		t.Fatalf("backup was not recovered: %s", b)
	}
}
