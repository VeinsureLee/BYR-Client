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
	case "user", "self", "history", "blacklist":
		return "self"
	}
	return page
}

func (a *App) shell(c *ui.Context) {
	if a.Core.Session().Token == "" {
		ui.Column(c).Fill().Children(func() {
			a.statusView(c)
			a.loginView(c)
		})
		return
	}
	ui.Row(c).Fill().Children(func() {
		a.navigationView(c)
		ui.Divider(c)
		ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
			ui.Row(c).FillWidth().Padding(14, 24).Gap(12).Background(c.Theme().Surface).Children(func() {
				if len(a.back) > 0 {
					a.action(c, "返回", "back", quiet, a.goBack)
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, "北邮人论坛  /  "+a.breadcrumb()).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine()
					ui.Text(c, a.title()).FontSize(20).Bold().MaxLines(1).Ellipsis("…")
				})
				if a.route.Page != "compose" && a.route.Page != "mail-compose" {
					a.action(c, "刷新", "refresh", quiet, a.load)
				}
				ui.Text(c, a.Version).FontSize(11).TextColor(c.Theme().TextMuted)
			})
			ui.Divider(c)
			a.statusView(c)
			ui.Column(c.Key(string(a.route.Page))).Grow(1).FillWidth().MinWidth(0).Padding(20, 24).Gap(14).Children(func() {
				if a.pageError != "" {
					a.pageErrorView(c)
					return
				}
				if a.pageLoading {
					ui.Column(c).Fill().Center().Gap(16).Children(func() {
						ui.Spinner(c).Size(26, 26)
						ui.Text(c, a.message).TextColor(c.Theme().TextMuted)
					})
					return
				}
				a.page(c)
			})
		})
	})
}

func (a *App) navigationView(c *ui.Context) {
	ui.Column(c).Width(104).Shrink(0).FillHeight().Background(c.Theme().Surface).Padding(10, 10).Gap(3).Children(func() {
		ui.Column(c).FillWidth().Padding(8, 10, 14).Gap(1).Children(func() {
			ui.Text(c, "BYR").FontSize(22).Bold().TextColor(c.Theme().Accent)
			ui.Text(c, "北邮人论坛").FontSize(10).TextColor(c.Theme().TextMuted)
		})
		active := navigationGroup(a.route.Page)
		if a.route.Page == "thread" && len(a.back) > 0 {
			active = navigationGroup(a.back[0].Page)
		}
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(3).Children(func() {
				for _, item := range navigation {
					e := ui.ButtonBase(c.Key("nav-"+item.page)).Label(item.label).FillWidth().Padding(8, 0).
						Gap(4).AlignItems(ui.Center).Radius(10).Selected(active == item.page).Disabled(a.busy)
					fg := c.Theme().TextMuted
					if active == item.page {
						e.Background(accentSurface(c))
						fg = c.Theme().Accent
					} else if e.Hovered() {
						e.Background(c.Theme().SurfaceHover)
					}
					e.TextColor(fg).Children(func() {
						ui.Icon(c, icons[item.icon]).Size(21, 21)
						ui.Text(c, item.label).FontSize(12)
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
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				a.avatar(c, forum.User{ID: forum.Scalar(a.Core.Session().Username)}, 30)
				ui.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
					ui.Text(c, a.Core.Session().Username).FontSize(12).Bold().MaxLines(1).Ellipsis("…")
					ui.Text(c, "已登录").FontSize(10).TextColor(c.Theme().TextMuted)
				})
			})
			settings := ui.ButtonBase(c).Label("设置").FillWidth().Row().Justify(ui.Start).
				AlignItems(ui.Center).Padding(8, 5).Gap(7).Radius(8).Disabled(a.busy)
			if settings.Hovered() {
				settings.Background(c.Theme().SurfaceHover)
			}
			settings.TextColor(c.Theme().TextMuted).Children(func() {
				ui.Icon(c, icons["settings"]).Size(17, 17).Shrink(0)
				ui.Text(c, "设置").FontSize(12)
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
	if a.message == "" || a.pageError != "" || a.pageLoading {
		return
	}
	color, icon := c.Theme().Success, "check"
	if a.busy {
		color, icon = c.Theme().Accent, "clock"
	} else if a.failed {
		color, icon = c.Theme().Danger, "alert"
	}
	ui.Column(c).FillWidth().Padding(9, 24).Gap(4).
		Background(c.Theme().Surface.Mix(color, .07)).TextColor(color).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons[icon]).Size(16, 16).Shrink(0)
			ui.Text(c, a.message).Selectable().Grow(1).MaxLines(3)
			if !a.busy {
				if control(c, "关闭提示", "close", quiet).Clicked() {
					a.message = ""
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
			ui.Text(c, a.diagnostic).FontSize(11).TextColor(c.Theme().TextMuted).FillWidth().Selectable()
		}
	})
}
