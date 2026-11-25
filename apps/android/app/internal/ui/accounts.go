package ui

import (
	"context"
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/application"
)

func (a *App) openAccountManager() {
	if a.current().Page == PageAccounts {
		a.loadAccounts()
		return
	}
	if a.push(Route{Page: PageAccounts}) {
		a.loadAccounts()
	}
}

func (a *App) loadAccounts() {
	if a.core == nil || a.current().Page != PageAccounts {
		return
	}
	route := a.current()
	a.run("加载账号列表", func(ctx context.Context) (func(), error) {
		accounts, err := a.core.SavedAccounts(ctx)
		return func() {
			if a.current() == route {
				a.savedAccounts = accounts
			}
		}, err
	})
}

func (a *App) addAccount() {
	if a.busy {
		return
	}
	a.username.SetText("")
	a.password.SetText("")
	a.remember.Value = true
	if a.push(Route{Page: PageLogin, Name: "add-account"}) {
		a.window.Invalidate()
	}
}

func (a *App) switchAccount(username string) {
	if a.busy || a.core == nil {
		return
	}
	a.run("切换账号", func(ctx context.Context) (func(), error) {
		err := a.core.SwitchAccount(ctx, username)
		return func() {
			a.resetSessionUI()
			a.showTopRoot()
			a.notice("已切换账号：" + username)
		}, err
	})
}

func (a *App) forgetAccount(username string) {
	if a.busy || a.core == nil {
		return
	}
	wasActive := applicationAccountKey(a.core.SessionInfo().Username) == applicationAccountKey(username)
	a.run("移除保存账号", func(ctx context.Context) (func(), error) {
		err := a.core.ForgetAccount(ctx, username)
		return func() {
			if wasActive {
				a.resetSessionUI()
				a.savedAccounts = nil
				return
			}
			a.savedAccounts = nil
			a.loadAccounts()
			a.notice("已移除账号：" + username)
		}, err
	})
}

func applicationAccountKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func (a *App) accountsPage(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "已登录账号", fmt.Sprintf("%d 个账号", len(a.savedAccounts)))
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				dims, clicked := a.primaryButton(gtx, "accounts-add", "添加账号")
				if clicked {
					a.addAccount()
				}
				return dims
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(a.savedAccounts) == 0 {
				if a.busy || a.failed {
					return a.emptyOrPending(gtx, "没有保存的账号")
				}
				return a.empty(gtx, "没有保存的账号")
			}
			return material.List(a.theme, &a.auxList).Layout(gtx, len(a.savedAccounts), func(gtx layout.Context, i int) layout.Dimensions {
				return a.savedAccountRow(gtx, a.savedAccounts[i])
			})
		}),
	)
}

func (a *App) savedAccountRow(gtx layout.Context, account application.SavedAccount) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.avatarCircle(gtx, account.Username, 34, colorAccentSoft, colorAccent)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(14), account.Username)
									l.Font.Weight = 600
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									status := "已保存"
									if !account.Remembered {
										status = "本次会话 · 未记住"
									} else if account.Active {
										status = "当前账号"
									}
									l := material.Caption(a.theme, status)
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						children := []layout.FlexChild{}
						if !account.Active {
							children = append(children,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									dims, clicked := a.pill(gtx, "accounts-switch:"+account.Username, "切换", "self", false, false)
									if clicked {
										a.switchAccount(account.Username)
									}
									return dims
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							)
						}
						if account.Remembered {
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dims, clicked := a.pill(gtx, "accounts-forget:"+account.Username, "移除", "close", false, true)
								if clicked {
									a.forgetAccount(account.Username)
								}
								return dims
							}))
						}
						if len(children) == 0 {
							return layout.Dimensions{}
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
					})
				}),
			)
		})
	})
}
