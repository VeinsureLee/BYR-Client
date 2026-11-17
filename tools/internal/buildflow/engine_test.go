package buildflow

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.work": "go 1.27.4\n", "core/go.mod": "module example/core\ngo 1.25.6\n",
		"apps/desktop/go.mod":          "module example/desktop\ngo 1.27.4\n",
		"apps/desktop/mygo.json":       `{"name":"BYR Client","slug":"byr-client","version":"0.1.0","identifier":"cn.byr.bbs.byrclient"}`,
		"apps/android/decision.json":   `{"status":"pending","selectedFramework":null,"candidates":["fyne","gio"],"productionModule":null}`,
		"core/resources/fonts/LICENSE": "font license",
	} {
		writeFixture(t, filepath.Join(root, filepath.FromSlash(name)), body)
	}
	return root
}

func writeFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestTargetsRejectUnsupportedPlatformsAndTraversal(t *testing.T) {
	for _, input := range []string{"android/arm64", "windows/../../elsewhere", "linux/386", "darwin/", "web", "windows/amd64,linux/amd64"} {
		if _, err := ParseTarget(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	for _, input := range []string{"windows/amd64", "windows/arm64", "linux/amd64", "darwin/universal"} {
		target, err := ParseTarget(input)
		if err != nil || target.String() != input {
			t.Fatalf("target: %+v, %v", target, err)
		}
	}
}

func TestModuleChecksDoNotUseWorkspaceDependencyResolution(t *testing.T) {
	root := fixture(t)
	var calls []string
	engine := Engine{Root: root, Run: func(ctx context.Context, dir string, env map[string]string, name string, args ...string) error {
		if env["GOWORK"] != "off" {
			t.Fatal("module isolation missing")
		}
		if name != "go" || len(args) != 2 || args[1] != "./..." {
			t.Fatal("unexpected check command")
		}
		relative, _ := filepath.Rel(root, dir)
		calls = append(calls, filepath.ToSlash(relative)+":"+args[0])
		return nil
	}}
	if err := engine.Check(context.Background(), "all"); err != nil {
		t.Fatal(err)
	}
	want := []string{"core:test", "core:vet", "tools:test", "tools:vet", "apps/desktop:test", "apps/desktop:vet", "apps/android/app:test", "apps/android/app:vet"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("commands: %v", calls)
	}
}

func TestAndroidCoreReportIsNotAnApplication(t *testing.T) {
	root := fixture(t)
	engine := Engine{Root: root, Run: func(ctx context.Context, dir string, env map[string]string, name string, args ...string) error {
		if dir != filepath.Join(root, "core") || env["GOOS"] != "android" || env["GOARCH"] != "arm64" || env["CGO_ENABLED"] != "0" || env["GOWORK"] != "off" {
			t.Fatal("Android check leaked desktop dependencies or wrong target")
		}
		if !reflect.DeepEqual(args, []string{"build", "./..."}) {
			t.Fatal(args)
		}
		return nil
	}}
	path, err := engine.CheckAndroidCore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Kind string `json:"kind"`
		APK  bool   `json:"apkProduced"`
	}
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &report); err != nil || report.Kind != "shared-core-compile-check" || report.APK {
		t.Fatalf("misleading Android report: %s", data)
	}
	if _, err := os.Stat(filepath.Join(root, "dist")); !os.IsNotExist(err) {
		t.Fatal("core check created distributable software")
	}
}

func TestFailedCoreCheckDoesNotClaimSuccess(t *testing.T) {
	root := fixture(t)
	engine := Engine{Root: root, Run: func(context.Context, string, map[string]string, string, ...string) error {
		return errors.New("compiler failed")
	}}
	if _, err := engine.CheckAndroidCore(context.Background()); err == nil {
		t.Fatal("compiler failure was ignored")
	}
	if _, err := os.Stat(filepath.Join(root, "build", "checks", "android-arm64.json")); !os.IsNotExist(err) {
		t.Fatal("published a false success report")
	}
}

