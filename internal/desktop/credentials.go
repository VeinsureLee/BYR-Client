package desktop

import (
	"encoding/json"

	"BYR-Client/internal/platform/appdata"
)

const credentialsKey = "byr-credentials"

// loadSavedCredentials prefers the system credential store and falls back to a
// 0600 file only where the platform has no credential store.
func (u *UI) loadSavedCredentials() {
	if !u.persist {
		return
	}
	if !u.rememberPassword {
		if u.loginUser == "" {
			u.loginUser = u.settings.LastUser
		}
		return
	}
	var credentials appdata.Credentials
	if data, err := appSecrets.Get(credentialsKey); err == nil {
		if json.Unmarshal(data, &credentials) == nil {
			_ = appdata.ClearCredentials()
		}
	}
	if credentials.Username == "" || credentials.Password == "" {
		if fallback, ok := appdata.LoadCredentials(); ok {
			credentials = fallback
		}
	}
	if credentials.Username != "" {
		u.loginUser = credentials.Username
		u.loginPassword = credentials.Password
		return
	}
	if u.loginUser == "" {
		u.loginUser = u.settings.LastUser
	}
}

func (u *UI) saveSavedCredentials(username, password string) {
	if !u.rememberPassword || username == "" || password == "" {
		u.clearSavedCredentials()
		return
	}
	credentials := appdata.Credentials{Username: username, Password: password}
	data, err := json.Marshal(credentials)
	if err == nil {
		if err := appSecrets.Set(credentialsKey, data); err == nil {
			_ = appdata.ClearCredentials()
			return
		}
	}
	_ = appdata.SaveCredentials(credentials)
}

func (u *UI) clearSavedCredentials() {
	_ = appSecrets.Delete(credentialsKey)
	_ = appdata.ClearCredentials()
	u.loginPassword = ""
}

func (u *UI) hasSavedCredentials() bool {
	return u.loginUser != "" && u.loginPassword != ""
}
