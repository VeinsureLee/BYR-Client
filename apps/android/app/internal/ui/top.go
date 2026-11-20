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
	route := a.current()
	a.run("加载热门讨论", func(ctx context.Context) (func(), error) {
		if tab == "recommend" {
			v, err := a.core.Forum.Recommendations(ctx)
			return func() {
				if a.current() == route && a.topTab == tab {
					a.articles = v.Articles
				}
			}, err
		}
		v, err := a.core.Forum.TopTen(ctx)
		return func() {
			if a.current() == route && a.topTab == tab {
				a.articles = v.Articles
			}
		}, err
	})
}

func (a *App) topPage(gtx layout.Context) layout.Dimensions {
	tabs := []moduleTab{
		{Key: "top", Label: "今日十大"},
		{Key: "recommend", Label: "版面推荐"},
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, a.sectionActions)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.moduleTabs(gtx, tabs, a.topTab, a.selectTopTab)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(a.articles) == 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "", fmt.Sprintf("%d 条", len(a.articles)))
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
