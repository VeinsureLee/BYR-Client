package ui

import (
	"context"
	"fmt"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (a *App) loadTop() {
	route := a.current()
	recommend := route.Page == PageTop && route.Name == "recommend"
	tab := "top"
	if recommend {
		tab = "recommend"
	}
	a.topTab = tab
	a.run("加载热门讨论", func(ctx context.Context) (func(), error) {
		if recommend {
			v, err := a.core.Forum.Recommendations(ctx)
			return func() {
				if a.current() == route {
					a.articles = v.Articles
				}
			}, err
		}
		v, err := a.core.Forum.TopTen(ctx)
		return func() {
			if a.current() == route {
				a.articles = v.Articles
			}
		}, err
	})
}

func (a *App) topPage(gtx layout.Context) layout.Dimensions {
	route := a.current()
	list := &a.topList
	key := "top"
	if route.Name == "recommend" {
		list = &a.recommendList
		key = "recommend"
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, a.sectionActions)
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
				label := "暂无热门讨论"
				if key == "recommend" {
					label = "暂无版面推荐"
				}
				return a.emptyOrPending(gtx, label)
			}
			return material.List(a.theme, list).Layout(gtx, len(a.articles), func(gtx layout.Context, i int) layout.Dimensions {
				return a.articleTile(gtx, "top:"+key, a.articles[i], "")
			})
		}),
	)
}
