package platform

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/local"
)

func TestApplicationStateSurvivesRestartWithoutCredentials(t *testing.T) {
	ctx := context.Background()
	file := FileState{Path: filepath.Join(t.TempDir(), "app", "state.json")}
	s, err := local.Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetPreferences(ctx, local.Preferences{Theme: 2, FontScale: 1.2, IPv6: true}); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveDraft(ctx, "alice", "composeDraft.Go.new", local.Draft{Title: "标题", Content: "内容"}); err != nil {
		t.Fatal(err)
	}
	restored, err := local.Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Preferences().IPv6 || restored.Preferences().Theme != 2 {
		t.Fatal("preferences lost")
	}
	d, ok := restored.Draft("alice", "composeDraft.Go.new")
	if !ok || d.Content != "内容" {
		t.Fatal("draft lost")
	}
	b, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "access_token") || strings.Contains(string(b), "password") {
		t.Fatal("credentials written to ordinary state file")
	}
}

func TestDirWritableRejectsFilePath(t *testing.T) {
	dir := t.TempDir()
	if !dirWritable(dir) {
		t.Fatal("temporary directory should be writable")
	}
	file := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if dirWritable(file) {
		t.Fatal("a regular file must not be treated as a writable directory")
	}
}

func TestInvalidStateIsReportedWhenBackupCannotRecoverIt(t *testing.T) {
	file := FileState{Path: filepath.Join(t.TempDir(), "state.json")}
	if err := os.WriteFile(file.Path, []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Load(context.Background()); err == nil {
		t.Fatal("invalid state was returned as a successful load")
	}
}