func TestPendingAndroidBuildDoesNotSelectOrDownloadAFramework(t *testing.T) {
	root := fixture(t)
	engine := Engine{Root: root, Run: func(context.Context, string, map[string]string, string, ...string) error {
		t.Fatal("pending Android build ran a tool")
		return nil
	}}
	if err := engine.BuildAndroid(); !errors.Is(err, ErrAndroidUndecided) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist")); !os.IsNotExist(err) {
		t.Fatal("created a fake Android artifact")
	}
	writeFixture(t, filepath.Join(root, "apps", "android", "decision.json"), `{"status":"accepted","selectedFramework":"fyne","productionModule":"app"}`)
	if err := engine.BuildAndroid(); err == nil || errors.Is(err, ErrAndroidUndecided) {
		t.Fatal("a metadata edit incorrectly enabled a production build")
	}
}

func fakeMyGo(t *testing.T) RunCommand {
	t.Helper()
	return func(ctx context.Context, dir string, env map[string]string, name string, args ...string) error {
		if env["GOWORK"] != "off" {
			t.Fatal("desktop must build independently of go.work")
		}
		if !strings.Contains(strings.Join(args, " "), "tool mygo build") {
			t.Fatalf("unexpected native build command: %v", args)
		}
		var output string
		for index, arg := range args {
			if arg == "-o" {
				output = args[index+1]
			}
		}
		if output == "" {
			t.Fatal("MyGo output is not staged")
		}
		writeFixture(t, filepath.Join(output, "windows-amd64", "BYR Client.exe"), "native application fixture")
		writeFixture(t, filepath.Join(output, "windows-amd64", "resources", "required.dat"), "required runtime resource")
		skipInstaller := false
		for _, arg := range args {
			if arg == "-skip-installer" {
				skipInstaller = true
			}
		}
		if !skipInstaller {
			writeFixture(t, filepath.Join(output, "windows-amd64", "byr-client-setup-0.1.0-amd64.exe"), "installer fixture")
		}
		return nil
	}
}

func TestDesktopReleaseContainsRuntimeResourcesAndManifest(t *testing.T) {
	root := fixture(t)
	writeFixture(t, filepath.Join(root, "dist", "android", "untouched.txt"), "another target")
	engine := Engine{Root: root, Run: fakeMyGo(t)}
	output, err := engine.BuildDesktop(context.Background(), "windows/amd64", false)
	if err != nil {
		t.Fatal(err)
	}
	if output != filepath.Join(root, "dist", "desktop", "windows-amd64") {
		t.Fatal(output)
	}
	manifestBytes, _ := os.ReadFile(filepath.Join(output, "manifest.json"))
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Stage != "feature-migration" || manifest.Framework != "mygo" || manifest.Entrypoint != "app/BYR Client.exe" {
		t.Fatalf("incorrect manifest: %+v", manifest)
	}
	archivePath := filepath.Join(output, manifest.Archive)
	archiveBytes, _ := os.ReadFile(archivePath)
	digest := sha256.Sum256(archiveBytes)
	if manifest.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("archive checksum is wrong")
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	contents := map[string]string{}
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if file.Name == "BYR Client.exe" && file.Mode().Perm() != 0755 {
			t.Fatalf("archive entry point lost executable permissions: %v", file.Mode())
		}
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(input)
		input.Close()
		if err != nil {
			t.Fatal(err)
		}
		contents[file.Name] = string(data)
	}
	if contents["BYR Client.exe"] != "native application fixture" || contents["resources/required.dat"] != "required runtime resource" {
		t.Fatalf("incomplete archive: %v", contents)
	}
	if !fileExists(filepath.Join(root, "dist", "android", "untouched.txt")) {
		t.Fatal("desktop packaging modified Android output")
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	// Replace the complete output, so resources removed from a later build
	// cannot remain alongside the new executable.
	writeFixture(t, filepath.Join(output, "app", "stale.dll"), "stale")
	if _, err := engine.BuildDesktop(context.Background(), "windows/amd64", false); err != nil {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(output, "app", "stale.dll")) {
		t.Fatal("stale resources survived a complete replacement")
	}
}

