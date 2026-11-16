package buildflow

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectedGioProductionBuildPackagesStableAPK(t *testing.T) {
	root := fixture(t)
	writeFixture(t, filepath.Join(root, "apps", "android", "decision.json"), `{"status":"selected","selectedFramework":"gio","productionModule":"apps/android/app"}`)
	module := filepath.Join(root, "apps", "android", "app")
	writeFixture(t, filepath.Join(module, "go.mod"), "module example/android-app\ngo 1.25.6\n")
	writeFixture(t, filepath.Join(root, "core", "resources", "fonts", "LICENSE"), "font license")
	engine := Engine{Root: root, Run: func(ctx context.Context, directory string, env map[string]string, name string, args ...string) error {
		if directory != module || env["GOWORK"] != "off" {
			t.Fatalf("Android build was not isolated: %s %v", directory, env)
		}
		command := strings.Join(args, " ")
		if !strings.Contains(command, "tool gogio -target android -arch arm64") ||
			!strings.Contains(command, "cn.byr.bbs.byrclient") ||
			!strings.Contains(command, "-version 0.4.1.12") ||
			!strings.Contains(command, "-icon resources/icon.png") {
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
	output, err := engine.BuildAndroidProduction(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if output != filepath.Join(root, "dist", "android") {
		t.Fatal(output)
	}
	data, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["production"] != true || manifest["stage"] != "android-preview" || manifest["signing"] != "debug" || manifest["framework"] != "gio" {
		t.Fatalf("unexpected manifest: %s", data)
	}
	if manifest["version"] != "0.4.1" || manifest["versionCode"] != float64(12) {
		t.Fatalf("manifest version drifted from the app: %s", data)
	}
	if _, err := os.Stat(filepath.Join(output, "BYR-Client-android-arm64.apk")); err != nil {
		t.Fatal(err)
	}
	if entries, _ := filepath.Glob(filepath.Join(root, "dist", "android-*")); len(entries) != 0 {
		t.Fatalf("timestamped Android variants were created: %v", entries)
	}
}

func TestPendingDecisionStillRefusesProductionBuild(t *testing.T) {
	root := fixture(t)
	engine := Engine{Root: root, Run: func(context.Context, string, map[string]string, string, ...string) error {
		t.Fatal("pending build ran a tool")
		return nil
	}}
	if _, err := engine.BuildAndroidProduction(context.Background()); err == nil {
		t.Fatal("pending decision produced an Android build")
	}
}
