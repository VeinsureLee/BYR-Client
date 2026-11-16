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