func TestDesktopInstallerIsPublishedBesideArchive(t *testing.T) {
	root := fixture(t)
	engine := Engine{Root: root, Run: fakeMyGo(t)}
	output, err := engine.BuildDesktop(context.Background(), "windows/amd64", true)
	if err != nil {
		t.Fatal(err)
	}
	installerPath := filepath.Join(output, "BYR-Client-windows-amd64-setup.exe")
	installer, err := os.ReadFile(installerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(installer) != "installer fixture" {
		t.Fatalf("installer contents = %q", installer)
	}
	manifestBytes, _ := os.ReadFile(filepath.Join(output, "manifest.json"))
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(installer)
	if manifest.Installer != "BYR-Client-windows-amd64-setup.exe" ||
		manifest.InstallerSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("installer manifest fields = %+v", manifest)
	}
	archive, err := zip.OpenReader(filepath.Join(output, manifest.Archive))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if strings.Contains(file.Name, "setup") {
			t.Fatalf("installer leaked into the portable archive: %s", file.Name)
		}
	}
}

func TestFailedBuildPreservesPreviouslyUsableSoftware(t *testing.T) {
	root := fixture(t)
	previous := filepath.Join(root, "dist", "desktop", "windows-amd64", "app", "BYR Client.exe")
	writeFixture(t, previous, "previous working build")
	engine := Engine{Root: root, Run: func(context.Context, string, map[string]string, string, ...string) error {
		return errors.New("native compile failed")
	}}
	if _, err := engine.BuildDesktop(context.Background(), "windows/amd64", false); err == nil {
		t.Fatal("build failure ignored")
	}
	data, _ := os.ReadFile(previous)
	if string(data) != "previous working build" {
		t.Fatal("failed build destroyed previous software")
	}
}

func TestPublishRestoresPreviousOutputIfRenameFails(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "windows-amd64")
	writeFixture(t, filepath.Join(destination, "keep.txt"), "previous")
	if err := publish(parent, filepath.Join(parent, "missing-prepared-folder"), destination); err == nil {
		t.Fatal("expected failed publication")
	}
	data, _ := os.ReadFile(filepath.Join(destination, "keep.txt"))
	if string(data) != "previous" {
		t.Fatal("previous output was not restored")
	}
}

func TestGeneratedCleanupCannotEscapeItsParent(t *testing.T) {
	parent, outside := t.TempDir(), t.TempDir()
	writeFixture(t, filepath.Join(outside, "keep.txt"), "do not remove")
	if err := removeGenerated(parent, outside); err == nil {
		t.Fatal("accepted recursive cleanup outside generated directory")
	}
	if !fileExists(filepath.Join(outside, "keep.txt")) {
		t.Fatal("outside file was removed")
	}
	if err := requireInside(parent, parent); err == nil {
		t.Fatal("accepted deletion of parent itself")
	}
}

func TestFindRootFromNestedModule(t *testing.T) {
	root := fixture(t)
	found, err := FindRoot(filepath.Join(root, "apps", "desktop"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(root)
	if found != want {
		t.Fatalf("root %s, want %s", found, want)
	}
}

func TestEnvironmentOverrideReplacesInheritedGoTarget(t *testing.T) {
	result := mergeEnvironment([]string{"GOOS=windows", "GOARCH=amd64", "OTHER=unchanged"}, map[string]string{"GOOS": "android", "GOARCH": "arm64", "GOWORK": "off"})
	joined := strings.Join(result, "\n")
	if strings.Contains(joined, "GOOS=windows") || strings.Contains(joined, "GOARCH=amd64") || !strings.Contains(joined, "OTHER=unchanged") || !strings.Contains(joined, "GOWORK=off") {
		t.Fatalf("incorrect subprocess environment: %v", result)
	}
}

func TestCrossCompiledArchiveMakesUnixEntryAndInstallerExecutable(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	writeFixture(t, filepath.Join(app, "BYR Client.app", "Contents", "MacOS", "BYR Client"), "cross-compiled binary")
	writeFixture(t, filepath.Join(app, "install.sh"), "#!/bin/sh\n")
	path := filepath.Join(root, "portable.zip")
	if err := zipDirectory(app, path, filepath.Join("BYR Client.app", "Contents", "MacOS", "BYR Client")); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.FileInfo().IsDir() || file.Name == "install.sh" || strings.HasSuffix(file.Name, "/BYR Client") {
			if file.Mode().Perm() != 0755 {
				t.Fatalf("non-portable archive permissions for %s: %v", file.Name, file.Mode())
			}
		}
	}
}
