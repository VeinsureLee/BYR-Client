package appdata

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSettingsRoundTripAndDefaults(t *testing.T) {
	t.Setenv("BYR_CLIENT_HOME", t.TempDir())
	settings := LoadSettings()
	if settings.ThemeMode != 0 || !settings.RememberLogin {
		t.Fatalf("defaults = %+v", settings)
	}
	settings.ThemeMode = 2
	settings.Accent = 3
	settings.FontScale = 3
	settings.AIProvider = "kimi"
	settings.AIModel = "moonshot-v1-8k"
	settings.AIBaseURL = "https://api.moonshot.cn/v1"
	settings.AIKeyID = "key-123"
	settings.SplitWidth = 410
	settings.RememberLogin = false
	settings.RememberPassword = false
	settings.LastUser = "alice"
	if err := SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	loaded := LoadSettings()
	if loaded.ThemeMode != 2 || loaded.Accent != 3 || loaded.FontScale != 3 || loaded.AIProvider != "kimi" || loaded.AIModel != "moonshot-v1-8k" || loaded.AIKeyID != "key-123" || loaded.SplitWidth != 410 || loaded.RememberLogin || loaded.RememberPassword || loaded.LastUser != "alice" {
		t.Fatalf("loaded = %+v", loaded)
	}
	info, err := os.Stat(SettingsPath())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("settings mode = %o", info.Mode().Perm())
	}
}

func TestClearSession(t *testing.T) {
	t.Setenv("BYR_CLIENT_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(SessionPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(SessionPath(), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ClearSession(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SessionPath()); !os.IsNotExist(err) {
		t.Fatalf("session still exists: %v", err)
	}
}

func TestCredentialsFallbackFile(t *testing.T) {
	t.Setenv("BYR_CLIENT_HOME", t.TempDir())
	if err := SaveCredentials(Credentials{Username: "alice", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	credentials, ok := LoadCredentials()
	if !ok || credentials.Username != "alice" || credentials.Password != "secret" {
		t.Fatalf("credentials = %+v ok=%v", credentials, ok)
	}
	if err := ClearCredentials(); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadCredentials(); ok {
		t.Fatal("credentials survived clear")
	}
}

func TestAIKeyFallbackFile(t *testing.T) {
	t.Setenv("BYR_CLIENT_HOME", t.TempDir())
	if err := SaveAIKey("sk-test"); err != nil {
		t.Fatal(err)
	}
	key, ok := LoadAIKey()
	if !ok || key != "sk-test" {
		t.Fatalf("key = %q ok=%v", key, ok)
	}
	if err := ClearAIKey(); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadAIKey(); ok {
		t.Fatal("AI key survived clear")
	}
}
