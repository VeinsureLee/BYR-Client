// Command fynebuild is the single build entry point for the Fyne prototype.
// It only changes the environment of its child processes, never the user's
// global Go or Android configuration.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	fyneTool   = "fyne.io/tools/cmd/fyne@v1.7.4"
	ndkVersion = "28.2.13676358"
	appName    = "BYRFynePrototype"
	appID      = "com.byrclient.fyneprototype"
)

type options struct {
	target  string
	arch    string
	sdk     string
	ndk     string
	proxy   string
	verbose bool
}

func main() {
	var opts options
	flag.StringVar(&opts.target, "target", "windows", "windows, android, or check")
	flag.StringVar(&opts.arch, "arch", "", "Android: arm64 (default) or amd64 (x86_64 emulator)")
	flag.StringVar(&opts.sdk, "sdk", "", "Android SDK root; otherwise discover from environment or adb")
	flag.StringVar(&opts.ndk, "ndk", "", "Android NDK root; default: SDK/ndk/"+ndkVersion)
	flag.StringVar(&opts.proxy, "goproxy", "", "optional Go module proxy for child commands only")
	flag.BoolVar(&opts.verbose, "verbose", false, "show Fyne's full Android build log")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Unexpected positional argument. Use -target windows|android|check.")
		os.Exit(2)
	}
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "Build failed:", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.target != "windows" && opts.target != "android" && opts.target != "check" {
		return fmt.Errorf("unknown target %q: choose windows, android, or check", opts.target)
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	env := childEnv(opts.proxy)
	sdk := findSDK(opts.sdk)
	ndk := opts.ndk
	if ndk == "" {
		ndk = os.Getenv("ANDROID_NDK_HOME")
	}
	if ndk == "" && sdk != "" {
		ndk = filepath.Join(sdk, "ndk", ndkVersion)
	}
	if opts.target == "check" {
		return check(root, sdk, ndk, env)
	}
	if !exists(filepath.Join(root, "cmd", "byr-fyne", "main.go")) {
		return errors.New("cmd/byr-fyne/main.go was not found")
	}
	if opts.target == "windows" {
		if runtime.GOOS != "windows" {
			return errors.New("the Windows prototype must be built on a Windows runner with a C compiler")
		}
		if err := checkCommand("C compiler", compilerName()); err != nil {
			return err
		}
		output := filepath.Join(root, "dist", "fyne", "windows", "BYR-Fyne.exe")
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return err
		}
		env = setEnv(env, "CGO_ENABLED", "1")
		env = setEnv(env, "GOOS", "windows")
		env = setEnv(env, "GOARCH", "amd64")
		started := time.Now()
		if err := command(root, env, "go", "build", "-trimpath", "-ldflags=-H=windowsgui -s -w", "-o", output, "./cmd/byr-fyne"); err != nil {
			return err
		}
		return reportArtifact(output, started)
	}
	arch := opts.arch
	if arch == "" {
		arch = "arm64"
	}
	if arch != "arm64" && arch != "amd64" {
		return errors.New("Android -arch must be arm64 or amd64")
	}
	if err := checkAndroid(sdk, ndk); err != nil {
		return err
	}
	env = setEnv(env, "ANDROID_HOME", sdk)
	env = setEnv(env, "ANDROID_SDK_ROOT", sdk)
	env = setEnv(env, "ANDROID_NDK_HOME", ndk)
	env = setEnv(env, "CGO_ENABLED", "1")
	// The Fyne CLI is a host executable. Its internal mobile builder supplies
	// target-specific GOOS, GOARCH, CC, and CXX to the actual app compilation.
	env = setEnv(env, "GOOS", runtime.GOOS)
	env = setEnv(env, "GOARCH", runtime.GOARCH)
	mainDir := filepath.Join(root, "cmd", "byr-fyne")
	args := []string{"run", fyneTool, "package", "--target", "android/" + arch,
		"--name", appName, "--app-id", appID, "--app-version", "0.1.0",
		"--app-build", "1", "--icon", filepath.Join(root, "resources", "icon.png")}
	if opts.verbose {
		args = append(args, "--verbose")
	}
	started := time.Now()
	fmt.Printf("Android SDK: %s\nAndroid NDK: %s\n", sdk, ndk)
	if err := command(mainDir, env, "go", args...); err != nil {
		return err
	}
	// Mobile packaging does not accept --source-dir or an output directory.
	// Keep that implementation detail inside this runner.
	source := filepath.Join(mainDir, appName+".apk")
	output := filepath.Join(root, "dist", "fyne", "android", "BYR-Fyne-"+arch+".apk")
	if err := copyFile(source, output); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil {
		return err
	}
	return reportArtifact(output, started)
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if exists(filepath.Join(dir, "go.mod")) && exists(filepath.Join(dir, "resources", "icon.png")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("run this command from inside the BYR-Client repository")
		}
		dir = parent
	}
}

