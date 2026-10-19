// Package appdata stores local desktop preferences and the BYR session cookie.
// Passwords are never written to disk.
package appdata

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Settings struct {
	ThemeMode        int     `json:"themeMode"` // 0 follow system, 1 light, 2 dark
	Accent           int     `json:"accent"`    // 0 blue, 1 purple, 2 green, 3 orange
	FontScale        int     `json:"fontScale"` // 0 small, 1 standard, 2 large, 3 extra large
	SplitWidth       float32 `json:"splitWidth"`
	SplitHeight      float32 `json:"splitHeight"`
	RememberLogin    bool    `json:"rememberLogin"`
	RememberPassword bool    `json:"rememberPassword"`
	LastUser         string  `json:"lastUser,omitempty"`
	AIProvider       string  `json:"aiProvider,omitempty"`
	AIModel          string  `json:"aiModel,omitempty"`
	AIBaseURL        string  `json:"aiBaseURL,omitempty"`
	AIKeyID          string  `json:"aiKeyId,omitempty"`
}

func DefaultSettings() Settings {
	return Settings{
		ThemeMode:        0,
		Accent:           0,
		FontScale:        1,
		SplitWidth:       380,
		SplitHeight:      340,
		RememberLogin:    true,
		RememberPassword: true,
		AIProvider:       "deepseek",
		AIModel:          "deepseek-chat",
		AIBaseURL:        "https://api.deepseek.com/v1",
	}
}

func Dir() string {
	if value := os.Getenv("BYR_CLIENT_HOME"); value != "" {
		return value
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "BYR-Client")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".byr-client")
	}
	return ".byr-client"
}

func SettingsPath() string {
	return filepath.Join(Dir(), "settings.json")
}

func SessionPath() string {
	return filepath.Join(Dir(), "session.json")
}

func CredentialsPath() string {
	return filepath.Join(Dir(), "credentials.json")
}

func AIKeyPath() string {
	return filepath.Join(Dir(), "ai-key.txt")
}

func AIKeysPath() string {
	return filepath.Join(Dir(), "ai-keys.json")
}

func SaveAIKey(key string) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(AIKeyPath(), []byte(key))
}

func LoadAIKey() (string, bool) {
	data, err := os.ReadFile(AIKeyPath())
	if err != nil {
		return "", false
	}
	key := string(data)
	if key == "" {
		return "", false
	}
	return key, true
}

func ClearAIKey() error {
	err := os.Remove(AIKeyPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func SaveAIKeys(data []byte) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(AIKeysPath(), data)
}

func LoadAIKeys() ([]byte, bool) {
	data, err := os.ReadFile(AIKeysPath())
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

func ClearAIKeys() error {
	err := os.Remove(AIKeysPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func LoadSettings() Settings {
	settings := DefaultSettings()
	data, err := os.ReadFile(SettingsPath())
	if err != nil {
		return settings
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return DefaultSettings()
	}
	if settings.SplitWidth <= 0 {
		settings.SplitWidth = 380
	}
	if settings.SplitHeight <= 0 {
		settings.SplitHeight = 340
	}
	if settings.ThemeMode < 0 || settings.ThemeMode > 2 {
		settings.ThemeMode = 0
	}
	if settings.Accent < 0 || settings.Accent > 3 {
		settings.Accent = 0
	}
	if settings.FontScale < 0 || settings.FontScale > 3 {
		settings.FontScale = 1
	}
	if settings.AIProvider != "kimi" {
		settings.AIProvider = "deepseek"
	}
	if settings.AIProvider == "kimi" {
		if settings.AIModel == "" {
			settings.AIModel = "moonshot-v1-8k"
		}
		if settings.AIBaseURL == "" {
			settings.AIBaseURL = "https://api.moonshot.cn/v1"
		}
	} else {
		if settings.AIModel == "" {
			settings.AIModel = "deepseek-chat"
		}
		if settings.AIBaseURL == "" {
			settings.AIBaseURL = "https://api.deepseek.com/v1"
		}
	}
	return settings
}

func SaveSettings(settings Settings) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(SettingsPath(), data)
}

func SaveCredentials(credentials Credentials) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(CredentialsPath(), data)
}

func LoadCredentials() (Credentials, bool) {
	data, err := os.ReadFile(CredentialsPath())
	if err != nil {
		return Credentials{}, false
	}
	var credentials Credentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return Credentials{}, false
	}
	if credentials.Username == "" || credentials.Password == "" {
		return Credentials{}, false
	}
	return credentials, true
}

func ClearCredentials() error {
	err := os.Remove(CredentialsPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func ClearSession() error {
	err := os.Remove(SessionPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func writeFileAtomic(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
