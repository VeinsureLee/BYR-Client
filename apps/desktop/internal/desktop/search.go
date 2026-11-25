package desktop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/egoist/mygo/ui"
)

func (a *App) loadSearch() {
	query := strings.TrimSpace(a.searchQuery)
	if query == "" {
		a.notice("请输入搜索关键词")
		return
	}
	user := a.searchMode == "user"
	sameQuery := !user && a.searchHasRun && a.searchSubmittedQuery == query
	if !sameQuery && a.route.Number != 1 {
		a.route.Number = 1
	}
	page := max(1, a.route.Number)
	route := a.route
	if user {
		a.searchHasRun = true
		a.searchSubmittedQuery = query
		a.searchResults = nil
		a.searchPagination = forum.Pagination{}
		a.searchIncomplete = false
		a.searchFailedBoards = 0
		a.searchRetryQuery = ""
		a.searchRetryPage = 0
		a.loadData("加载用户资料", func(ctx context.Context) (func(), error) {
			user, err := a.Core.LookupUser(ctx, query)
			if errors.Is(err, application.ErrStaleSession) {
				err = context.Canceled
			}
			return func() {
				if a.route != route || strings.TrimSpace(a.searchQuery) != query || a.searchMode != "user" {
					return
				}
				a.showLoadedUser(query, user)
			}, err
		})
		return
	}
	a.searchHasRun = true
	a.searchSubmittedQuery = query
	a.searchRetryQuery = ""
	a.searchRetryPage = 0
	if !sameQuery {
		a.searchResults = nil
		a.searchPagination = forum.Pagination{}
		a.searchIncomplete = false
		a.searchFailedBoards = 0
		a.searchRetryQuery = ""
		a.searchRetryPage = 0
	}
	a.searchRetryQuery, a.searchRetryPage = query, page
	work := func(ctx context.Context) (func(), error) {
		result, err := a.Core.Search(ctx, application.SearchRequest{
			Keyword: query,
			Page:    page,
			Size:    30,
		})
		return func() {
			if a.route.Page != "search" || a.route.Number != page || a.searchSubmittedQuery != query {
				return
			}
			a.searchResults = result.Articles
			a.searchPagination = result.Pagination
			a.searchIncomplete = result.Incomplete
			a.searchFailedBoards = result.FailedBoards
			a.searchRetryQuery = ""
			a.searchRetryPage = 0
		}, err
	}
	if sameQuery {
		a.pageError = ""
		a.retryLoad = func() { a.loadSearchPageFor(query, page) }
		a.run("搜索全站主题", work)
		return
	}
	a.loadData("搜索全站主题", work)
}

// loadSearchPage requests another page while the previous page remains the
// displayed route. Only a successful response changes the route number,
// results and list state; retry keeps the same submitted query and target.
func (a *App) loadSearchPage(page int) {
	a.loadSearchPageFor(a.searchSubmittedQuery, page)
}

// loadSearchPageFor pins the submitted keyword with the target page. An edit in
// the search field without a new submission must not suppress the retry or
// accept a response for another query.
func (a *App) loadSearchPageFor(query string, page int) {
	if a.busy || a.route.Page != "search" || a.searchMode == "user" {
		return
	}
	query = strings.TrimSpace(query)
	if query == "" || !a.searchHasRun || a.searchSubmittedQuery != query {
		return
	}
	if page < 1 {
		page = 1
	}
	a.searchRetryQuery = query
	a.searchRetryPage = page
	a.pageError = ""
	a.retryLoad = func() { a.loadSearchPageFor(query, page) }
	a.run("搜索全站主题", func(ctx context.Context) (func(), error) {
		result, err := a.Core.Search(ctx, application.SearchRequest{
			Keyword: query,
			Page:    page,
			Size:    30,
		})
		return func() {
			if a.route.Page != "search" || a.searchSubmittedQuery != query || a.searchMode != "all" {
				return
			}
			a.route.Number = page
			a.desktopListState(a.articleListKey()).ScrollTo(0, ui.Start)
			a.searchResults = result.Articles
			a.searchPagination = result.Pagination
			a.searchIncomplete = result.Incomplete
			a.searchFailedBoards = result.FailedBoards
			a.searchRetryQuery = ""
			a.searchRetryPage = 0
			a.retryLoad = nil
		}, err
	})
}

func (a *App) retrySearch() {
	query := a.searchSubmittedQuery
	if query == "" {
		a.loadSearch()
		return
	}
	a.loadSearchPageFor(query, max(1, a.route.Number))
}

// refreshSearch reloads page one of the submitted keyword without changing the
// displayed page until the new page commits. The editor draft, result page and
// scroll position stay intact while the request waits or fails.
func (a *App) refreshSearch() {
	if a.searchMode == "user" || !a.searchHasRun || a.searchSubmittedQuery == "" {
		a.refreshing = true
		a.loadSearch()
		return
	}
	a.refreshing = true
	a.loadSearchPageFor(a.searchSubmittedQuery, 1)
}

func (a *App) clearSearch() {
	for key, state := range a.listStates {
		if key.Page == "search" {
			*state = ui.ListState{}
		}
	}
	a.searchQuery = ""
	a.searchResults = nil
	a.searchPagination = forum.Pagination{}
	a.searchHasRun = false
	a.searchSubmittedQuery = ""
	a.searchIncomplete = false
	a.searchFailedBoards = 0
	a.searchRetryQuery = ""
	a.searchRetryPage = 0
	a.route.Number = 1
	a.failed = false
	a.message = ""
	a.pageError = ""
	a.retryLoad = nil
	a.diagnostic = ""
}

