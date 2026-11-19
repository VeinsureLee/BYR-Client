package desktop

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/resources/fonts"
	"github.com/egoist/mygo/ui"
)

type profileField struct {
	label string
	value string
}

func userProfileFields(u forum.User) []profileField {
	fields := []profileField{
		{label: "在线状态", value: profileFlag(bool(u.IsOnline), "在线", "离线")},
		{label: "注册状态", value: profileFlag(bool(u.IsRegister), "已注册", "未注册")},
		{label: "生命力", value: string(u.Life)},
		{label: "性别", value: u.Gender},
		{label: "星座", value: u.Astro},
		{label: "主页", value: u.HomePage},
		{label: "QQ", value: string(u.QQ)},
		{label: "MSN", value: u.MSN},
	}
	if !bool(u.IsHide) {
		fields = append(fields,
			profileField{label: "上次登录时间", value: formatTime(u.LastLoginTime)},
			profileField{label: "最后登录 IP", value: u.LastLoginIP},
		)
	}
	fields = append(fields,
		profileField{label: "关注数", value: fmt.Sprintf("%d", u.FollowNum)},
		profileField{label: "粉丝数", value: fmt.Sprintf("%d", u.FansNum)},
		profileField{label: "是否已关注", value: profileFlag(bool(u.IsFollow), "是", "否")},
		profileField{label: "是否为粉丝", value: profileFlag(bool(u.IsFan), "是", "否")},
		profileField{label: "发帖数", value: fmt.Sprintf("%d", u.PostCount)},
		profileField{label: "积分", value: fmt.Sprintf("%d", u.Score)},
		profileField{label: "等级", value: string(u.Level)},
	)
	return fields
}

func profileFlag(value bool, yes, no string) string {
	if value {
		return yes
	}
	return no
}

func profileRow(c *ui.Context, field profileField) {
	if strings.TrimSpace(field.label) == "" || strings.TrimSpace(field.value) == "" {
		return
	}
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Start).Children(func() {
		ui.Text(c, field.label).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Text(c, field.value).Grow(1).MinWidth(0).MaxLines(3).Ellipsis("…")
	})
}

