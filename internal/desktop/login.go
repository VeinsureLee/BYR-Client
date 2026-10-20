package desktop

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"BYR-Client/internal/adapter/byr"
	svc "BYR-Client/internal/app"
	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
	"BYR-Client/internal/platform/appdata"
	"BYR-Client/internal/ports"
	"BYR-Client/internal/source/demo"
	"BYR-Client/internal/store/memory"
)

func byrBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("BYR_BASE_URL")); value != "" {
		return value
	}
	return "https://bbs.byr.cn"
}

func (u *UI) loginView(c *ui.Context) {
	t := c.Theme()
	remaining := u.loginCooldownSeconds(c.Now())
	if remaining > 0 {
		c.After(min(time.Second, u.loginRetryAt.Sub(c.Now())))
	}
	_, height := c.Size()
	ui.Scroll(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).FillWidth().MinHeight(height).Padding(24).Center().Children(func() {
			card := ui.Column(c).Width(460).Padding(30).Gap(13).Radius(14).
				Background(t.Surface).Border(1, t.Border).Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.12)).
				Children(func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						u.appLogo(c, 42)
						ui.Column(c).Gap(1).Children(func() {
							ui.Text(c, "BYR Client").Bold().FontSize(u.fs(16))
							ui.Text(c, "北邮人论坛桌面客户端").FontSize(u.fs(11)).TextColor(t.TextMuted)
						})
					})
					ui.Text(c, "登录北邮人论坛").FontSize(u.fs(23)).Bold()
					ui.Text(c, "登录后浏览版面、阅读讨论，也可以搜索帖子或向 AI 提问。").
						FontSize(u.fs(12)).TextColor(t.TextMuted).LineHeight(1.5)

					enter := false
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "账号").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
						if ui.TextInput(c, &u.loginUser).Placeholder("论坛用户 ID").Label("账号").Disabled(u.loginBusy).Submitted() {
							enter = true
						}
					})
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "密码").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
						if ui.TextInput(c, &u.loginPassword).Password().Placeholder("论坛密码").Label("密码").
							Disabled(u.loginBusy).Submitted() {
							enter = true
						}
					})

					if u.loginError != "" {
						ui.Text(c, u.loginError).FontSize(u.fs(11)).TextColor(t.Danger)
					}
					if ui.Checkbox(c, &u.rememberLogin, "保持登录（保存会话 Cookie，下次免登录）").Changed() {
						u.saveSettings()
						if !u.rememberLogin {
							_ = appdata.ClearSession()
						}
					}
					if ui.Checkbox(c, &u.rememberPassword, "保存账号密码（系统凭证，下次自动登录）").Changed() {
						u.saveSettings()
						if u.rememberPassword {
							if u.hasSavedCredentials() {
								u.saveSavedCredentials(u.loginUser, u.loginPassword)
							}
						} else {
							u.clearSavedCredentials()
						}
					}
					label := "登录"
					if remaining > 0 {
						label = fmt.Sprintf("登录（%ds）", remaining)
					}
					ui.PrimaryButton(c, label).Label("登录").FillWidth().MinHeight(38).
						Disabled(u.loginBusy || remaining > 0).OnClick(u.beginLogin)
					if u.loginBusy {
						ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
							ui.Spinner(c)
							ui.Text(c, u.loginStatus).FontSize(u.fs(12)).TextColor(t.TextMuted).Grow(1)
						})
					}
					if remaining > 0 {
						ui.Textf(c, "请等待 %ds 后再登录", remaining).FontSize(u.fs(12)).TextColor(t.TextMuted)
					}
					ui.Text(c, "登录信息仅保存在本机。").FontSize(u.fs(12)).TextColor(t.TextMuted)
					ui.Divider(c)
					ui.Button(c, "先使用演示数据").FillWidth().Disabled(u.loginBusy).OnClick(u.startDemo)
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "离线查看界面与检索流程").FontSize(u.fs(12)).TextColor(t.TextMuted).Grow(1)
						ui.Text(c, byrBaseURL()).FontSize(u.fs(12)).TextColor(t.TextMuted).SingleLine().Shrink(0)
					})
					if enter {
						u.beginLogin()
						c.Invalidate()
					}
				})
			card.Transition(pageTransition())
		})
	})
}

func (u *UI) beginLogin() {
	u.beginLoginWith(u.loginUser, u.loginPassword, "正在登录…")
}

