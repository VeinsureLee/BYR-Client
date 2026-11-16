package buildflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var ErrAndroidUndecided = errors.New("Android UI framework is undecided; compare docs/android-options.md before adding a production module; no APK was built")

type AndroidDecision struct {
	Status             string   `json:"status"`
	SelectedFramework  *string  `json:"selectedFramework"`
	Candidates         []string `json:"candidates"`
	ProductionModule   *string  `json:"productionModule"`
	EvaluationPriority []string `json:"evaluationPriority"`
}

// Engine is the build Facade over isolated modules, native MyGo packaging and
// classified output. It never chooses the application's product workflows.
type Engine struct {
	Root   string
	Run    RunCommand
	Output io.Writer
}

func (e Engine) Check(ctx context.Context, scope string) error {
	modules := map[string]string{"core": "core", "desktop": "apps/desktop", "android": "apps/android/app", "tools": "tools", "gio-probe": "apps/android/probes/gio"}
	selected := []string{scope}
	if scope == "all" {
		selected = []string{"core", "tools", "desktop", "android"}
	}
	for _, name := range selected {
		directory, ok := modules[name]
		if !ok {
			return fmt.Errorf("unknown check scope %q", scope)
		}
		for _, action := range []string{"test", "vet"} {
			if e.Output != nil {
				fmt.Fprintf(e.Output, "[%s] go %s ./...\n", name, action)
			}
			if err := e.Run(ctx, filepath.Join(e.Root, directory), hostEnvironment(), "go", action, "./..."); err != nil {
				return err
			}
		}
	}
	return nil
}

func hostEnvironment() map[string]string {
	return map[string]string{"GOWORK": "off", "CGO_ENABLED": "0", "GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH}
}

func (e Engine) CheckAndroidCore(ctx context.Context) (string, error) {
	// These are library packages. This is not gomobile binding, linking a GUI,
	// validating Android lifecycle behaviour, or producing an installable APK.
	environment := map[string]string{"GOWORK": "off", "CGO_ENABLED": "0", "GOOS": "android", "GOARCH": "arm64"}
	if err := e.Run(ctx, filepath.Join(e.Root, "core"), environment, "go", "build", "./..."); err != nil {
		return "", err
	}
	build, err := outputDirectory(e.Root, "build")
	if err != nil {
		return "", err
	}
	checks := filepath.Join(build, "checks")
	if err := os.MkdirAll(checks, 0755); err != nil {
		return "", err
	}
	if err := requireInside(build, checks); err != nil {
		return "", err
	}
	path := filepath.Join(checks, "android-arm64.json")
	err = writeJSON(path, map[string]any{
		"kind": "shared-core-compile-check", "target": "android/arm64",
		"module": "core", "uiFramework": nil, "apkProduced": false,
		"checkedAt": time.Now().UTC().Format(time.RFC3339),
	})
	return path, err
}

func (e Engine) AndroidStatus() (AndroidDecision, error) {
	var decision AndroidDecision
	data, err := os.ReadFile(filepath.Join(e.Root, "apps", "android", "decision.json"))
	if err != nil {
		return decision, err
	}
	err = json.Unmarshal(data, &decision)
	return decision, err
}

func (e Engine) BuildAndroid() error {
	decision, err := e.AndroidStatus()
	if err != nil {
		return err
	}
	if decision.Status == "pending" && decision.SelectedFramework == nil && decision.ProductionModule == nil {
		return ErrAndroidUndecided
	}
	return errors.New("Android decision requires an implemented and validated production module; editing decision.json alone cannot enable APK builds")
}

func (e Engine) RunDesktop(ctx context.Context) error {
	return e.Run(ctx, filepath.Join(e.Root, "apps", "desktop"), hostEnvironment(), "go", "run", ".")
}

type desktopMetadata struct {
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	Version    string `json:"version"`
	Identifier string `json:"identifier"`
}

type Manifest struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Target        string `json:"target"`
	Framework     string `json:"framework"`
	ApplicationID string `json:"applicationId"`
	Stage         string `json:"stage"`
	Entrypoint    string `json:"entrypoint"`
	Archive       string `json:"archive"`
	SHA256        string `json:"sha256"`
	PackagedAt    string `json:"packagedAt"`
}

