package platform

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveImageWritesOriginalBytesToConfiguredExportDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BYR_IMAGE_EXPORT_DIR", dir)
	data := []byte("original encoded image bytes")
	path, err := SaveImage(context.Background(), "../photo.gif", data)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := path, filepath.Join(dir, "photo.gif"); got != want {
		t.Fatalf("saved path = %q, want %q", got, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("saved bytes = %q, want original %q", got, data)
	}
}

func TestSaveImageRejectsEmptyBytes(t *testing.T) {
	if _, err := SaveImage(context.Background(), "empty.png", nil); err == nil {
		t.Fatal("SaveImage accepted empty data")
	}
}