func (u *UI) beginLoginWith(username, password, status string) {
	now := u.loginTime()
	if u.loginBusy || u.closed || u.loginCooldownSeconds(now) > 0 {
		return
	}
	user := strings.TrimSpace(username)
	if user == "" || password == "" {
		u.loginError = "请输入账号和密码"
		return
	}
	win := u.window()
	u.loginBusy = true
	u.loginError = ""
	u.loginStatus = status
	u.settings.LastUser = user
	u.startLoginCooldown(now)
	sessionFile := appdata.SessionPath()
	if !u.rememberLogin {
		sessionFile = ""
		_ = appdata.ClearSession()
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		client := byr.New(byr.Config{BaseURL: byrBaseURL(), SessionFile: sessionFile})
		application := svc.NewWithAuth(client, memory.New(), client, ports.SystemClock{}, 10*time.Minute)
		keep := false
		defer func() {
			if !keep {
				application.Close()
			}
		}()

		if _, err := application.Login(ctx, user, password); err != nil {
			u.loginFailed(win, err)
			return
		}
		u.setLoginStatus(win, "正在初始化论坛结构…")
		if err := application.Initialize(ctx); err != nil {
			u.loginFailed(win, err)
			return
		}
		if win == nil {
			return
		}
		application.SetNotify(u.invalidate)
		application.Start()
		keep = true

		win.Update(func() {
			if u.closed {
				application.Close()
				return
			}
			u.app = application
			u.settings.LastUser = user
			u.saveSettings()
			if u.rememberPassword {
				u.saveSavedCredentials(user, password)
			} else {
				u.clearSavedCredentials()
			}
			u.loginBusy = false
			u.loginStatus = ""
			u.loginError = ""
			u.loginPassword = ""
			u.resetWorkspace()
			u.region = regionBrowse
		})
	}()
}

func (u *UI) loginFailed(win *mygo.Window, err error) {
	if win == nil {
		return
	}
	win.Update(func() {
		u.recordLoginFailure(err, u.loginTime())
	})
}

func (u *UI) setLoginStatus(win *mygo.Window, status string) {
	if win == nil {
		return
	}
	win.Update(func() { u.loginStatus = status })
}

func (u *UI) startDemo() {
	if u.closed || u.loginBusy {
		return
	}
	application := svc.New(demo.New(time.Now()), memory.New(), ports.SystemClock{}, 10*time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := application.Initialize(ctx); err != nil {
		u.loginError = err.Error()
		application.Close()
		return
	}
	application.SetNotify(u.invalidate)
	application.Start()
	u.app = application
	u.loginBusy = false
	u.loginStatus = ""
	u.loginError = ""
	u.loginPassword = ""
	u.resetWorkspace()
	u.region = regionBrowse
}

func (u *UI) logout() {
	application := u.app
	if application == nil || u.loginBusy {
		return
	}
	win := u.window()
	u.loginBusy = true
	u.loginStatus = "正在退出登录…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = application.Logout(ctx)
		application.Close()
		if win == nil {
			return
		}
		win.Update(func() {
			u.app = nil
			u.loginBusy = false
			u.loginStatus = ""
			u.loginPassword = ""
			u.loginError = ""
			u.resetWorkspace()
			u.loadSavedCredentials()
		})
	}()
}

func (u *UI) resetWorkspace() {
	u.region = regionBrowse
	u.returnRegion = regionBrowse
	u.returnValid = false
	u.browseNode = ""
	u.browseBoard = ""
	u.browseThread = domain.ArticleKey{}
	u.treeOpen = make(map[forum.NodeID]bool)
	u.replyOpen = make(map[string]bool)
	u.lastHighlightFloor = 0
	u.activeHistory = ""
	u.keywordHistory = ""
	u.activeRun = ""
	u.run = svc.RunSnapshot{}
	u.sourceKey = nil
	u.sourceFloor = 0
	u.collapsed = make(map[string]bool)
	u.resultScroll = ui.ScrollState{}
	u.aiScroll = ui.ScrollState{}
	u.boardViews = nil
	u.readingViews = nil
	u.directoryViews = nil
	u.searchViews = nil
	u.directoryTree = ui.ScrollState{}
	u.listOnly = false

	u.searchMode = 0
	u.keywordTokens = nil
	u.keywordDraft = ""
	u.author = ""
	u.timeKind = "不限"
	u.scopeLabel = "全站"
	u.scopeExtraLabel = ""
	u.scopeExtraNode = ""
	u.showDetails = false
	u.perBoard = domain.DefaultPerBoardLimit
	u.total = domain.DefaultTotalLimit
	u.threadPages = domain.DefaultThreadPages
	u.concurrency = domain.DefaultConcurrency
	u.searchError = ""
	u.undoHint = false

	u.stopAIContext()
	u.aiQuestion = ""
	u.aiAnswer = ""
	u.aiError = ""
	u.aiStatus = ""
	u.aiTrace = nil
	u.aiSources = nil
	u.chat = nil
	u.chatSeq = 0
	u.activeChat = 0
	u.aiRunning = false
	u.aiRunToken++
	u.aiHistoryID = ""
	u.aiTesting = false
	u.aiTestStatus = ""
	u.aiTimeKind = "近一个月"
	u.aiScope = "全站"
	u.aiDetails = false
	u.aiTotal = 50
	u.aiPerBoard = 10
	u.images = make(map[string]*imageState)
	for _, win := range append([]*mygo.Window(nil), u.browserWindows...) {
		win.Close()
	}
	u.browserWindows = nil
	u.previewImage = domain.Image{}
	u.previewZoom = 0
	u.previewOpen = false
}

