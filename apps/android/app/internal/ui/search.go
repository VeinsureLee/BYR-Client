package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func searchListKey(route Route, query string) listKey {
	return listKey{Page: PageSearch, Query: strings.TrimSpace(query), Number: max(1, route.Number)}
}

func (a *App) loadSearch() {
	if a.searchMode != "all" && a.searchMode != "user" {
		a.notice("AI 搜索尚未配置，当前仅支持全站搜索和人物搜索")
		return
	}
	query := strings.TrimSpace(a.searchEdit.Text())
	if query == "" {
		a.fail(errors.New("请输入搜索关键词"))
		return
	}
	route := a.current()
	page := max(1, route.Number)
	if a.searchMode == "user" {
		a.searchHasRun = true
		a.searchListQuery = query
		a.searchResults = nil
		a.searchPagination = forum.Pagination{}
		a.run("加载用户资料", func(ctx context.Context) (func(), error) {
			user, err := a.core.Forum.User(ctx, query)
			return func() {
				if a.current() != route || strings.TrimSpace(a.searchEdit.Text()) != query || a.searchMode != "user" {
					return
				}
				if !a.push(Route{Page: PageUser, Name: query}) {
					return
				}
				a.user = user
			}, err
		})
		return
	}
	key := searchListKey(route, query)
	a.searchHasRun = true
	a.searchListQuery = query
	a.searchResults = nil
	a.searchPagination = forum.Pagination{}
	a.run("搜索全站主题", func(ctx context.Context) (func(), error) {
		result, err := a.core.Search(ctx, application.SearchRequest{
			Keyword: query,
			Page:    page,
			Size:    30,
		})
		return func() {
			if a.current() != route || strings.TrimSpace(a.searchEdit.Text()) != query || a.searchMode != "all" {
				return
			}
			a.mobileList(key).Position = layout.Position{}
			a.searchResults = result.Articles
			a.searchPagination = result.Pagination
		}, err
	})
}

func (a *App) clearSearch() {
	if a.busy && a.cancel != nil {
		a.cancel()
	}
	for key, state := range a.listStates {
		if key.Page == PageSearch {
			state.Position = layout.Position{}
		}
	}
	a.searchEdit.SetText("")
	if len(a.stack) > 0 {
		a.stack[len(a.stack)-1].Number = 1
	}
	a.searchResults = nil
	a.searchPagination = forum.Pagination{}
	a.searchHasRun = false
	a.searchListQuery = ""
	a.failed = false
	a.message = ""
	a.window.Invalidate()
}

func (a *App) setSearchMode(mode string) {
	if mode != "ai" && mode != "all" && mode != "user" || a.searchMode == mode {
		return
	}
	if a.busy && a.cancel != nil {
		a.cancel()
	}
	a.searchMode = mode
	a.searchResults = nil
	a.searchPagination = forum.Pagination{}
	a.searchHasRun = false
	a.searchListQuery = ""
	a.failed = false
	a.message = ""
	a.window.Invalidate()
}

func (a *App) searchPage(gtx layout.Context) layout.Dimensions {
	route := a.current()
	state := a.mobileList(searchListKey(route, a.searchListQuery))
	all := a.searchMode == "all"
	user := a.searchMode == "user"
	items := []moduleTab{
		{Key: "ai", Label: "AI 搜索"},
		{Key: "all", Label: "全站搜索"},
		{Key: "user", Label: "人物"},
	}
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.moduleTabs(gtx, items, a.searchMode, a.setSearchMode)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				placeholder := "描述你想找的内容"
				if all {
					placeholder = "输入标题关键词"
				} else if user {
					placeholder = "输入完整用户名或论坛 ID"
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if event, ok := a.searchEdit.Update(gtx); ok {
							if _, submit := event.(widget.SubmitEvent); submit {
								a.loadSearch()
							}
						}
						return a.inputBox(gtx, &a.searchEdit, placeholder)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if a.searchEdit.Text() == "" {
							return layout.Dimensions{}
						}
						dims, clicked := a.iconButton(gtx, "search-clear", "close")
						if clicked {
							a.clearSearch()
						}
						return dims
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := "开始 AI 搜索"
				if all {
					label = "开始全站搜索"
				} else if user {
					label = "查找人物"
				}
				dims, clicked := a.primaryButton(gtx, "search-submit", label)
				if clicked {
					a.loadSearch()
				}
				return dims
			})
		},
	}
	if a.searchMode == "ai" {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return a.empty(gtx, "AI 搜索尚未配置")
		})
	} else if !a.searchHasRun {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			hint := "按主题标题搜索全站内容"
			if user {
				hint = "输入完整论坛 ID 或用户名查找人物"
			}
			return a.sectionTitle(gtx, hint, "")
		})
	} else if user {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return a.emptyOrPending(gtx, "没有找到用户")
		})
	} else {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.sectionTitle(gtx, "搜索结果", fmt.Sprintf("关键词：%s", strings.TrimSpace(a.searchEdit.Text())))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if a.searchPagination.TotalItems == 0 {
							return layout.Dimensions{}
						}
						return a.badgeChip(gtx, fmt.Sprintf("%d 条", a.searchPagination.TotalItems), colorAccentSoft, colorAccent)
					}),
				)
			})
		})
		if len(a.searchResults) == 0 {
			rows = append(rows, func(gtx layout.Context) layout.Dimensions {
				return a.emptyOrPending(gtx, "没有找到主题")
			})
		} else {
			for i := range a.searchResults {
				article := a.searchResults[i]
				rows = append(rows, func(gtx layout.Context) layout.Dimensions {
					return a.articleTile(gtx, "search", article, "")
				})
			}
			rows = append(rows, a.pager)
		}
	}
	return material.List(a.theme, state).Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		return rows[i](gtx)
	})
}
