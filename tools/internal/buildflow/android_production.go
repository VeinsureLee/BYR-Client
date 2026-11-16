package buildflow

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// BuildAndroid packages the selected production Android module. The owner has
// selected Gio (see apps/android/decision.json); this method refuses to build
// when the decision still points elsewhere. The APK uses the debug signing key
// until a release keystore is configured.
func (e Engine) BuildAndroidProduction(ctx context.Context) (string, error) {
	decision, err := e.AndroidStatus()
	if err != nil {
		return "", err
	}
	if decision.Status == "pending" || decision.SelectedFramework == nil || decision.ProductionModule == nil {
		return "", ErrAndroidUndecided
	}
	if *decision.SelectedFramework != "gio" {
		return "", fmt.Errorf("Android framework %q is not implemented; only gio is available", *decision.SelectedFramework)
	}
	module := filepath.Join(e.Root, filepath.FromSlash(*decision.ProductionModule))
	if !fileExists(filepath.Join(module, "go.mod")) {
		return "", errors.New("production Android module is missing")
	}
	dist, err := outputDirectory(e.Root, "dist")
	if err != nil {
		return "", err
	}
	prepared, err := os.MkdirTemp(dist, ".android-")
	if err != nil {
		return "", err
	}
	defer removeGenerated(dist, prepared)
	const artifact = "BYR-Client-android-arm64.apk"
	path := filepath.Join(prepared, artifact)
	environment := hostEnvironment()
	if sdk := os.Getenv("ANDROID_HOME"); sdk == "" {
		if sdk = os.Getenv("ANDROID_SDK_ROOT"); sdk != "" {
			environment["ANDROID_HOME"] = sdk
		}
	}
	if err := e.Run(ctx, module, environment, "go", "tool", "gogio",
		"-target", "android", "-arch", "arm64", "-minsdk", "24", "-targetsdk", "36",
		"-appid", "cn.byr.bbs.byrclient", "-name", "BYR Client",
		"-version", "0.4.1.12", "-icon", "resources/icon.png",
		"-o", path, "."); err != nil {
		return "", err
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", errors.New("gogio did not produce a readable APK")
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
		return "", errors.New("Android APK is missing its manifest or ARM64 library")
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
		"name": "BYR Client", "version": "0.4.1", "versionCode": 12,
		"target": "android/arm64", "framework": "gio", "stage": "android-preview",
		"production": true, "deviceTested": false, "signing": "debug",
		"minSdk": 24, "targetSdk": 36, "applicationId": "cn.byr.bbs.byrclient",
		"artifact": artifact, "sha256": hex.EncodeToString(digest.Sum(nil)),
		"packagedAt": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return "", err
	}
	destination := filepath.Join(dist, "android")
	if err := publish(dist, prepared, destination); err != nil {
		return "", err
	}
	return destination, nil
}
