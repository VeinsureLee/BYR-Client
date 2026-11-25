package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func searchListKey(route Route, query string) listKey {
	return listKey{Page: PageSearch, Query: strings.TrimSpace(query), Number: max(1, route.Number)}
}

const searchClearGapDp = 6
const searchClearAccessoryDp = searchClearGapDp + iconButtonSizeDp

func searchActionWidth(available, accessory int, hasAccessory bool) int {
	if hasAccessory {
		available -= accessory
	}
	return max(0, available)
}

func (a *App) searchSubmitButton(gtx layout.Context, key, label string, reserveClear bool) (layout.Dimensions, bool) {
	if !reserveClear {
		return a.primaryButton(gtx, key, label)
	}
	buttonWidth := searchActionWidth(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(searchClearAccessoryDp)), true)
	clicked := false
	dims := layout.Inset{Right: unit.Dp(searchClearAccessoryDp)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = buttonWidth
		gtx.Constraints.Max.X = buttonWidth
		var button layout.Dimensions
		button, clicked = a.primaryButton(gtx, key, label)
		return button
	})
	return dims, clicked
}

func (a *App) loadSearch() {
	if a.busy {
		return
	}
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
	user := a.searchMode == "user"
	sameQuery := !user && a.searchHasRun && a.searchListQuery == query
	if !sameQuery && route.Number != 1 && len(a.stack) > 0 {
		a.stack[len(a.stack)-1].Number = 1
		route = a.current()
	}
	page := max(1, route.Number)
	if user {
		a.searchHasRun = true
		a.searchListQuery = query
		a.searchResults = nil
		a.searchPagination = forum.Pagination{}
		a.searchIncomplete = false
		a.searchFailedBoards = 0
		a.searchRetryQuery = ""
		a.searchRetryPage = 0
		a.run("加载用户资料", func(ctx context.Context) (func(), error) {
			user, err := a.core.LookupUser(ctx, query)
			if errors.Is(err, application.ErrStaleSession) {
				err = context.Canceled
			}
			return func() {
				if a.current() != route || a.searchListQuery != query || a.searchMode != "user" {
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
	a.searchRetryQuery = query
	a.searchRetryPage = page
	a.run("搜索全站主题", func(ctx context.Context) (func(), error) {
		result, err := a.core.Search(ctx, application.SearchRequest{
			Keyword: query,
			Page:    page,
			Size:    30,
		})
		return func() {
			if a.current() != route || a.searchListQuery != query || a.searchMode != "all" {
				return
			}
			if !sameQuery {
				a.mobileList(key).Position = layout.Position{}
			}
			a.searchResults = result.Articles
			a.searchPagination = result.Pagination
			a.searchIncomplete = result.Incomplete
			a.searchFailedBoards = result.FailedBoards
			a.searchRetryQuery = ""
			a.searchRetryPage = 0
		}, err
	})
}

// loadSearchPage requests a page without changing the displayed page, results
// or list position. The commit happens only after a successful response; a
// failed page keeps the previous page usable and leaves the target for retry.
func (a *App) loadSearchPage(page int) {
	a.loadSearchPageFor(a.searchListQuery, page)
}

// loadSearchPageFor keeps the submitted query with the target page so an
// unrelated edit in the search field cannot suppress the retry or misattribute
// the response.
func (a *App) loadSearchPageFor(query string, page int) {
	if a.busy || a.searchMode != "all" {
		return
	}
	query = strings.TrimSpace(query)
	if query == "" || a.searchListQuery != query {
		return
	}
	if page < 1 {
		page = 1
	}
	route := a.current()
	a.searchRetryQuery = query
	a.searchRetryPage = page
	a.run("搜索全站主题", func(ctx context.Context) (func(), error) {
		result, err := a.core.Search(ctx, application.SearchRequest{
			Keyword: query,
			Page:    page,
			Size:    30,
		})
		return func() {
			if a.current() != route || a.searchListQuery != query || a.searchMode != "all" {
				return
			}
			a.stack[len(a.stack)-1].Number = page
			a.mobileList(searchListKey(a.current(), query)).Position = layout.Position{}
			a.searchResults = result.Articles
			a.searchPagination = result.Pagination
			a.searchIncomplete = result.Incomplete
			a.searchFailedBoards = result.FailedBoards
			a.searchRetryQuery = ""
			a.searchRetryPage = 0
		}, err
	})
}

func (a *App) retrySearch() {
	query := a.searchListQuery
	if query == "" {
		a.loadSearch()
		return
	}
	a.loadSearchPageFor(query, max(1, a.current().Number))
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
	a.resetSearchIncomplete()
	a.failed = false
	a.message = ""
	a.window.Invalidate()
}

func (a *App) resetSearchIncomplete() {
	a.searchIncomplete = false
	a.searchFailedBoards = 0
	a.searchRetryQuery = ""
	a.searchRetryPage = 0
}

func (a *App) setSearchMode(mode string) {
	if mode != "ai" && mode != "all" && mode != "user" || a.searchMode == mode {
		return
	}
	if a.busy && a.cancel != nil {
		a.cancel()
	}
	a.searchMode = mode
	if mode == "user" {
		a.searchEdit.InputHint = key.HintEmail
	} else {
		a.searchEdit.InputHint = key.HintAny
	}
	a.searchResults = nil
	a.searchPagination = forum.Pagination{}
	a.searchHasRun = false
	a.searchListQuery = ""
	a.resetSearchIncomplete()
	a.failed = false
	a.message = ""
	a.window.Invalidate()
}

func (a *App) searchPage(gtx layout.Context) layout.Dimensions {
	route := a.current()
	items := []moduleTab{
		{Key: "ai", Label: "AI 搜索"},
		{Key: "all", Label: "全站搜索"},
		{Key: "user", Label: "人物"},
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.moduleTabs(gtx, items, a.searchMode, a.setSearchMode)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				placeholder := "描述你想找的内容"
				if a.searchMode == "all" {
					placeholder = "输入标题关键词"
				} else if a.searchMode == "user" {
					placeholder = "输入完整用户名或论坛 ID"
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						intent := a.applySingleLineIntent(gtx, &a.searchEdit, false)
						if intent.Submitted {
							a.loadSearch()
						}
						return a.inputBox(gtx, &a.searchEdit, placeholder)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if a.searchEdit.Text() == "" {
							return layout.Dimensions{}
						}
						return layout.Spacer{Width: unit.Dp(searchClearGapDp)}.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if a.searchEdit.Text() == "" {
							return layout.Dimensions{}
						}
						dims, clicked := a.iconButtonLabel(gtx, "search-clear", "close", "清空搜索")
						if clicked {
							a.clearSearch()
						}
						return dims
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := "开始 AI 搜索"
				if a.searchMode == "all" {
					label = "开始全站搜索"
				} else if a.searchMode == "user" {
					label = "查找人物"
				}
				dims, clicked := a.searchSubmitButton(gtx, "search-submit", label, a.searchEdit.Text() != "")
				if clicked {
					a.loadSearch()
				}
				return dims
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if a.searchMode == "ai" {
				return a.empty(gtx, "AI 搜索尚未配置")
			}
			if !a.searchHasRun {
				hint := "输入关键词开始搜索"
				if a.searchMode == "user" {
					hint = "输入完整论坛 ID 或用户名查找人物"
				}
				if a.busy || a.failed {
					return a.emptyOrPending(gtx, hint)
				}
				return a.empty(gtx, hint)
			}
			if a.searchMode == "user" {
				return a.emptyOrPending(gtx, "没有找到用户")
			}
			if len(a.searchResults) == 0 {
				if !a.busy && !a.failed && a.searchIncomplete {
					return a.incompleteSearchEmpty(gtx)
				}
				return a.emptyOrPending(gtx, "没有找到主题")
			}
			return a.searchResultsList(gtx, route)
		}),
	)
}

// searchIncompleteNotice is shown above partial results so a board failure is
// visible without discarding the hits that did arrive.
func (a *App) searchIncompleteNotice(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(14), "结果可能不完整")
							l.Font.Weight = 600
							l.Color = colorWarning
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Caption(a.theme, fmt.Sprintf("有 %d 个版面加载失败，未显示其内容。", max(1, a.searchFailedBoards)))
							l.Color = colorMuted
							return l.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.textAction(gtx, "search-incomplete-retry", "重试")
					if clicked {
						a.retrySearch()
					}
					return dims
				}),
			)
		})
	})
}

