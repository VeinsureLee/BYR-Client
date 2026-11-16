package buildflow

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

// BuildAndroidProbe creates an explicitly separate evaluation APK. A successful
// build never changes decision.json or enables production Android releases.
func (e Engine) BuildAndroidProbe(ctx context.Context, framework string) (string, error) {
	if framework != "gio" {
		return "", errors.New("only the Gio evaluation probe exists; Fyne remains the documented comparison candidate")
	}
	module := filepath.Join(e.Root, "apps", "android", "probes", "gio")
	if !fileExists(filepath.Join(module, "go.mod")) {
		return "", errors.New("Gio evaluation module is missing")
	}
	dist, err := outputDirectory(e.Root, "dist")
	if err != nil {
		return "", err
	}
	evaluation := filepath.Join(dist, "evaluation", "android")
	if err := os.MkdirAll(evaluation, 0755); err != nil {
		return "", err
	}
	if err := requireInside(dist, evaluation); err != nil {
		return "", err
	}
	prepared, err := os.MkdirTemp(evaluation, ".gio-")
	if err != nil {
		return "", err
	}
	defer removeGenerated(evaluation, prepared)
	const artifact = "BYR-Gio-Probe.apk"
	path := filepath.Join(prepared, artifact)
	environment := hostEnvironment()
	if sdk := os.Getenv("ANDROID_HOME"); sdk == "" {
		if sdk = os.Getenv("ANDROID_SDK_ROOT"); sdk != "" {
			environment["ANDROID_HOME"] = sdk
		}
	}
	if err := e.Run(ctx, module, environment, "go", "tool", "gogio",
		"-target", "android", "-arch", "arm64", "-minsdk", "24", "-targetsdk", "36",
		"-appid", "cn.byr.bbs.byr_client.gioprobe", "-name", "BYR Gio 技术验证",
		"-version", "0.1.0.1", "-o", path, "."); err != nil {
		return "", err
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", errors.New("Gio tool did not produce a readable APK")
	}
	var manifest, native bool
	for _, file := range archive.File {
		if file.Name == "AndroidManifest.xml" && file.UncompressedSize64 > 0 {
			manifest = true
		}
		if file.Name == "lib/arm64-v8a/libgio.so" && file.UncompressedSize64 > 0 {
			native = true
		}
	}
	if err := archive.Close(); err != nil {
		return "", err
	}
	if !manifest || !native {
		return "", errors.New("Gio APK is missing its manifest or ARM64 library")
	}
	input, err := os.Open(path)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	_, copyErr := io.Copy(digest, input)
	closeErr := input.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	license, err := os.ReadFile(filepath.Join(e.Root, "core", "resources", "fonts", "LICENSE"))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(prepared, "FONT-LICENSE.txt"), license, 0644); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(prepared, "manifest.json"), map[string]any{
		"name": "BYR Gio 技术验证", "version": "0.1.0+1", "target": "android/arm64",
		"framework": "gio", "stage": "technology-evaluation", "production": false,
		"deviceTested": false, "signing": "debug", "minSdk": 24, "targetSdk": 36,
		"applicationId": "cn.byr.bbs.byr_client.gioprobe", "artifact": artifact,
		"sha256": hex.EncodeToString(digest.Sum(nil)), "packagedAt": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return "", err
	}
	destination := filepath.Join(evaluation, "gio")
	if err := publish(evaluation, prepared, destination); err != nil {
		return "", err
	}
	return destination, nil
}