func (u *UI) refreshCatalog() {
	application := u.app
	if application == nil || u.loginBusy {
		return
	}
	win := u.window()
	u.loginBusy = true
	u.loginStatus = "正在刷新论坛目录…"
	u.loginError = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		err := application.Initialize(ctx)
		if win == nil {
			return
		}
		win.Update(func() {
			u.loginBusy = false
			u.loginStatus = ""
			if err != nil {
				u.loginError = err.Error()
				return
			}
			u.browseNode = ""
			u.browseBoard = ""
			u.browseThread = domain.ArticleKey{}
			u.collapsed = make(map[string]bool)
			u.treeOpen = make(map[forum.NodeID]bool)
		})
	}()
}

func (u *UI) tryRestoreSession() {
	if u.closed {
		return
	}
	if !u.rememberLogin {
		_ = appdata.ClearSession()
	}
	win := u.window()
	if win == nil {
		return
	}
	if _, err := os.Stat(appdata.SessionPath()); err != nil || !u.rememberLogin {
		u.autoLoginIfSaved()
		return
	}
	u.loginBusy = true
	u.loginStatus = "正在恢复登录…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		client := byr.New(byr.Config{BaseURL: byrBaseURL(), SessionFile: appdata.SessionPath()})
		application := svc.NewWithAuth(client, memory.New(), client, ports.SystemClock{}, 10*time.Minute)
		keep := false
		defer func() {
			if !keep {
				application.Close()
			}
		}()
		session, ok, err := application.RestoreSession(ctx)
		if err != nil {
			u.loginFailed(win, err)
			return
		}
		if !ok {
			_ = appdata.ClearSession()
			win.Update(func() {
				u.loginBusy = false
				u.loginStatus = ""
				u.autoLoginIfSaved()
			})
			return
		}
		u.setLoginStatus(win, "正在初始化论坛结构…")
		if err := application.Initialize(ctx); err != nil {
			u.loginFailed(win, err)
			return
		}
		application.SetNotify(u.invalidate)
		application.Start()
		keep = true
		win.Update(func() {
			if u.closed {
				application.Close()
				return
			}
			u.app = application
			u.settings.LastUser = session.AccountID
			u.saveSettings()
			u.loginBusy = false
			u.loginStatus = ""
			u.loginError = ""
			u.resetWorkspace()
			u.region = regionBrowse
		})
	}()
}

func (u *UI) autoLoginIfSaved() bool {
	if u.closed || u.loginBusy || u.loginCooldownSeconds(u.loginTime()) > 0 || !u.rememberPassword || !u.hasSavedCredentials() {
		return false
	}
	u.beginLoginWith(u.loginUser, u.loginPassword, "正在使用保存的账号登录…")
	return true
}

func (u *UI) setApplicationActive(active bool) {
	if application := u.app; application != nil {
		application.SetApplicationActive(active)
	}
}

func (u *UI) closeApp() {
	u.closed = true
	if application := u.app; application != nil {
		application.Close()
		u.app = nil
	}
}

func (u *UI) sessionLine() (string, string) {
	if u.app == nil {
		return "", ""
	}
	session := u.app.Session()
	if !session.LoggedIn {
		return "未登录", ""
	}
	name := session.DisplayName
	if name == "" {
		name = session.AccountID
	}
	if session.AccountID == "demo" {
		return name, "演示模式"
	}
	return name, fmt.Sprintf("%s · 会话代次 %d", session.AccountID, session.Epoch)
}
