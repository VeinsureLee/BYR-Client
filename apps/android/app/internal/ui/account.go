package ui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
	"github.com/VeinsureLee/BYR-Client/core/resources/fonts"
)

func (a *App) openUser(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if !a.push(Route{Page: PageUser, Name: name}) {
		return
	}
	a.user = forum.User{}
	a.loadUser(name)
}

func (a *App) loadUser(name string) {
	a.run("加载用户资料", func(ctx context.Context) (func(), error) {
		u, err := a.core.Forum.User(ctx, name)
		return func() {
			if a.current().Page == PageUser && a.current().Name == name {
				a.user = u
			}
		}, err
	})
}

func (a *App) openHistory() {
	if !a.push(Route{Page: PageHistory}) {
		return
	}
	a.history = a.core.Local.History(a.core.Session().Username)
}

func (a *App) openBlacklist() {
	if !a.push(Route{Page: PageBlacklist, Number: 1}) {
		return
	}
	a.blacklist = forum.Blacklist{}
	a.loadBlacklist()
}

func (a *App) loadBlacklist() {
	page := max(1, a.current().Number)
	a.run("加载黑名单", func(ctx context.Context) (func(), error) {
		v, err := a.core.Forum.Blacklist(ctx, page)
		return func() { a.blacklist = v; a.pagination = v.Pagination }, err
	})
}

func (a *App) openSettings() {
	if !a.push(Route{Page: PageSettings}) {
		return
	}
	a.ipv6.Value = a.core.Local.Preferences().IPv6
}

func (a *App) openVote(id string) {
	if id == "" || !a.push(Route{Page: PageVote, ID: id}) {
		return
	}
	a.vote = forum.Vote{}
	a.choice = ""
	a.loadVote()
}

func (a *App) loadVote() {
	id := a.current().ID
	a.run("加载投票", func(ctx context.Context) (func(), error) {
		v, err := a.core.Forum.Vote(ctx, id)
		return func() { a.vote = v }, err
	})
}

func (a *App) openBet(id string) {
	if id == "" || !a.push(Route{Page: PageBet, ID: id}) {
		return
	}
	a.bet = forum.Bet{}
	a.choice = ""
	a.scoreIn.SetText("10")
	a.scoreIn.SingleLine = true
	a.loadBet()
}

func (a *App) loadBet() {
	id := a.current().ID
	a.run("加载竞猜", func(ctx context.Context) (func(), error) {
		v, err := a.core.Forum.Bet(ctx, id)
		return func() { a.bet = v }, err
	})
}