func childEnv(proxy string) []string {
	env := os.Environ()
	if proxy != "" {
		env = setEnv(env, "GOPROXY", proxy)
	}
	return env
}

func setEnv(env []string, key, value string) []string {
	result := make([]string, 0, len(env)+1)
	for _, item := range env {
		name, _, _ := strings.Cut(item, "=")
		if !strings.EqualFold(name, key) {
			result = append(result, item)
		}
	}
	return append(result, key+"="+value)
}

func findSDK(explicit string) string {
	if explicit != "" {
		full, _ := filepath.Abs(explicit)
		return full
	}
	candidates := []string{os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT")}
	if adb, err := exec.LookPath("adb"); err == nil {
		candidates = append(candidates, filepath.Dir(filepath.Dir(adb)))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		candidates = append(candidates, filepath.Join(local, "Android", "Sdk"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "Android", "Sdk"), filepath.Join(home, "Library", "Android", "sdk"))
	}
	for _, candidate := range candidates {
		if candidate != "" && exists(filepath.Join(candidate, "platform-tools")) && exists(filepath.Join(candidate, "ndk", ndkVersion)) {
			return candidate
		}
	}
	for _, candidate := range candidates {
		if candidate != "" && exists(filepath.Join(candidate, "platform-tools")) {
			return candidate
		}
	}
	return ""
}

func compilerName() string {
	if name := os.Getenv("CC"); name != "" {
		return name
	}
	return "gcc"
}

func check(root, sdk, ndk string, env []string) error {
	fmt.Printf("Project: %s\nHost: %s/%s\nPinned Fyne CLI: %s\n", root, runtime.GOOS, runtime.GOARCH, fyneTool)
	if err := command(root, env, "go", "version"); err != nil {
		return err
	}
	if err := checkCommand("C compiler", compilerName()); err != nil {
		fmt.Println("Windows build unavailable:", err)
	}
	if err := checkAndroid(sdk, ndk); err != nil {
		fmt.Println("Android build unavailable:", err)
		return err
	}
	fmt.Printf("Android SDK: %s\nAndroid NDK: %s\n", sdk, ndk)
	adb := filepath.Join(sdk, "platform-tools", executable("adb"))
	if exists(adb) {
		return command(root, env, adb, "devices", "-l")
	}
	return nil
}

func checkAndroid(sdk, ndk string) error {
	if sdk == "" || !exists(filepath.Join(sdk, "platforms")) {
		return errors.New("Android SDK not found; pass -sdk or set ANDROID_HOME")
	}
	if !exists(filepath.Join(sdk, "build-tools")) {
		return errors.New("Android SDK build-tools not found")
	}
	if ndk == "" || !exists(filepath.Join(ndk, "toolchains", "llvm", "prebuilt")) {
		return fmt.Errorf("Android NDK not found; install ndk;%s or pass -ndk", ndkVersion)
	}
	if err := checkCommand("Java", "java"); err != nil {
		return err
	}
	return checkCommand("Java compiler", "javac")
}

func checkCommand(label, name string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found (%s)", label, name)
	}
	fmt.Printf("%s: %s\n", label, path)
	return nil
}

func command(dir string, env []string, name string, args ...string) error {
	fmt.Printf("\n> %s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func executable(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("APK was not produced at %s: %w", source, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func reportArtifact(path string, started time.Time) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	fmt.Printf("\nBuilt: %s\nSize: %.1f MiB\nElapsed: %s\n", path, float64(info.Size())/(1024*1024), time.Since(started).Round(time.Second))
	return nil
}
