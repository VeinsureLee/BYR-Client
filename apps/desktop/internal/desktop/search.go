package desktop

import (
	"context"
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
	page := max(1, a.route.Number)
	a.route.Number = page
	route := a.route
	if a.searchMode == "user" {
		a.searchHasRun = true
		a.searchResults = nil
		a.searchPagination = forum.Pagination{}
		a.loadData("加载用户资料", func(ctx context.Context) (func(), error) {
			user, err := a.Core.Forum.User(ctx, query)
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
	a.searchResults = nil
	a.searchPagination = forum.Pagination{}
	a.loadData("搜索全站主题", func(ctx context.Context) (func(), error) {
		result, err := a.Core.Search(ctx, application.SearchRequest{
			Keyword: query,
			Page:    page,
			Size:    30,
		})
		return func() {
			if a.route.Page != "search" || a.route.Number != page || strings.TrimSpace(a.searchQuery) != query {
				return
			}
			a.searchResults = result.Articles
			a.searchPagination = result.Pagination
		}, err
	})
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

func (a *App) searchView(c *ui.Context) {
	user := a.searchMode == "user"
	ui.Row(c).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).
		Padding(0, 0).Children(func() {
		if control(c, "全站搜索", "search", buttonToneForSearch(!user)).Clicked() {
			a.searchMode = "all"
			a.searchHasRun = false
			a.searchResults = nil
			a.searchPagination = forum.Pagination{}
		}
		if control(c, "人物", "user", buttonToneForSearch(user)).Clicked() {
			a.searchMode = "user"
			a.searchHasRun = false
			a.searchResults = nil
			a.searchPagination = forum.Pagination{}
		}
		ui.TextInput(c, &a.searchQuery).Label("搜索关键词").
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
		emptyState(c, "search", "输入关键词开始搜索", "可按主题标题或完整用户名/论坛 ID 查找。")
		return
	}
	if user {
		emptyState(c, "user", "未找到用户", "检查完整用户名或论坛 ID 后重试。")
		return
	}
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, "搜索结果").FontSize(15).Bold()
			ui.Text(c, fmt.Sprintf("关键词：%s", strings.TrimSpace(a.searchQuery))).
				FontSize(12).TextColor(c.Theme().TextMuted)
		})
		if a.searchPagination.TotalItems > 0 {
			badge(c, fmt.Sprintf("%d 条", a.searchPagination.TotalItems), c.Theme().Accent)
		}
	})
	if len(a.searchResults) == 0 {
		emptyState(c, "search", "没有找到主题", "换一个标题关键词试试。")
		return
	}
	a.articleList(c, a.searchResults)
	a.pager(c, a.searchPagination, func(page int) {
		a.route.Number = page
		a.loadSearch()
	})
}

func buttonToneForSearch(active bool) buttonTone {
	if active {
		return selected
	}
	return quiet
}