func (a *App) userPage(gtx layout.Context) layout.Dimensions {
	u := a.user
	if u.ID == "" && a.busy {
		return a.pendingPage(gtx, "正在加载用户资料…")
	}
	return material.List(a.theme, &a.auxList).Layout(gtx, 2, func(gtx layout.Context, i int) layout.Dimensions {
		switch i {
		case 0:
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.avatar(gtx, u, 64)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										l := material.Label(a.theme, unit.Sp(20), u.DisplayName())
										l.Font.Weight = 700
										return l.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										meta := []string{}
										if u.ID != "" {
											meta = append(meta, "ID "+string(u.ID))
										}
										if u.Gender != "" {
											meta = append(meta, u.Gender)
										}
										if u.Astro != "" {
											meta = append(meta, u.Astro)
										}
										if t := formatTime(u.LastLoginTime); t != "" {
											meta = append(meta, "上次登录 "+t)
										}
										l := material.Label(a.theme, unit.Sp(11), strings.Join(meta, " · "))
										l.Color = colorMuted
										return l.Layout(gtx)
									}),
									layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return a.badgeChip(gtx, fmt.Sprintf("发帖 %d", u.PostCount), colorAccentSoft, colorAccent)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return a.badgeChip(gtx, fmt.Sprintf("积分 %d", u.Score), colorWarningSoft, colorWarning)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												if u.Level == "" {
													return layout.Dimensions{}
												}
												return a.badgeChip(gtx, "等级 "+string(u.Level), colorAccentSoft, colorAccent)
											}),
										)
									}),
								)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.filledPill(gtx, "user-mail", "发私信", "send")
								if clicked {
									a.openMailCompose(string(u.ID), "", "", "send")
								}
								return dims
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.pill(gtx, "user-block", "加入黑名单", "alert", true, true)
								if clicked && !a.busy {
									id := string(u.ID)
									a.run("加入黑名单", func(ctx context.Context) (func(), error) {
										_, err := a.core.Forum.AddBlacklist(ctx, id)
										return func() { a.message = "已加入黑名单"; a.failed = false }, err
									})
								}
								return dims
							}),
						)
					}),
				)
			})
		default:
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(13), "用户详情")
						l.Font.Weight = 700
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						detail := []string{}
						if u.HomePage != "" {
							detail = append(detail, "主页 "+u.HomePage)
						}
						if u.QQ != "" {
							detail = append(detail, "QQ "+string(u.QQ))
						}
						if u.FansNum > 0 || u.FollowNum > 0 {
							detail = append(detail, fmt.Sprintf("关注 %d · 粉丝 %d", u.FollowNum, u.FansNum))
						}
						if len(detail) == 0 {
							detail = append(detail, "这个用户还没有填写更多资料。")
						}
						l := material.Label(a.theme, unit.Sp(12), strings.Join(detail, "\n"))
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
				)
			})
		}
	})
}

func (a *App) historyPage(gtx layout.Context) layout.Dimensions {
	entries := a.core.Local.History(a.core.Session().Username)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(16), "最近阅读")
								l.Font.Weight = 700
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(11), fmt.Sprintf("保留最近 %d 条阅读记录", len(entries)))
								l.Color = colorMuted
								return l.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dims, clicked := a.pill(gtx, "history-clear", "清空", "trash", true, true)
						if clicked && len(entries) > 0 && !a.busy {
							a.run("清空阅读历史", func(ctx context.Context) (func(), error) {
								err := a.core.Local.ClearHistory(ctx, a.core.Session().Username)
								return func() { a.history = nil }, err
							})
						}
						return dims
					}),
				)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(entries) == 0 {
				return a.empty(gtx, "还没有阅读记录")
			}
			return material.List(a.theme, &a.auxList).Layout(gtx, len(entries), func(gtx layout.Context, i int) layout.Dimensions {
				e := entries[i]
				btn := a.click("history:" + e.Board + ":" + e.ID)
				clicked := btn.Clicked(gtx)
				dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(14), e.Title)
									l.Font.Weight = 600
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(11), e.Board)
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						})
					})
				})
				if clicked {
					a.openThread(e.Board, e.ID)
				}
				return dims
			})
		}),
	)
}

func (a *App) blacklistPage(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(14), "添加黑名单")
						l.Font.Weight = 700
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.inputBox(gtx, &a.blacklistIn, "用户名")
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.filledPill(gtx, "blacklist-add", "添加", "plus")
								if clicked {
									id := strings.TrimSpace(a.blacklistIn.Text())
									if id == "" {
										a.fail(errors.New("请输入用户名"))
									} else if !a.busy {
										a.run("添加黑名单", func(ctx context.Context) (func(), error) {
											_, err := a.core.Forum.AddBlacklist(ctx, id)
											return func() { a.blacklistIn.SetText(""); a.loadBlacklist() }, err
										})
									}
								}
								return dims
							}),
						)
					}),
				)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(a.blacklist.Users) == 0 {
				return a.emptyOrPending(gtx, "黑名单为空")
			}
			return material.List(a.theme, &a.auxList).Layout(gtx, len(a.blacklist.Users), func(gtx layout.Context, i int) layout.Dimensions {
				u := a.blacklist.Users[i]
				return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.avatar(gtx, u, 36)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										l := material.Label(a.theme, unit.Sp(14), u.DisplayName())
										l.Font.Weight = 600
										return l.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										l := material.Label(a.theme, unit.Sp(11), string(u.ID))
										l.Color = colorMuted
										return l.Layout(gtx)
									}),
								)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.pill(gtx, "blacklist-remove:"+string(u.ID), "移出", "close", true, true)
								if clicked && !a.busy {
									id := string(u.ID)
									a.run("移出黑名单", func(ctx context.Context) (func(), error) {
										_, err := a.core.Forum.RemoveBlacklist(ctx, id)
										return func() { a.loadBlacklist() }, err
									})
								}
								return dims
							}),
						)
					})
				})
			})
		}),
		layout.Rigid(a.pager),
	)
}

