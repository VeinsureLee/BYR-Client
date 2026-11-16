package buildflow

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGioEvaluationCannotSelectProductionFramework(t *testing.T) {
	root := fixture(t)
	module := filepath.Join(root, "apps", "android", "probes", "gio")
	writeFixture(t, filepath.Join(module, "go.mod"), "module example/gio-probe\ngo 1.25.6\n")
	writeFixture(t, filepath.Join(root, "core", "resources", "fonts", "LICENSE"), "font license")
	before, _ := os.ReadFile(filepath.Join(root, "apps", "android", "decision.json"))
	engine := Engine{Root: root, Run: func(ctx context.Context, directory string, env map[string]string, name string, args ...string) error {
		if directory != module || env["GOWORK"] != "off" {
			t.Fatal("Gio probe was not isolated")
		}
		command := strings.Join(args, " ")
		if !strings.Contains(command, "tool gogio -target android -arch arm64") || !strings.Contains(command, "cn.byr.bbs.byr_client.gioprobe") {
			t.Fatal(command)
		}
		var output string
		for index, arg := range args {
			if arg == "-o" {
				output = args[index+1]
			}
		}
		file, err := os.Create(output)
		if err != nil {
			return err
		}
		archive := zip.NewWriter(file)
		for _, path := range []string{"AndroidManifest.xml", "lib/arm64-v8a/libgio.so"} {
			writer, err := archive.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write([]byte("fixture")); err != nil {
				t.Fatal(err)
			}
		}
		if err := archive.Close(); err != nil {
			return err
		}
		return file.Close()
	}}
	output, err := engine.BuildAndroidProbe(context.Background(), "gio")
	if err != nil {
		t.Fatal(err)
	}
	if output != filepath.Join(root, "dist", "evaluation", "android", "gio") {
		t.Fatal(output)
	}
	data, _ := os.ReadFile(filepath.Join(output, "manifest.json"))
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["production"] != false || manifest["deviceTested"] != false || manifest["stage"] != "technology-evaluation" || manifest["signing"] != "debug" {
		t.Fatalf("misleading evaluation manifest: %s", data)
	}
	after, _ := os.ReadFile(filepath.Join(root, "apps", "android", "decision.json"))
	if string(before) != string(after) {
		t.Fatal("probe build changed the production decision")
	}
	if _, err := os.Stat(filepath.Join(root, "dist", "android")); !os.IsNotExist(err) {
		t.Fatal("probe was presented as production Android software")
	}
	if !errors.Is(engine.BuildAndroid(), ErrAndroidUndecided) {
		t.Fatal("probe build enabled production Android builds")
	}
}

func TestFailedProbePreservesPreviousEvaluationAPK(t *testing.T) {
	root := fixture(t)
	writeFixture(t, filepath.Join(root, "apps", "android", "probes", "gio", "go.mod"), "module example/gio-probe\n")
	previous := filepath.Join(root, "dist", "evaluation", "android", "gio", "BYR-Gio-Probe.apk")
	writeFixture(t, previous, "previous evaluation")
	engine := Engine{Root: root, Run: func(context.Context, string, map[string]string, string, ...string) error {
		return errors.New("Android toolchain failed")
	}}
	if _, err := engine.BuildAndroidProbe(context.Background(), "gio"); err == nil {
		t.Fatal("toolchain failure ignored")
	}
	data, _ := os.ReadFile(previous)
	if string(data) != "previous evaluation" {
		t.Fatal("previous evaluation was destroyed")
	}
	if _, err := engine.BuildAndroidProbe(context.Background(), "fyne"); err == nil {
		t.Fatal("accepted a nonexistent Fyne probe")
	}
}