func (a *App) userView(c *ui.Context) {
	u := a.user
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.Column(c).FillWidth().Padding(18).Gap(12).
				Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
				ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Start).Children(func() {
					a.avatar(c, u, 72)
					ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
						ui.Text(c, u.DisplayName()).FontSize(22).Bold()
						meta := []string{}
						if u.ID != "" {
							meta = append(meta, "ID "+string(u.ID))
						}
						if len(meta) > 0 {
							ui.Text(c, strings.Join(meta, " · ")).FontSize(12).TextColor(c.Theme().TextMuted)
						}
						ui.Row(c).Wrap().Gap(6).Children(func() {
							badge(c, fmt.Sprintf("发帖 %d", u.PostCount), c.Theme().Accent)
							if u.Score != 0 {
								badge(c, fmt.Sprintf("积分 %d", u.Score), c.Theme().Success)
							}
							if u.Level != "" {
								badge(c, "等级 "+string(u.Level), c.Theme().Warning)
							}
							if bool(u.IsOnline) {
								badge(c, "在线", c.Theme().Success)
							}
						})
					})
				})
				for _, field := range userProfileFields(u) {
					profileRow(c, field)
				}
				ui.Divider(c)
				ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
					if a.route.Page == "self" {
						for _, item := range []struct {
							page        Page
							label, icon string
						}{
							{"favorites", "我的收藏", "star"},
							{"history", "阅读历史", "clock"},
							{"blacklist", "黑名单", "user"},
							{"mailbox", "我的信箱", "mail"},
							{"settings", "设置", "settings"},
						} {
							item := item
							if control(c, item.label, item.icon, quiet).Clicked() {
								a.navigate(Route{Page: item.page}, true)
							}
						}
						ui.Spacer(c)
						a.action(c, "退出登录", "back", destructive, a.logout)
					} else {
						if !u.Anonymous() && u.ID != "" {
							a.action(c, "发私信", "send", filled, func() {
								a.navigate(Route{Page: "mail-compose", Name: string(u.ID)}, true)
							})
							a.action(c, "加入黑名单", "alert", destructive, func() {
								a.confirm = &confirmation{text: "将 " + u.DisplayName() + " 加入黑名单？", run: func() {
									a.run("加入黑名单", func(ctx context.Context) (func(), error) {
										_, err := a.Core.Forum.AddBlacklist(ctx, string(u.ID))
										return func() { a.notice("已加入黑名单") }, err
									})
								}}
							})
						}
						if u.HomePage != "" {
							a.action(c, "个人主页", "link", quiet, func() {
								a.run("打开主页", func(ctx context.Context) (func(), error) { return nil, a.device.Open(u.HomePage) })
							})
						}
					}
				})
			})
		})
	})
}
func (a *App) logout() {
	a.confirm = &confirmation{text: "退出当前账号？已保存的草稿和历史会保留。", run: func() {
		a.run("退出登录", func(ctx context.Context) (func(), error) {
			err := a.Core.Logout(ctx)
			return func() {
				a.clearImages()
				a.username, a.password = "", ""
				a.editor = editor{}
				a.mailEditor = mailEditor{}
				a.sections = nil
				a.section = forum.Section{}
				a.board = forum.Board{}
				a.thread = forum.Thread{}
				a.articles = nil
				a.searchQuery = ""
				a.searchResults = nil
				a.searchPagination = forum.Pagination{}
				a.searchHasRun = false
				a.boards = nil
				a.contacts = nil
				a.chat = nil
				a.back = nil
				a.route = Route{Page: "login"}
			}, err
		})
	}}
}
func (a *App) historyView(c *ui.Context) {
	account := a.Core.Session().Username
	entries := a.Core.Local.History(account)
	ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, "最近阅读").FontSize(16).Bold()
			ui.Textf(c, "保留最近 %d 条阅读记录", len(entries)).FontSize(12).TextColor(c.Theme().TextMuted)
		})
		a.action(c, "清空阅读历史", "trash", destructive, func() {
			a.confirm = &confirmation{text: "清空此账号的本地阅读历史？", run: func() {
				if err := a.Core.Local.ClearHistory(context.Background(), account); err != nil {
					a.fail(err)
				}
			}}
		})
	})
	if len(entries) == 0 {
		emptyState(c, "clock", "还没有阅读记录", "打开主题后会在这里留下最近阅读记录。")
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(7).Children(func() {
			for _, e := range entries {
				e := e
				ui.Column(c.Key("history-"+e.Board+"-"+e.ID)).FillWidth().Padding(11, 13).Gap(4).
					Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(10).Children(func() {
					textLink(c, e.Title, func() {
						a.navigate(Route{Page: "thread", Name: e.Board, ID: e.ID}, true)
					}, a.busy)
					ui.Text(c, e.Board).FontSize(12).TextColor(c.Theme().TextMuted)
				})
			}
		})
	})
}
func (a *App) blacklistView(c *ui.Context) {
	ui.Column(c).FillWidth().Padding(12).Gap(8).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).Children(func() {
		ui.Text(c, "添加黑名单").FontSize(14).Bold()
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.TextInput(c, &a.forward).Label("拉黑用户名").Placeholder("用户名").Grow(1)
			a.action(c, "添加到黑名单", "plus", filled, func() {
				id := strings.TrimSpace(a.forward)
				if id == "" {
					return
				}
				a.run("添加黑名单", func(ctx context.Context) (func(), error) {
					_, err := a.Core.Forum.AddBlacklist(ctx, id)
					return func() { a.forward = ""; a.load() }, err
				})
			})
		})
	})
	if len(a.blacklist.Users) == 0 {
		emptyState(c, "user", "黑名单为空", "被拉黑的用户会显示在这里。")
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(7).Children(func() {
			for _, u := range a.blacklist.Users {
				u := u
				ui.Row(c.Key("blocked-"+string(u.ID))).FillWidth().Padding(11, 13).Gap(10).
					Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(10).
					AlignItems(ui.Center).Children(func() {
					a.avatar(c, u, 30)
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, u.DisplayName()).Bold().MaxLines(1).Ellipsis("…")
						if u.ID != "" {
							ui.Text(c, string(u.ID)).FontSize(11).TextColor(c.Theme().TextMuted)
						}
					})
					a.action(c, "移出黑名单", "close", destructive, func() {
						a.run("移出黑名单", func(ctx context.Context) (func(), error) {
							_, err := a.Core.Forum.RemoveBlacklist(ctx, string(u.ID))
							return func() { a.load() }, err
						})
					})
				})
			}
		})
	})
	a.pager(c, a.blacklist.Pagination, a.pageNumber)
}
func (a *App) settingsView(c *ui.Context) {
	p := a.Core.Local.Preferences()
	save := func() {
		if err := a.Core.Local.SetPreferences(context.Background(), p); err != nil {
			a.fail(err)
		}
	}
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.Column(c).FillWidth().Padding(16).Gap(10).
				Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
				ui.Text(c, "外观").FontSize(16).Bold()
				ui.Text(c, "主题").FontSize(12).TextColor(c.Theme().TextMuted)
				ui.Row(c).FillWidth().Wrap().Gap(12).Children(func() {
					for i, label := range []string{"跟随系统", "浅色", "深色"} {
						if ui.Radio(c, &p.Theme, i, label).Changed() {
							save()
						}
					}
				})
				ui.Divider(c)
				ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "字号").Grow(1)
					a.action(c, "减小字号", "back", quiet, func() {
						p.FontScale -= .1
						save()
					})
					ui.Textf(c, "%.0f%%", p.FontScale*100).FontSize(13).Bold()
					a.action(c, "增大字号", "next", quiet, func() {
						p.FontScale += .1
						save()
					})
				})
			})
			ui.Column(c).FillWidth().Padding(16).Gap(10).
				Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
				ui.Text(c, "网络").FontSize(16).Bold()
				if ui.Checkbox(c, &p.IPv6, "使用 IPv6 论坛地址").Changed() {
					save()
				}
				ui.Text(c, "切换后会在下一次请求生效。").FontSize(12).TextColor(c.Theme().TextMuted)
			})
			ui.Column(c).FillWidth().Padding(16).Gap(10).
				Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
				ui.Text(c, "关于").FontSize(16).Bold()
				ui.Text(c, "BYR Client · "+a.Version).FontSize(12).TextColor(c.Theme().TextMuted)
				ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
					a.action(c, "查看论坛发布信息", "clock", quiet, func() {
						a.run("查询发布信息", func(ctx context.Context) (func(), error) {
							v, err := a.Core.Forum.Version(ctx, false)
							return func() {
								var text strings.Builder
								for _, version := range v.Versions {
									fmt.Fprintf(&text, "%s\n%s\n%s\n", version.Version, version.Changelog, version.URL)
								}
								a.notice(text.String())
							}, err
						})
					})
					a.action(c, "字体许可证", "message", quiet, func() { a.infoText = fonts.License })
					ui.Spacer(c)
					a.action(c, "退出登录", "back", destructive, a.logout)
				})
			})
		})
	})
}
func (a *App) voteView(c *ui.Context) {
	v := a.vote
	ui.Column(c).FillWidth().Padding(16).Gap(10).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, v.Title).FontSize(18).Bold().Grow(1)
			if bool(v.IsEnd) {
				badge(c, "已结束", c.Theme().Danger)
			} else {
				badge(c, "进行中", c.Theme().Success)
			}
		})
		if v.End != "" {
			ui.Text(c, "截止："+string(v.End)).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		for _, option := range v.Options {
			ui.Row(c).FillWidth().Padding(8, 10).Gap(8).
				Background(c.Theme().SurfaceHover).Radius(8).Children(func() {
				ui.Radio(c, &a.choice, string(option.ID), option.Label).
					Disabled(a.busy || bool(v.IsEnd) || bool(v.IsDeleted))
				ui.Spacer(c)
				ui.Textf(c, "%d 票", option.Num).FontSize(12).TextColor(c.Theme().TextMuted)
			})
		}
		if ui.PrimaryButton(c, "提交投票").Disabled(a.busy || a.choice == "" || bool(v.IsEnd) || bool(v.IsDeleted)).Clicked() {
			id, choice := a.route.ID, a.choice
			a.run("提交投票", func(ctx context.Context) (func(), error) {
				updated, err := a.Core.Forum.SubmitVote(ctx, id, choice)
				return func() { a.vote = updated; a.notice("投票已提交") }, err
			})
		}
	})
}
func (a *App) betView(c *ui.Context) {
	v := a.bet
	ui.Column(c).FillWidth().Padding(16).Gap(10).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
		ui.Text(c, v.Title).FontSize(18).Bold()
		if v.Description != "" {
			ui.Text(c, v.Description).FontSize(13).TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).Wrap().Gap(6).Children(func() {
			if v.End != "" {
				badge(c, "截止 "+string(v.End), c.Theme().Warning)
			}
			badge(c, fmt.Sprintf("可用积分 %d", v.Score), c.Theme().Accent)
		})
		for _, item := range v.Items {
			ui.Row(c).FillWidth().Padding(8, 10).Gap(8).
				Background(c.Theme().SurfaceHover).Radius(8).Children(func() {
				ui.Radio(c, &a.choice, string(item.ID), item.Label).Disabled(a.busy)
				ui.Spacer(c)
				ui.Text(c, string(item.Percent)).FontSize(12).TextColor(c.Theme().TextMuted)
			})
		}
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.End).Children(func() {
			ui.TextInput(c, &a.score).Label("竞猜积分").Placeholder("积分").Width(180).Disabled(a.busy)
			if ui.PrimaryButton(c, "提交竞猜").Disabled(a.busy || a.choice == "").Clicked() {
				id, choice := a.route.ID, a.choice
				score, err := strconv.Atoi(a.score)
				if err != nil || score <= 0 {
					a.notice("请输入大于 0 的积分")
					return
				}
				a.run("提交竞猜", func(ctx context.Context) (func(), error) {
					updated, err := a.Core.Forum.SubmitBet(ctx, id, choice, score)
					return func() { a.bet = updated; a.notice("竞猜已提交") }, err
				})
			}
		})
	})
}