func (a *App) settingsPage(gtx layout.Context) layout.Dimensions {
	p := a.core.Local.Preferences()
	return material.List(a.theme, &a.auxList).Layout(gtx, 3, func(gtx layout.Context, i int) layout.Dimensions {
		switch i {
		case 0:
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(15), "外观")
						l.Font.Weight = 700
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(11), "主题")
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.pill(gtx, "settings-theme:light", "浅色", "", p.Theme != 2, false)
								if clicked && !a.busy {
									p.Theme = 1
									a.savePreferences(p)
								}
								return dims
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.pill(gtx, "settings-theme:dark", "深色", "", p.Theme == 2, false)
								if clicked && !a.busy {
									p.Theme = 2
									a.savePreferences(p)
								}
								return dims
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(11), "字号")
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						sizes := []float32{0.9, 1.0, 1.15}
						labels := []string{"小", "标准", "大"}
						children := make([]layout.FlexChild, 0, 6)
						for index := range sizes {
							index := index
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.pill(gtx, fmt.Sprintf("settings-font:%d", index), labels[index], "", p.FontScale == sizes[index], false)
								if clicked && !a.busy {
									p.FontScale = sizes[index]
									a.savePreferences(p)
								}
								return dims
							}))
							children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout))
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(11), "字号会在确认后立即应用。")
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
				)
			})
		case 1:
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(15), "网络")
							l.Font.Weight = 700
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims := material.CheckBox(a.theme, &a.ipv6, "使用 IPv6 论坛地址").Layout(gtx)
							if a.ipv6.Value != p.IPv6 && !a.busy {
								p.IPv6 = a.ipv6.Value
								a.savePreferences(p)
							}
							return dims
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(11), "切换后会在下一次请求生效。")
							l.Color = colorMuted
							return l.Layout(gtx)
						}),
					)
				})
			})
		default:
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(15), "关于")
						l.Font.Weight = 700
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(11), "BYR Client Android · "+a.version+" · 共享 Go 核心")
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.pill(gtx, "settings-version", "论坛发布信息", "clock", false, false)
								if clicked && !a.busy {
									a.run("查询发布信息", func(ctx context.Context) (func(), error) {
										v, err := a.core.Forum.Version(ctx, false)
										return func() {
											var text strings.Builder
											for _, version := range v.Versions {
												fmt.Fprintf(&text, "%s：%s %s\n", version.Version, version.Changelog, version.URL)
											}
											a.message = strings.TrimSpace(text.String())
											a.failed = false
										}, err
									})
								}
								return dims
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.pill(gtx, "settings-license", "字体许可证", "link", false, false)
								if clicked {
									a.message = fonts.License
									a.failed = false
								}
								return dims
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dims, clicked := a.pill(gtx, "settings-logout", "退出登录", "back", true, true)
						if clicked {
							a.logout()
						}
						return dims
					}),
				)
			})
		}
	})
}

func (a *App) savePreferences(p local.Preferences) {
	if err := a.core.Local.SetPreferences(context.Background(), p); err != nil {
		a.fail(err)
		return
	}
	a.message = "设置已保存"
	a.failed = false
	a.window.Invalidate()
}

