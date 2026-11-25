package desktop

import (
	"context"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/egoist/mygo/ui"
)

var navigation = []struct {
	page        Page
	label, icon string
}{
	{"home", "首页", "home"}, {"top", "十大", "hot"}, {"timeline", "时间线", "clock"},
	{"favorites", "收藏", "star"}, {"compose", "发帖", "plus"}, {"search", "搜索", "search"},
	{"contacts", "消息", "message"}, {"self", "我", "user"},
}

func navigationGroup(page Page) Page {
	switch page {
	case "home", "board", "thread":
		return "home"
	case "contacts", "chat", "mailbox", "mail", "mail-compose", "refers":
		return "contacts"
	case "user", "self", "history", "blacklist", "accounts", "settings":
		return "self"
	}
	return page
}

func (a *App) shell(c *ui.Context) {
	session := a.Core.SessionInfo()
	if !session.Authenticated {
		ui.Column(c).Fill().Children(func() {
			a.statusView(c)
			if a.route.Page == "accounts" {
				a.accountsView(c)
			} else {
				a.loginView(c)
			}
		})
		return
	}
	margin := a.pageMargin()
	ui.Row(c).Fill().Children(func() {
		a.navigationView(c)
		ui.Divider(c)
		ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
			ui.Row(c).FillWidth().Padding(12, margin).Gap(12).Background(c.Theme().Surface).Children(func() {
				if len(a.back) > 0 {
					back := control(c, "返回", "back", quiet).Disabled(a.busy && !a.pageLoading)
					if back.Clicked() {
						a.goBack()
					}
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, "北邮人论坛  /  "+a.breadcrumb()).FontSize(a.textSize(c, 11)).TextColor(c.Theme().TextMuted).SingleLine()
					ui.Text(c, a.title()).FontSize(a.textSize(c, 20)).FontWeight(600).MaxLines(1).Ellipsis("…")
				})
				if a.route.Page != "compose" && a.route.Page != "mail-compose" {
					a.action(c, "刷新", "refresh", quiet, a.refreshCurrent)
				}
				ui.Text(c, a.Version).FontSize(a.textSize(c, 11)).TextColor(c.Theme().TextMuted)
			})
			ui.Divider(c)
			ui.Column(c.Key(string(a.route.Page))).Grow(1).FillWidth().MinWidth(0).Padding(16, margin).Gap(14).Children(func() {
				if a.pageError != "" {
					a.pageErrorView(c)
					return
				}
				if a.pageLoading && !a.hasLoadedThreadContent() {
					ui.Column(c).Fill().Center().Gap(16).Children(func() {
						ui.Spinner(c).Size(26, 26)
						ui.Text(c, a.message).TextColor(c.Theme().TextMuted)
					})
					return
				}
				a.page(c)
				if a.route.Page == "thread" {
					if a.hasLoadedThreadContent() {
						a.threadFeedbackSlot(c)
					}
				} else if a.pageLoading {
					ui.Box(c).Absolute().Bottom(56).Left(0).Right(0).Children(func() { a.threadLoadingStrip(c) })
				} else if a.message != "" || a.historyNotice != "" {
					ui.Box(c).Absolute().Bottom(56).Left(0).Right(0).Children(func() { a.statusView(c) })
				}
			})
		})
	})
}

const threadFeedbackHeight float32 = 72

func (a *App) threadFeedbackSlot(c *ui.Context) {
	ui.Box(c).FillWidth().Height(threadFeedbackHeight).Children(func() {
		if a.pageLoading {
			a.threadLoadingSlot(c)
		} else if a.message != "" || a.historyNotice != "" {
			a.threadStatusView(c)
		}
	})
}

func (a *App) threadLoadingStrip(c *ui.Context) {
	a.renderThreadLoading(c, false)
}

func (a *App) threadLoadingSlot(c *ui.Context) {
	a.renderThreadLoading(c, true)
}

