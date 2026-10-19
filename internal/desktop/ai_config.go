package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"BYR-Client/internal/ai"
	"BYR-Client/internal/platform/appdata"
)

const (
	aiKeysSecret    = "byr-ai-keys"
	legacyAIKeyName = "byr-ai-key"
)

type aiKeyEntry struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Label    string `json:"label"`
	Key      string `json:"key"`
}

func aiProviderLabel(provider string) string {
	if provider == string(ai.Kimi) {
		return "Kimi"
	}
	return "DeepSeek"
}

func aiProviderKey(label string) ai.Provider {
	if strings.EqualFold(strings.TrimSpace(label), "Kimi") {
		return ai.Kimi
	}
	return ai.DeepSeek
}

func (u *UI) loadAIConfig() {
	if !u.persist {
		return
	}
	u.aiProvider = aiProviderLabel(u.settings.AIProvider)
	u.aiModel = u.settings.AIModel
	u.aiBaseURL = u.settings.AIBaseURL
	if u.aiModel == "" || u.aiBaseURL == "" {
		base, model := ai.Defaults(aiProviderKey(u.aiProvider))
		if u.aiModel == "" {
			u.aiModel = model
		}
		if u.aiBaseURL == "" {
			u.aiBaseURL = base
		}
	}
	u.aiKeys = u.loadAIKeys()
	u.aiKeyID = u.settings.AIKeyID
	if !u.hasKeyID(u.aiKeyID) {
		u.aiKeyID = ""
		for _, entry := range u.aiKeys {
			if entry.Provider == string(aiProviderKey(u.aiProvider)) {
				u.aiKeyID = entry.ID
				break
			}
		}
	}
	if u.aiKeyID == "" && len(u.aiKeys) > 0 {
		u.aiKeyID = u.aiKeys[0].ID
	}
	u.applyActiveAIKey()
	if entry, ok := u.activeAIKey(); ok && aiProviderLabel(entry.Provider) != aiProviderLabel(u.settings.AIProvider) {
		base, model := ai.Defaults(aiProviderKey(entry.Provider))
		u.aiBaseURL = base
		u.aiModel = model
	}
}

func (u *UI) loadAIKeys() []aiKeyEntry {
	var entries []aiKeyEntry
	if data, err := appSecrets.Get(aiKeysSecret); err == nil {
		_ = json.Unmarshal(data, &entries)
		_ = appdata.ClearAIKeys()
	}
	if len(entries) == 0 {
		if data, ok := appdata.LoadAIKeys(); ok {
			_ = json.Unmarshal(data, &entries)
		}
	}
	filtered := entries[:0]
	for _, entry := range entries {
		if strings.TrimSpace(entry.Key) != "" {
			filtered = append(filtered, entry)
		}
	}
	entries = filtered
	if len(entries) > 0 {
		return entries
	}
	// Migrate the previous single-key version if it exists.
	legacy := ""
	if data, err := appSecrets.Get(legacyAIKeyName); err == nil {
		legacy = strings.TrimSpace(string(data))
	}
	if legacy == "" {
		if key, ok := appdata.LoadAIKey(); ok {
			legacy = key
		}
	}
	if legacy == "" {
		return nil
	}
	entry := aiKeyEntry{
		ID:       newAIKeyID(),
		Provider: string(aiProviderKey(u.aiProvider)),
		Label:    "默认 Key",
		Key:      legacy,
	}
	entries = []aiKeyEntry{entry}
	if data, err := json.Marshal(entries); err == nil {
		if err := appSecrets.Set(aiKeysSecret, data); err == nil {
			_ = appdata.ClearAIKeys()
			_ = appSecrets.Delete(legacyAIKeyName)
			_ = appdata.ClearAIKey()
		} else {
			_ = appdata.SaveAIKeys(data)
		}
	}
	return entries
}

func (u *UI) persistAIKeys() {
	u.settings.AIKeyID = u.aiKeyID
	u.settings.AIProvider = string(aiProviderKey(u.aiProvider))
	u.settings.AIModel = u.aiModel
	u.settings.AIBaseURL = u.aiBaseURL
	u.saveSettings()
	if len(u.aiKeys) == 0 {
		_ = appSecrets.Delete(aiKeysSecret)
		_ = appdata.ClearAIKeys()
		return
	}
	data, err := json.Marshal(u.aiKeys)
	if err != nil {
		return
	}
	if err := appSecrets.Set(aiKeysSecret, data); err == nil {
		_ = appdata.ClearAIKeys()
	} else {
		_ = appdata.SaveAIKeys(data)
	}
}

func (u *UI) activeAIKey() (aiKeyEntry, bool) {
	for _, entry := range u.aiKeys {
		if entry.ID == u.aiKeyID {
			return entry, true
		}
	}
	return aiKeyEntry{}, false
}

func (u *UI) hasKeyID(id string) bool {
	if id == "" {
		return false
	}
	for _, entry := range u.aiKeys {
		if entry.ID == id {
			return true
		}
	}
	return false
}

