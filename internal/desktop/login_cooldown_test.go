package desktop

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"BYR-Client/internal/platform/appdata"
)

func TestLoginCooldownCountsDownAndUnlocks(t *testing.T) {
	u := newUI(false)
	tester := ui.NewTester(u.view, 980, 640)
	u.loginClock = tester.Now
	u.recordLoginFailure(errors.New("请勿频繁登录"), tester.Now())
	tester.Frame()
	if !tester.HasText("登录（10s）") || !tester.HasText("请等待 10s 后再登录") {
		t.Fatalf("countdown missing: %v", tester.Texts())
	}
	if err := tester.Click("登录"); err != nil {
		t.Fatal(err)
	}
	if u.loginError != "请勿频繁登录" || u.loginBusy {
		t.Fatal("disabled login button accepted a click")
	}
	// Enter and automatic login share the same guard as a button click.
	u.beginLoginWith("alice", "secret", "正在登录…")
	u.loginUser, u.loginPassword, u.rememberPassword = "alice", "secret", true
	if u.loginBusy || u.autoLoginIfSaved() {
		t.Fatal("another login path bypassed the cooldown")
	}
	u.loginUser, u.loginPassword = "", ""
	tester.Advance(9 * time.Second)
	if !tester.HasText("登录（1s）") || u.loginBusy {
		t.Fatal("countdown did not update or retried automatically")
	}
	tester.Advance(time.Second)
	if tester.HasText("请等待") || u.loginCooldownSeconds(tester.Now()) != 0 {
		t.Fatalf("countdown did not expire: %v", tester.Texts())
	}
	if err := tester.Click("登录"); err != nil {
		t.Fatal(err)
	}
	if u.loginError != "请输入账号和密码" {
		t.Fatal("login button was not enabled after expiry")
	}
	if r, ok := tester.Find("登录"); !ok || r.Y < 0 || r.Y+r.H > 640 {
		t.Fatalf("login button outside minimum window: %+v", r)
	}
}

func TestLoginRateLimitRestartsFullCooldown(t *testing.T) {
	start := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for _, message := range []string{"请勿频繁登录", "登录请求失败: BYR HTTP 429 Too Many Requests"} {
		t.Run(message, func(t *testing.T) {
			u := newUI(false)
			u.startLoginCooldown(start)
			responseAt := start.Add(20 * time.Second)
			u.loginBusy = true
			u.recordLoginFailure(errors.New(message), responseAt)
			if u.loginBusy || u.loginCooldownSeconds(responseAt) != 10 {
				t.Fatal("slow rate-limit response did not start a full cooldown")
			}
			if u.loginCooldownSeconds(responseAt.Add(9500*time.Millisecond)) != 1 || u.loginCooldownSeconds(responseAt.Add(10*time.Second)) != 0 {
				t.Fatal("cooldown rounded down or expired at the wrong time")
			}
		})
	}
	u := newUI(false)
	u.startLoginCooldown(start)
	u.recordLoginFailure(errors.New("密码错误"), start.Add(2*time.Second))
	if u.loginCooldownSeconds(start.Add(2*time.Second)) != 8 {
		t.Fatal("ordinary failure unnecessarily restarted the cooldown")
	}
}

func TestLoginCooldownSurvivesRestart(t *testing.T) {
	t.Setenv("BYR_CLIENT_HOME", t.TempDir())
	now := time.Now().Truncate(time.Millisecond)
	u := newUI(true)
	u.startLoginCooldown(now)
	restored := newUI(true)
	if restored.loginCooldownSeconds(now.Add(2*time.Second)) != 8 {
		t.Fatalf("persisted cooldown lost: %v", restored.loginRetryAt)
	}
	if got := appdata.LoadSettings().LoginRetryAfter; got != now.Add(loginCooldown).UnixMilli() {
		t.Fatalf("wrong persisted retry time: %d", got)
	}
	// A successful session or demo workspace reset must not remove the login guard.
	restored.resetWorkspace()
	if restored.loginCooldownSeconds(now.Add(2*time.Second)) != 8 {
		t.Fatal("workspace reset removed login cooldown")
	}
}

func TestDarkBackgroundChoicePersistsAndMatchesExport(t *testing.T) {
	t.Setenv("BYR_CLIENT_HOME", t.TempDir())
	u := newUI(true)
	t.Cleanup(u.closeApp)
	u.themeMode = 2
	neutral := u.exportTheme()
	for _, color := range []ui.Color{neutral.Background, neutral.Surface, neutral.Border, sidebarColor(&neutral, 0)} {
		if color.R != color.G || color.G != color.B {
			t.Fatalf("neutral background has a color cast: %+v", color)
		}
	}
	if neutral.Background.R > 20 {
		t.Fatalf("default dark background is too light: %+v", neutral.Background)
	}
	u.region = regionSettings
	u.startDemo()
	u.region = regionSettings
	tester := ui.NewTester(u.view, 1320, 1100)
	if err := tester.Click("蓝灰"); err != nil {
		t.Fatal(err)
	}
	if u.darkBackground != 1 || u.exportTheme().Background != ui.Hex("#171C24") {
		t.Fatal("background selector did not change the palette")
	}
	restored := newUI(true)
	if restored.darkBackground != 1 || restored.exportTheme().Background != u.exportTheme().Background {
		t.Fatal("background choice did not survive restart or export")
	}
}