func (a *App) renderThreadLoading(c *ui.Context, fillHeight bool) {
	content := func() {
		ui.Spinner(c).Size(16, 16)
		ui.Text(c, a.message).FontSize(a.textSize(c, 12)).TextColor(c.Theme().TextMuted)
	}
	if fillHeight {
		ui.Row(c).FillWidth().FillHeight().Padding(8, 12).Gap(8).AlignItems(ui.Center).
			Background(c.Theme().Background).Radius(8).Children(content)
		return
	}
	ui.Row(c).FillWidth().Padding(8, 12).Gap(8).AlignItems(ui.Center).
		Background(c.Theme().Background).Radius(8).Children(content)
}

func (a *App) pageMargin() float32 {
	switch {
	case a.windowWidth > 0 && a.windowWidth < 1000:
		return 16
	case a.windowWidth > 0 && a.windowWidth < 1440:
		return 24
	default:
		return 32
	}
}

func (a *App) navWidth() float32 {
	switch {
	case a.windowWidth > 0 && a.windowWidth < 1000:
		return 64
	case a.windowWidth > 0 && a.windowWidth < 1440:
		return 176
	default:
		return 184
	}
}

func (a *App) compactNavigation() bool { return a.windowWidth > 0 && a.windowWidth < 1000 }

func (a *App) navigationView(c *ui.Context) {
	compact := a.compactNavigation()
	ui.Column(c).Width(a.navWidth()).Shrink(0).FillHeight().Background(c.Theme().Surface).Padding(8, 8).Gap(3).Children(func() {
		ui.Column(c).FillWidth().Padding(6, 8, 12).Gap(1).Children(func() {
			brandSize, brand := float32(22), "BYR"
			if compact {
				brandSize, brand = 16, "B"
			}
			ui.Text(c, brand).FontSize(a.textSize(c, brandSize)).FontWeight(600).
				TextColor(c.Theme().Accent).SingleLine()
			if !compact {
				ui.Text(c, "北邮人论坛").FontSize(a.textSize(c, 10)).TextColor(c.Theme().TextMuted)
			}
		})
		active := navigationGroup(a.route.Page)
		if a.route.Page == "thread" && len(a.back) > 0 {
			active = navigationGroup(a.back[0].Page)
		}
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(3).Children(func() {
				for _, item := range navigation {
					e := ui.ButtonBase(c.Key("nav-"+item.page)).Label(item.label).FillWidth().
						Padding(8, 0).Gap(6).MinHeight(40).Radius(8).Selected(active == item.page).Disabled(a.busy)
					if compact {
						e.AlignItems(ui.Center).Justify(ui.Center)
					} else {
						e.AlignItems(ui.Center)
					}
					e.Tooltip(item.label)
					fg := c.Theme().TextMuted
					if active == item.page {
						e.Background(accentSurface(c))
						fg = c.Theme().Accent
					} else if e.Hovered() {
						e.Background(c.Theme().SurfaceHover)
					}
					e.TextColor(fg).Children(func() {
						ui.Icon(c, icons[item.icon]).Size(22, 22)
						if !compact {
							ui.Text(c, item.label).FontSize(a.textSize(c, 14)).FontWeight(500)
						}
					})
					if e.Clicked() {
						r := Route{Page: item.page}
						if item.page == "compose" && (a.route.Page == "board" || a.route.Page == "thread") {
							r.Name = a.route.Name
						}
						a.navigate(r, false)
					}
				}
			})
		})
		ui.Divider(c)
		ui.Column(c).FillWidth().Padding(4, 8, 8).Gap(6).Children(func() {
			userRow := ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center)
			if compact {
				userRow = userRow.Justify(ui.Center)
			}
			userRow.Children(func() {
				a.avatar(c, forum.User{ID: forum.Scalar(a.Core.SessionInfo().Username)}, 30)
				if !compact {
					ui.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
						ui.Text(c, a.Core.SessionInfo().Username).FontSize(a.textSize(c, 12)).FontWeight(600).MaxLines(1).Ellipsis("…")
						ui.Text(c, "已登录").FontSize(a.textSize(c, 10)).TextColor(c.Theme().TextMuted)
					})
				}
			})
			settings := ui.ButtonBase(c).Label("设置").FillWidth().Row().
				AlignItems(ui.Center).Padding(8, 5).Gap(7).MinHeight(36).Radius(8).Disabled(a.busy)
			if compact {
				settings.Justify(ui.Center)
			} else {
				settings.Justify(ui.Start)
			}
			settings.Tooltip("设置")
			if settings.Hovered() {
				settings.Background(c.Theme().SurfaceHover)
			}
			settings.TextColor(c.Theme().TextMuted).Children(func() {
				ui.Icon(c, icons["settings"]).Size(19, 19).Shrink(0)
				if !compact {
					ui.Text(c, "设置").FontSize(a.textSize(c, 14))
				}
			})
			if settings.Clicked() {
				a.navigate(Route{Page: "settings"}, false)
			}
		})
	})
}
func (a *App) breadcrumb() string {
	switch a.route.Page {
	case "thread", "board":
		return a.route.Name
	case "compose":
		if a.editor.EditID != "" {
			return "编辑帖子"
		}
		if a.editor.ReplyID != "" {
			return "回复帖子"
		}
		return "写点什么"
	case "home":
		return "发现版面"
	case "top":
		return "热门讨论"
	case "timeline":
		return "全部帖子 · 按时间"
	case "favorites":
		return "我的收藏"
	case "search":
		return "搜索论坛"
	default:
		return "我的社区"
	}
}

