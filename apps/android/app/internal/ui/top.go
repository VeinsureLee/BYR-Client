package ui

import (
	"context"
	"fmt"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (a *App) selectTopTab(tab string) {
	if a.busy || a.topTab == tab {
		return
	}
	a.topTab = tab
	a.topList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.loadTop()
}

func (a *App) loadTop() {
	tab := a.topTab
	a.run("加载热门讨论", func(ctx context.Context) (func(), error) {
		if tab == "recommend" {
			v, err := a.core.Forum.Recommendations(ctx)
			return func() { a.articles = v.Articles }, err
		}
		v, err := a.core.Forum.TopTen(ctx)
		return func() { a.articles = v.Articles }, err
	})
}

func (a *App) topPage(gtx layout.Context) layout.Dimensions {
	if len(a.articles) == 0 && a.busy {
		return a.pendingPage(gtx, "正在加载热门讨论…")
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(a.theme, unit.Sp(18), a.title())
											l.Font.Weight = 700
											return l.Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(a.theme, unit.Sp(11), "与桌面端相同的十大与版面推荐")
											l.Color = colorMuted
											return l.Layout(gtx)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if len(a.articles) == 0 {
										return layout.Dimensions{}
									}
									return a.badgeChip(gtx, fmt.Sprintf("%d 条", len(a.articles)), colorAccentSoft, colorAccent)
								}),
							)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									dims, clicked := a.pill(gtx, "top-tab:top", "今日十大", "hot", a.topTab == "top", false)
									if clicked {
										a.selectTopTab("top")
									}
									return dims
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									dims, clicked := a.pill(gtx, "top-tab:recommend", "版面推荐", "star", a.topTab == "recommend", false)
									if clicked {
										a.selectTopTab("recommend")
									}
									return dims
								}),
							)
						}),
					)
				})
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(a.articles) == 0 {
				return a.emptyOrPending(gtx, "暂无热门讨论")
			}
			return material.List(a.theme, &a.topList).Layout(gtx, len(a.articles), func(gtx layout.Context, i int) layout.Dimensions {
				return a.articleTile(gtx, "top:"+a.topTab, a.articles[i], "")
			})
		}),
	)
}