func (u *UI) applyActiveAIKey() {
	entry, ok := u.activeAIKey()
	if !ok {
		u.aiKey = ""
		u.aiKeySaved = false
		return
	}
	u.aiKey = entry.Key
	u.aiKeySaved = true
	u.aiProvider = aiProviderLabel(entry.Provider)
}

func (u *UI) saveAIConfig() {
	provider := aiProviderKey(u.aiProvider)
	u.settings.AIProvider = string(provider)
	u.settings.AIModel = strings.TrimSpace(u.aiModel)
	u.settings.AIBaseURL = strings.TrimRight(strings.TrimSpace(u.aiBaseURL), "/")
	if input := strings.TrimSpace(u.aiKeyInput); input != "" {
		label := strings.TrimSpace(u.aiKeyLabel)
		if label == "" {
			label = aiProviderLabel(string(provider)) + " Key"
		}
		entry := aiKeyEntry{ID: newAIKeyID(), Provider: string(provider), Label: label, Key: input}
		u.aiKeys = append(u.aiKeys, entry)
		u.aiKeyID = entry.ID
		u.aiKey = input
		u.aiKeySaved = true
		u.aiKeyInput = ""
		u.aiKeyLabel = ""
	}
	u.persistAIKeys()
	if u.aiKeySaved {
		u.aiTestStatus = "AI 配置已保存"
	} else {
		u.aiTestStatus = "请添加一个 API Key"
	}
}

func (u *UI) addAIKeyFromInput() {
	input := strings.TrimSpace(u.aiKeyInput)
	if input == "" {
		u.aiTestStatus = "请填写 API Key"
		return
	}
	label := strings.TrimSpace(u.aiKeyLabel)
	provider := aiProviderKey(u.aiProvider)
	if label == "" {
		label = aiProviderLabel(string(provider)) + " Key"
	}
	entry := aiKeyEntry{ID: newAIKeyID(), Provider: string(provider), Label: label, Key: input}
	u.aiKeys = append(u.aiKeys, entry)
	u.aiKeyID = entry.ID
	u.aiKey = input
	u.aiKeySaved = true
	u.aiKeyInput = ""
	u.aiKeyLabel = ""
	u.persistAIKeys()
	u.aiTestStatus = "已添加 API Key"
}

func (u *UI) selectAIKey(id string) {
	for _, entry := range u.aiKeys {
		if entry.ID != id {
			continue
		}
		previousProvider := aiProviderLabel(u.aiProvider)
		u.aiKeyID = id
		u.applyActiveAIKey()
		if aiProviderLabel(entry.Provider) != previousProvider {
			base, model := ai.Defaults(aiProviderKey(entry.Provider))
			u.aiBaseURL = base
			u.aiModel = model
		}
		u.persistAIKeys()
		return
	}
}

func (u *UI) deleteAIKey(id string) {
	next := make([]aiKeyEntry, 0, len(u.aiKeys))
	for _, entry := range u.aiKeys {
		if entry.ID != id {
			next = append(next, entry)
		}
	}
	u.aiKeys = next
	if u.aiKeyID == id {
		u.aiKeyID = ""
		if len(u.aiKeys) > 0 {
			u.aiKeyID = u.aiKeys[0].ID
		}
	}
	u.applyActiveAIKey()
	u.persistAIKeys()
	if _, ok := u.activeAIKey(); !ok {
		u.aiTestStatus = "已删除 Key，当前没有可用 Key"
	} else {
		u.aiTestStatus = "已删除 Key"
	}
}

func (u *UI) aiConfig() (ai.Config, bool) {
	provider := aiProviderKey(u.aiProvider)
	key := strings.TrimSpace(u.aiKeyInput)
	if key == "" {
		if entry, ok := u.activeAIKey(); ok {
			key = entry.Key
			provider = aiProviderKey(entry.Provider)
		} else {
			key = u.aiKey
		}
	}
	if key == "" {
		return ai.Config{}, false
	}
	return ai.Normalize(ai.Config{
		Provider: provider,
		APIKey:   key,
		BaseURL:  u.aiBaseURL,
		Model:    u.aiModel,
	}), true
}

func (u *UI) testAIConfig() {
	if u.aiTesting {
		return
	}
	config, ok := u.aiConfig()
	if !ok {
		u.aiTestStatus = "请先添加或选择一个 API Key"
		return
	}
	win := u.window()
	u.aiTesting = true
	u.aiTestStatus = "正在测试连接…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := ai.New(config).Test(ctx)
		if win == nil {
			return
		}
		win.Update(func() {
			u.aiTesting = false
			if err != nil {
				u.aiTestStatus = "连接失败：" + err.Error()
				return
			}
			u.aiTestStatus = "连接成功：" + string(config.Provider) + " · " + config.Model
		})
	}()
}

func newAIKeyID() string {
	return fmt.Sprintf("key-%d", time.Now().UnixNano())
}

func maskAIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