func (e Engine) BuildDesktop(ctx context.Context, value string, installer bool) (string, error) {
	target, err := ParseTarget(value)
	if err != nil {
		return "", err
	}
	if installer && target.OS != "windows" {
		return "", errors.New("--installer applies only to Windows")
	}
	var metadata desktopMetadata
	data, err := os.ReadFile(filepath.Join(e.Root, "apps", "desktop", "mygo.json"))
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", err
	}
	if metadata.Name == "" || metadata.Slug == "" || metadata.Version == "" ||
		!regexp.MustCompile(`^[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+$`).MatchString(metadata.Identifier) ||
		strings.ContainsAny(metadata.Name+metadata.Slug, "/\\:*?\"<>|") || metadata.Name == "." || metadata.Name == ".." {
		return "", errors.New("invalid desktop build metadata")
	}
	build, err := outputDirectory(e.Root, "build")
	if err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(build, "desktop-")
	if err != nil {
		return "", err
	}
	defer removeGenerated(build, staging)
	arguments := []string{"tool", "mygo", "build", "-platform", target.String(), "-o", staging}
	if !installer {
		arguments = append(arguments, "-skip-installer")
	}
	if runtime.GOOS != "darwin" || os.Getenv("BYR_SKIP_DMG") == "1" {
		arguments = append(arguments, "-skip-dmg")
	}
	if err := e.Run(ctx, filepath.Join(e.Root, "apps", "desktop"), hostEnvironment(), "go", arguments...); err != nil {
		return "", err
	}
	bundle := filepath.Join(staging, target.Directory())
	binary := metadata.Slug
	if target.OS == "windows" {
		binary = metadata.Name + ".exe"
	}
	if target.OS == "darwin" {
		binary = filepath.Join(metadata.Name+".app", "Contents", "MacOS", metadata.Name)
	}
	info, err := os.Stat(filepath.Join(bundle, binary))
	if err != nil || info.IsDir() || info.Size() == 0 {
		return "", fmt.Errorf("missing or empty desktop executable for %s", target)
	}
	dist, err := outputDirectory(e.Root, "dist")
	if err != nil {
		return "", err
	}
	desktop := filepath.Join(dist, "desktop")
	if err := os.MkdirAll(desktop, 0755); err != nil {
		return "", err
	}
	if err := requireInside(dist, desktop); err != nil {
		return "", err
	}
	prepared, err := os.MkdirTemp(desktop, "."+target.Directory()+"-")
	if err != nil {
		return "", err
	}
	defer removeGenerated(desktop, prepared)
	app := filepath.Join(prepared, "app")
	if err := requireInside(build, bundle); err != nil {
		return "", err
	}
	if err := os.Rename(bundle, app); err != nil {
		return "", err
	}
	archiveName := "BYR-Client-" + target.Directory() + ".zip"
	license, err := os.ReadFile(filepath.Join(e.Root, "core", "resources", "fonts", "LICENSE"))
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(app, "FONT-LICENSE.txt"), license, 0644); err != nil {
		return "", err
	}
	// The app exposes the embedded license in Settings as well. Keep this
	// sidecar outside .app so packaging never changes a signed bundle.
	archivePath := filepath.Join(prepared, archiveName)
	if err := zipDirectory(app, archivePath, binary); err != nil {
		return "", err
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	_, copyErr := io.Copy(digest, archive)
	closeErr := archive.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	manifest := Manifest{
		Name: metadata.Name, Version: metadata.Version, Target: target.String(), Framework: "mygo",
		Stage: "feature-migration", Entrypoint: filepath.ToSlash(filepath.Join("app", binary)),
		ApplicationID: metadata.Identifier,
		Archive:       archiveName, SHA256: hex.EncodeToString(digest.Sum(nil)), PackagedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeJSON(filepath.Join(prepared, "manifest.json"), manifest); err != nil {
		return "", err
	}
	destination := filepath.Join(desktop, target.Directory())
	return publishDesktop(desktop, prepared, destination)
}

// publishDesktop always publishes to the single stable per-target directory.
// Windows may hold a release directory open in Explorer while the window shows
// it; directory swaps fail there, so the contents are replaced in place
// instead. Stale timestamped variants from older builds are pruned, and no
// timestamped directories are ever created again.
func publishDesktop(parent, prepared, destination string) (string, error) {
	err := publish(parent, prepared, destination)
	if err == nil {
		pruneVariants(parent, destination)
		return destination, nil
	}
	if errors.Is(err, os.ErrPermission) {
		if replaceErr := replaceInPlace(parent, prepared, destination); replaceErr == nil {
			pruneVariants(parent, destination)
			return destination, nil
		} else {
			return "", fmt.Errorf("cannot replace %s: %v (close the running app or file windows on that folder and retry)", destination, errors.Join(err, replaceErr))
		}
	}
	return "", err
}

// pruneVariants removes leftover timestamped directories and previous-version
// backups of destination, keeping a single release directory per target.
func pruneVariants(parent, destination string) {
	for _, pattern := range []string{destination + "-*", destination + ".previous-*"} {
		stale, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range stale {
			_ = removeGenerated(parent, path)
		}
	}
}

// replaceInPlace moves the prepared contents over an existing destination
// without swapping the directory itself. Moving children into a directory an
// Explorer window is showing succeeds; only deleting or renaming the directory
// itself is blocked. A running executable still fails, which the caller reports.
func replaceInPlace(parent, prepared, destination string) error {
	if err := requireInside(parent, prepared); err != nil {
		return err
	}
	if err := requireInside(parent, destination); err != nil {
		return err
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(destination, entry.Name())); err != nil {
			return err
		}
	}
	preparedEntries, err := os.ReadDir(prepared)
	if err != nil {
		return err
	}
	for _, entry := range preparedEntries {
		if err := os.Rename(filepath.Join(prepared, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
			return err
		}
	}
	return removeGenerated(parent, prepared)
}

func publish(parent, prepared, destination string) error {
	if err := requireInside(parent, prepared); err != nil {
		return err
	}
	if err := requireInside(parent, destination); err != nil {
		return err
	}
	var backup string
	if _, err := os.Lstat(destination); err == nil {
		backup = destination + fmt.Sprintf(".previous-%d", time.Now().UnixNano())
		if err := requireInside(parent, backup); err != nil {
			return err
		}
		if err := os.Rename(destination, backup); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(prepared, destination); err != nil {
		if backup != "" {
			if restoreErr := os.Rename(backup, destination); restoreErr != nil {
				return fmt.Errorf("publish: %v; restoring previous output: %w", err, restoreErr)
			}
		}
		return err
	}
	if backup != "" {
		// The new release is already in place; a locked old copy (for example a
		// still-running executable) is pruned by a later successful publish.
		_ = removeGenerated(parent, backup)
	}
	return nil
}