func (a *App) incompleteSearchEmpty(gtx layout.Context) layout.Dimensions {
	return a.centeredContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(24), Right: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.drawIcon(gtx, "alert", 32, colorWarning)
					})
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					l := material.Label(a.theme, unit.Sp(17), "搜索未完整")
					l.Font.Weight = 600
					l.Alignment = text.Middle
					l.Color = a.theme.Palette.Fg
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					l := material.Body2(a.theme, fmt.Sprintf("有 %d 个版面加载失败，暂未取得结果。可以重试同一关键词。", max(1, a.searchFailedBoards)))
					l.Alignment = text.Middle
					l.Color = colorMuted
					l.MaxLines = 3
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.secondaryButton(gtx, "search-incomplete-retry", "重试搜索")
					if clicked {
						a.retrySearch()
					}
					return dims
				}),
			)
		})
	})
}

func (a *App) searchResultsList(gtx layout.Context, route Route) layout.Dimensions {
	key := searchListKey(route, a.searchListQuery)
	state := a.mobileList(key)
	extra := 0
	if a.searchIncomplete {
		extra = 1
	}
	rows := len(a.searchResults) + 2 + extra
	return material.List(a.theme, state).Layout(gtx, rows, func(gtx layout.Context, i int) layout.Dimensions {
		if i == 0 {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.sectionTitle(gtx, "搜索结果", fmt.Sprintf("关键词：%s", a.searchListQuery))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if a.searchPagination.TotalItems == 0 {
							return layout.Dimensions{}
						}
						return a.badgeChip(gtx, fmt.Sprintf("%d 条", a.searchPagination.TotalItems), colorAccentSoft, colorAccent)
					}),
				)
			})
		}
		if extra > 0 && i == 1 {
			return a.searchIncompleteNotice(gtx)
		}
		index := i - 1 - extra
		if index < len(a.searchResults) {
			return a.articleTile(gtx, "search", a.searchResults[index], "")
		}
		return a.pager(gtx)
	})
}
