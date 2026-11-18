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
			if a.route.Page != "search" {
				return
			}
			a.searchResults = result.Articles
			a.searchPagination = result.Pagination
		}, err
	})
}

func (a *App) searchView(c *ui.Context) {
	ui.Column(c).FillWidth().FillHeight().Gap(12).Children(func() {
		ui.Column(c).FillWidth().Padding(18).Gap(10).
			Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
			ui.Row(c).FillWidth().Wrap().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, "搜索论坛").FontSize(22).Bold()
					ui.Text(c, "按标题关键词搜索全站主题，结果可直接打开帖子").
						FontSize(13).TextColor(c.Theme().TextMuted)
				})
				badge(c, "全站搜索", c.Theme().Accent)
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.End).Children(func() {
				ui.TextInput(c, &a.searchQuery).Label("搜索关键词").
					Placeholder("输入标题关键词").Grow(1).Disabled(a.busy)
				a.action(c, "开始搜索", "search", filled, a.loadSearch)
			})
		})
		a.searchComingSoon(c, "AI 搜索", "AI 搜索需要额外的模型服务，当前版本暂未配置。", "hot")
		if !a.searchHasRun {
			emptyState(c, "search", "输入关键词开始搜索", "搜索论坛主题标题，结果会显示版面、作者和回复数。")
			return
		}
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "搜索结果").FontSize(16).Bold()
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
	})
}

func (a *App) searchComingSoon(c *ui.Context, title, detail, icon string) {
	ui.Row(c).FillWidth().Padding(16, 18).Gap(14).AlignItems(ui.Center).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
		ui.Column(c).Size(44, 44).Center().Shrink(0).Radius(12).
			Background(accentSurface(c)).Children(func() {
			ui.Icon(c, icons[icon]).Size(22, 22).TextColor(c.Theme().Accent)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(16).Bold()
				badge(c, "敬请期待", c.Theme().Warning)
			})
			ui.Text(c, detail).FontSize(12).TextColor(c.Theme().TextMuted)
		})
		control(c, "暂未启用", "clock", quiet).Disabled(true)
	})
}