func (a *App) votePage(gtx layout.Context) layout.Dimensions {
	v := a.vote
	if v.ID == "" && a.busy {
		return a.pendingPage(gtx, "正在加载投票…")
	}
	return material.List(a.theme, &a.auxList).Layout(gtx, 2, func(gtx layout.Context, i int) layout.Dimensions {
		switch i {
		case 0:
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(18), v.Title)
								l.Font.Weight = 700
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if bool(v.IsEnd) {
									return a.badgeChip(gtx, "已结束", colorDangerSoft, colorDanger)
								}
								return a.badgeChip(gtx, "进行中", colorAccentSoft, colorAccent)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(11), fmt.Sprintf("截止 %s · %d 人参与", string(v.End), v.UserCount))
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
				)
			})
		default:
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, len(v.Options)+2)
					for _, option := range v.Options {
						option := option
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							selected := a.choice == string(option.ID)
							label := option.Label
							if bool(v.IsResultVoted) || bool(v.IsEnd) {
								label = fmt.Sprintf("%s · %d 票", option.Label, option.Num)
							}
							dims, clicked := a.pill(gtx, "vote:"+string(option.ID), label, "check", selected, false)
							if clicked && !a.busy && !bool(v.IsEnd) && !bool(v.IsDeleted) {
								a.choice = string(option.ID)
							}
							return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return dims
							})
						}))
					}
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := "提交投票"
								if a.choice == "" {
									label = "请选择选项"
								}
								dims, clicked := a.primaryButton(gtx, "vote-submit", label)
								if clicked && a.choice != "" && !a.busy && !bool(v.IsEnd) {
									id := a.current().ID
									choice := a.choice
									a.run("提交投票", func(ctx context.Context) (func(), error) {
										updated, err := a.core.Forum.SubmitVote(ctx, id, choice)
										return func() {
											a.vote = updated
											a.message = "投票已提交"
											a.failed = false
										}, err
									})
								}
								return dims
							}),
						)
					}))
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			})
		}
	})
}

func (a *App) betPage(gtx layout.Context) layout.Dimensions {
	v := a.bet
	if v.ID == "" && a.busy {
		return a.pendingPage(gtx, "正在加载竞猜…")
	}
	return material.List(a.theme, &a.auxList).Layout(gtx, 2, func(gtx layout.Context, i int) layout.Dimensions {
		switch i {
		case 0:
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(18), v.Title)
						l.Font.Weight = 700
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.Description == "" {
							return layout.Dimensions{}
						}
						l := material.Label(a.theme, unit.Sp(12), v.Description)
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.badgeChip(gtx, "截止 "+string(v.End), colorWarningSoft, colorWarning)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.badgeChip(gtx, fmt.Sprintf("可用积分 %d", v.Score), colorAccentSoft, colorAccent)
							}),
						)
					}),
				)
			})
		default:
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, len(v.Items)+3)
					for _, item := range v.Items {
						item := item
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							selected := a.choice == string(item.ID)
							dims, clicked := a.pill(gtx, "bet:"+string(item.ID),
								fmt.Sprintf("%s · %s", item.Label, string(item.Percent)), "check", selected, false)
							if clicked && !a.busy {
								a.choice = string(item.ID)
							}
							return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return dims
							})
						}))
					}
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.inputBox(gtx, &a.scoreIn, "竞猜积分")
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.primaryButton(gtx, "bet-submit", "提交竞猜")
							if clicked && !a.busy {
								score, err := strconv.Atoi(strings.TrimSpace(a.scoreIn.Text()))
								if a.choice == "" {
									a.fail(errors.New("请选择竞猜选项"))
								} else if err != nil || score <= 0 {
									a.fail(errors.New("请输入大于 0 的积分"))
								} else {
									id, choice := a.current().ID, a.choice
									a.run("提交竞猜", func(ctx context.Context) (func(), error) {
										updated, err := a.core.Forum.SubmitBet(ctx, id, choice, score)
										return func() {
											a.bet = updated
											a.message = "竞猜已提交"
											a.failed = false
										}, err
									})
								}
							}
							return dims
						}),
					)
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			})
		}
	})
}