func (a *App) statusView(c *ui.Context) {
	a.renderStatusView(c, false)
}

func (a *App) threadStatusView(c *ui.Context) {
	a.renderStatusView(c, true)
}

func (a *App) renderStatusView(c *ui.Context, reserved bool) {
	message := a.message
	if message == "" {
		message = a.historyNotice
	}
	if message == "" || a.pageError != "" || a.pageLoading {
		return
	}
	color, icon := c.Theme().Success, "check"
	if a.busy {
		color, icon = c.Theme().Accent, "clock"
	} else if a.failed {
		color, icon = c.Theme().Danger, "alert"
	} else if a.message == "" && a.historyNotice != "" {
		color, icon = c.Theme().Warning, "alert"
	}
	content := func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons[icon]).Size(16, 16).Shrink(0)
			if reserved {
				ui.Text(c, message).Selectable().Grow(1).MaxLines(1).Ellipsis("…")
			} else {
				ui.Text(c, message).Selectable().Grow(1).MaxLines(3)
			}
			if !a.busy {
				if a.failed && a.retryLoad != nil {
					if control(c, "重试", "refresh", quiet).Clicked() {
						a.failed = false
						a.message = ""
						a.retryLoad()
					}
				}
				if control(c, "关闭提示", "close", quiet).Clicked() {
					a.message = ""
					a.historyNotice = ""
					a.diagnostic = ""
				}
				if a.diagnostic != "" && a.device != nil {
					if control(c, "复制诊断", "copy", quiet).Clicked() {
						detail := a.diagnostic
						a.run("复制诊断", func(ctx context.Context) (func(), error) {
							return func() { a.notice("诊断信息已复制") }, a.device.Copy(detail)
						})
					}
				}
			}
		})
		if a.failed && a.diagnostic != "" && a.message != a.diagnostic {
			if reserved {
				ui.Text(c, a.diagnostic).FontSize(11).TextColor(c.Theme().TextMuted).
					FillWidth().Selectable().MaxLines(1).Ellipsis("…")
			} else {
				ui.Text(c, a.diagnostic).FontSize(11).TextColor(c.Theme().TextMuted).FillWidth().Selectable()
			}
		}
	}
	if reserved {
		ui.Column(c).FillWidth().FillHeight().Padding(9, 24).Gap(4).
			Background(c.Theme().Surface.Mix(color, .07)).TextColor(color).Children(content)
		return
	}
	ui.Column(c).FillWidth().Padding(9, 24).Gap(4).
		Background(c.Theme().Surface.Mix(color, .07)).TextColor(color).Children(content)
}
