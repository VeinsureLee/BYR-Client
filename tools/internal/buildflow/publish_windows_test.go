//go:build windows

package buildflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// A running executable pins the release directory on Windows. Publishing must
// fail with a clear instruction instead of creating timestamped siblings or
// destroying the old release.
func TestLockedExecutableStillPublishesStableDirectory(t *testing.T) {
	root := fixture(t)
	target := filepath.Join(root, "dist", "desktop", "windows-amd64")
	binary := filepath.Join(target, "app", "BYR Client.exe")
	writeFixture(t, binary, "old application")
	name, err := syscall.UTF16PtrFromString(binary)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	e := Engine{Root: root, Run: fakeMyGo(t)}
	output, err := e.BuildDesktop(context.Background(), "windows/amd64", false)
	if err == nil || !strings.Contains(err.Error(), "close the running app") {
		t.Fatalf("expected a close-the-app error, got output=%s err=%v", output, err)
	}
	if output != "" {
		t.Fatalf("no output path should be reported, got %s", output)
	}
	old, readErr := os.ReadFile(binary)
	if readErr != nil || string(old) != "old application" {
		t.Fatalf("old application changed: %s %v", old, readErr)
	}
	if stale, globErr := filepath.Glob(target + "-*"); globErr != nil || len(stale) != 0 {
		t.Fatalf("timestamped siblings must not be created: %v %v", stale, globErr)
	}
}

// An Explorer window showing the release directory locks the directory itself.
// Publishing must replace its contents in place instead of creating a sibling.
func TestExplorerLockedDirectoryIsReplacedInPlace(t *testing.T) {
	root := fixture(t)
	target := filepath.Join(root, "dist", "desktop", "windows-amd64")
	writeFixture(t, filepath.Join(target, "app", "BYR Client.exe"), "old application")
	dirName, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(dirName, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	e := Engine{Root: root, Run: fakeMyGo(t)}
	output, err := e.BuildDesktop(context.Background(), "windows/amd64", false)
	if err != nil {
		t.Fatal(err)
	}
	if output != target {
		t.Fatalf("publish must keep the stable directory, got %s", output)
	}
	replacement, err := os.ReadFile(filepath.Join(target, "app", "BYR Client.exe"))
	if err != nil || !strings.Contains(string(replacement), "native application fixture") {
		t.Fatalf("release was not replaced in place: %q %v", replacement, err)
	}
	if stale, err := filepath.Glob(target + "-*"); err != nil || len(stale) != 0 {
		t.Fatalf("timestamped siblings must not be created: %v %v", stale, err)
	}
}

// Stale timestamped variants of earlier builds are pruned on a successful
// publish, keeping exactly one release directory per target.
func TestStaleTimestampedVariantsArePruned(t *testing.T) {
	root := fixture(t)
	target := filepath.Join(root, "dist", "desktop", "windows-amd64")
	stale := target + "-20260101T000000.000000000Z"
	writeFixture(t, filepath.Join(stale, "app", "BYR Client.exe"), "stale build")
	e := Engine{Root: root, Run: fakeMyGo(t)}
	output, err := e.BuildDesktop(context.Background(), "windows/amd64", false)
	if err != nil {
		t.Fatal(err)
	}
	if output != target {
		t.Fatalf("publish must keep the stable directory, got %s", output)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale timestamped variant was not pruned: %v", err)
	}
}