func (a *App) searchClearControl(c *ui.Context) {
	e := ui.ButtonBase(c.Key("search-clear")).Label("清空搜索").Row().AlignItems(ui.Center).
		Justify(ui.Center).Padding(8, 12).Gap(7).Radius(9).MinWidth(0).Disabled(a.busy)
	if e.Hovered() {
		e.Background(c.Theme().SurfaceHover)
	}
	e.TextColor(c.Theme().TextMuted).Children(func() {
		ui.Icon(c, icons["close"]).Size(17, 17).Shrink(0)
		ui.Text(c, "清空搜索").SingleLine()
	})
	if e.Clicked() {
		a.clearSearch()
	}
}

// switchSearchMode cancels the in-flight request and invalidates its callback
// before clearing search state, so a delayed success, error or refresh notice
// cannot land on the new mode.
func (a *App) switchSearchMode(mode string) {
	if (mode != "all" && mode != "user") || a.searchMode == mode {
		return
	}
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.serial++
	a.busy = false
	a.pageLoading = false
	a.refreshing = false
	a.failed = false
	a.message = ""
	a.historyNotice = ""
	a.pageError = ""
	a.retryLoad = nil
	a.diagnostic = ""
	a.searchMode = mode
	a.searchHasRun = false
	a.searchSubmittedQuery = ""
	a.searchResults = nil
	a.searchPagination = forum.Pagination{}
	a.searchIncomplete = false
	a.searchFailedBoards = 0
	a.searchRetryQuery = ""
	a.searchRetryPage = 0
	a.route.Number = 1
	a.pageJump = ""
}

func (a *App) searchView(c *ui.Context) {
	user := a.searchMode == "user"
	ui.Row(c).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).
		Padding(0, 0).Children(func() {
		if control(c, "全站搜索", "search", buttonToneForSearch(!user)).Clicked() {
			a.switchSearchMode("all")
		}
		if control(c, "人物", "user", buttonToneForSearch(user)).Clicked() {
			a.switchSearchMode("user")
		}
		themedTextInput(c, &a.searchQuery).Label("搜索关键词").
			Placeholder(func() string {
				if user {
					return "用户名或论坛 ID"
				}
				return "标题关键词"
			}()).Grow(1).Disabled(a.busy)
		if strings.TrimSpace(a.searchQuery) != "" {
			a.searchClearControl(c)
		}
		a.action(c, "开始搜索", "search", filled, a.loadSearch)
	})
	if !a.searchHasRun {
		if a.busy {
			emptyState(c, "search", "正在搜索…", "请稍候，搜索结果会显示在这里。")
			return
		}
		if a.failed {
			emptyState(c, "alert", "搜索失败", "请检查关键词或网络后重试。")
			return
		}
		emptyState(c, "search", "输入关键词开始搜索", "可按主题标题或完整用户名/论坛 ID 查找。")
		return
	}
	if user {
		if a.busy {
			emptyState(c, "user", "正在查找人物…", "请稍候。")
		} else if a.failed {
			emptyState(c, "alert", "查找失败", "请检查完整用户名或论坛 ID 后重试。")
		} else {
			emptyState(c, "user", "未找到用户", "检查完整用户名或论坛 ID 后重试。")
		}
		return
	}
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, "搜索结果").FontSize(15).Bold()
			ui.Text(c, fmt.Sprintf("关键词：%s", a.searchSubmittedQuery)).
				FontSize(12).TextColor(c.Theme().TextMuted)
		})
		if a.searchPagination.TotalItems > 0 {
			badge(c, fmt.Sprintf("%d 条", a.searchPagination.TotalItems), c.Theme().Accent)
		}
	})
	if len(a.searchResults) == 0 {
		if a.busy {
			emptyState(c, "search", "正在搜索…", "请稍候，搜索结果会显示在这里。")
		} else if a.failed {
			emptyState(c, "alert", "搜索失败", "请使用上方重试，或调整关键词。")
		} else if a.searchIncomplete {
			a.searchIncompleteEmpty(c)
		} else {
			emptyState(c, "search", "没有找到主题", "换一个标题关键词试试。")
		}
		return
	}
	if a.searchIncomplete {
		a.searchIncompleteNotice(c)
	}
	a.articleList(c, a.searchResults)
	a.pager(c, a.searchPagination, func(page int) {
		a.loadSearchPage(page)
	})
}

func (a *App) searchIncompleteNotice(c *ui.Context) {
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Padding(6, 8).
		Background(c.Theme().Surface.Mix(c.Theme().Warning, .12)).Radius(8).Children(func() {
		ui.Icon(c, icons["alert"]).Size(16, 16).TextColor(c.Theme().Warning).Shrink(0)
		ui.Text(c, fmt.Sprintf("有 %d 个版面加载失败，结果可能不完整。", max(1, a.searchFailedBoards))).
			FontSize(12).TextColor(c.Theme().TextMuted).Grow(1)
		if control(c, "重试", "refresh", quiet).Clicked() {
			a.retrySearch()
		}
	})
}

func (a *App) searchIncompleteEmpty(c *ui.Context) {
	ui.Column(c).Grow(1).FillWidth().FillHeight().Center().Padding(32).Gap(10).Children(func() {
		ui.Icon(c, icons["alert"]).Size(30, 30).TextColor(c.Theme().Warning)
		ui.Text(c, "搜索未完整").Bold().TextColor(c.Theme().TextMuted)
		ui.Text(c, fmt.Sprintf("有 %d 个版面加载失败，暂未取得结果。", max(1, a.searchFailedBoards))).
			TextColor(c.Theme().TextMuted)
		if control(c, "重试搜索", "refresh", quiet).Clicked() {
			a.retrySearch()
		}
	})
}

func buttonToneForSearch(active bool) buttonTone {
	if active {
		return selected
	}
	return quiet
}
