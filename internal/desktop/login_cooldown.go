package desktop

import (
	"errors"
	"strings"
	"time"

	"BYR-Client/internal/ports"
)

const loginCooldown = 10 * time.Second

func (u *UI) loginTime() time.Time {
	if u.loginClock != nil {
		return u.loginClock()
	}
	return time.Now()
}

func (u *UI) loginCooldownSeconds(now time.Time) int {
	remaining := u.loginRetryAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	return int((remaining + time.Second - 1) / time.Second)
}

func (u *UI) startLoginCooldown(now time.Time) {
	deadline := now.Add(loginCooldown)
	if deadline.After(u.loginRetryAt) {
		u.loginRetryAt = deadline
	}
	u.saveSettings()
}

// Run on the UI thread, just like other presentation-state changes.
func (u *UI) recordLoginFailure(err error, now time.Time) {
	if u.closed {
		return
	}
	u.loginBusy = false
	u.loginStatus = ""
	u.loginError = err.Error()
	if errors.Is(err, ports.ErrAuthRequired) {
		u.loginError = "登录状态无效，请重新登录"
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "频繁") || strings.Contains(message, "http 429") || strings.Contains(message, "too many requests") {
		// Start a full cooldown from the rejection, not from the request:
		// a slow response must not leave the retry button immediately enabled.
		u.startLoginCooldown(now)
	}
}
