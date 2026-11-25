package desktop

import (
	"context"
	"fmt"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/egoist/mygo/ui"
)

func (a *App) addAccount() {
	if a.busy {
		return
	}
	a.username, a.password = "", ""
	a.remember = true
	a.navigate(Route{Page: "login", Name: "add-account"}, true)
}

func (a *App) switchAccount(username string) {
	if a.busy {
		return
	}
	a.run("切换账号", func(ctx context.Context) (func(), error) {
		err := a.Core.SwitchAccount(ctx, username)
		return func() {
			a.resetAccountUI()
			a.notice("已切换账号：" + username)
		}, err
	})
}

func (a *App) forgetAccount(username string) {
	if a.busy {
		return
	}
	a.run("移除保存账号", func(ctx context.Context) (func(), error) {
		err := a.Core.ForgetAccount(ctx, username)
		return func() {
			if !a.Core.SessionInfo().Authenticated {
				a.resetAccountUI()
				return
			}
			a.load()
			a.notice("已移除账号：" + username)
		}, err
	})
}

func (a *App) resetAccountUI() {
	a.cancelThreadPreflight()
	a.clearImages()
	a.imageView = ""
	a.imageScale = 1
	a.username, a.password = "", ""
	a.editor = editor{}
	a.mailEditor = mailEditor{}
	a.sections = nil
	a.section = forum.Section{}
	a.board = forum.Board{}
	a.thread = forum.Thread{}
	a.threadModels = nil
	a.threadLoadedRoute = Route{}
	a.threadLoadedRequest = Route{}
	a.threadAnchorID = ""
	a.threadViewport = viewportKey{}
	a.threadForceTop = false
	a.threadRefreshClearSub = false
	a.articles = nil
	a.boards = nil
	a.favorites = forum.Favorite{}
	a.collection = forum.Collection{}
	a.searchQuery = ""
	a.searchResults = nil
	a.searchPagination = forum.Pagination{}
	a.searchHasRun = false
	a.user = forum.User{}
	a.savedAccounts = nil
	a.blacklist = forum.Blacklist{}
	a.refers = forum.Refers{}
	a.mailbox = forum.Mailbox{}
	a.mail = forum.Mail{}
	a.contacts = nil
	a.chat = nil
	a.expandedMessages = make(map[string]bool)
	a.expandedPosts = make(map[listKey]map[string]bool)
	a.pinnedExpanded = make(map[listKey]bool)
	a.vote = forum.Vote{}
	a.bet = forum.Bet{}
	a.pageError, a.message = "", ""
	a.historyNotice = ""
	a.failed = false
	a.back = nil
	a.confirm = nil
	a.listStates = make(map[listKey]*ui.ListState)
	a.homeScrolls = make(map[string]*ui.ScrollState)
	if !a.Core.SessionInfo().Authenticated {
		a.route = Route{Page: "login"}
		return
	}
	a.route = Route{Page: "home"}
	a.load()
}

func (a *App) accountsView(c *ui.Context) {
	ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, "保存的论坛账号").FontSize(16).Bold()
			ui.Text(c, fmt.Sprintf("%d 个账号 · 凭据仅存储在系统凭据库中", len(a.savedAccounts))).
				FontSize(12).TextColor(c.Theme().TextMuted)
		})
		a.action(c, "添加账号", "plus", filled, a.addAccount)
	})
	if len(a.savedAccounts) == 0 {
		emptyState(c, "user", "没有保存的账号", "添加账号并启用“记住登录”，之后即可在账号间切换。")
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			for _, item := range a.savedAccounts {
				account := item
				ui.Column(c.Key("saved-account-"+account.Username)).FillWidth().Padding(12, 14).Gap(8).
					Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).Children(func() {
					ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
						a.avatar(c, forum.User{ID: forum.Scalar(account.Username)}, 34)
						ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
							ui.Text(c, account.Username).FontSize(14).Bold().MaxLines(1).Ellipsis("…")
							status := "已保存"
							if !account.Remembered {
								status = "当前会话 · 未记住"
							} else if account.Active {
								status = "当前账号"
							}
							ui.Text(c, status).FontSize(11).TextColor(c.Theme().TextMuted)
						})
						if !account.Active {
							a.action(c, "切换", "user", quiet, func() { a.switchAccount(account.Username) })
						}
						if account.Remembered {
							a.action(c, "移除", "trash", destructive, func() {
								a.confirm = &confirmation{text: "从此设备移除已保存的账号 “" + account.Username + "”？", run: func() {
									a.forgetAccount(account.Username)
								}}
							})
						}
					})
				})
			}
		})
	})
}
