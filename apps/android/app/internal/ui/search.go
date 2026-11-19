package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func searchListKey(route Route, query string) listKey {
	return listKey{Page: PageSearch, Query: strings.TrimSpace(query), Number: max(1, route.Number)}
}

func (a *App) loadSearch() {
	if a.searchMode != "all" {
		a.message = "AI 搜索尚未配置，当前仅支持全站搜索"
		a.failed = false
		a.window.Invalidate()
		return
	}
	query := strings.TrimSpace(a.searchEdit.Text())
	if query == "" {
		a.fail(errors.New("请输入搜索关键词"))
		return
	}
	route := a.current()
	page := max(1, route.Number)
	key := searchListKey(route, query)
	a.searchHasRun = true
	a.searchResults = nil
	a.searchPagination = forum.Pagination{}
	a.run("搜索全站主题", func(ctx context.Context) (func(), error) {
		result, err := a.core.Search(ctx, application.SearchRequest{
			Keyword: query,
			Page:    page,
			Size:    30,
		})
		return func() {
			if a.current() != route {
				return
			}
			a.mobileList(key).Position = layout.Position{}
			a.searchResults = result.Articles
			a.searchPagination = result.Pagination
		}, err
	})
}

func (a *App) searchPage(gtx layout.Context) layout.Dimensions {
	route := a.current()
	state := a.mobileList(searchListKey(route, a.searchEdit.Text()))
	all := a.searchMode == "all"
	title := "AI 搜索"
	detail := "AI 搜索需要额外的模型服务，当前尚未配置"
	icon := "hot"
	if all {
		title = "全站搜索"
		detail = "按标题关键词搜索全站主题"
		icon = "search"
	}
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.iconTile(gtx, icon, 42, colorAccentSoft, colorAccent)
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(a.theme, unit.Sp(20), "搜索论坛")
											l.Font.Weight = 700
											return l.Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(a.theme, unit.Sp(11), detail)
											l.Color = colorMuted
											return l.Layout(gtx)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.badgeChip(gtx, title, colorAccentSoft, colorAccent)
								}),
							)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									dims, clicked := a.pill(gtx, "search-mode:ai", "AI 搜索", "hot", !all, false)
									if clicked {
										a.searchMode = "ai"
										a.window.Invalidate()
									}
									return dims
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									dims, clicked := a.pill(gtx, "search-mode:all", "全站搜索", "search", all, false)
									if clicked {
										a.searchMode = "all"
										a.window.Invalidate()
									}
									return dims
								}),
							)
						}),
					)
				})
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				placeholder := "描述你想找的内容，例如：最近关于选课的讨论"
				if all {
					placeholder = "输入标题关键词"
				}
				return a.inputBox(gtx, &a.searchEdit, placeholder)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := "开始 AI 搜索"
				if all {
					label = "开始全站搜索"
				}
				dims, clicked := a.primaryButton(gtx, "search-submit", label)
				if clicked {
					a.loadSearch()
				}
				return dims
			})
		},
	}
	if !a.searchHasRun {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(13), "搜索说明")
							l.Font.Weight = 700
							return l.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.searchFeatureRow(gtx, "全站搜索", "按主题标题检索，支持分页并可直接打开帖子")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.searchFeatureRow(gtx, "AI 搜索", "需要额外模型服务，暂未启用")
						}),
					)
				})
			})
		})
	} else {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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

func (a *App) searchFeatureRow(gtx layout.Context, title, detail string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if title == "AI 搜索" {
					return a.badgeChip(gtx, "敬请期待", colorWarningSoft, colorWarning)
				}
				return a.badgeChip(gtx, "已接入", colorAccentSoft, colorAccent)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(13), title)
						l.Font.Weight = 600
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(11), detail)
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
				)
			}),
		)
	})
}
