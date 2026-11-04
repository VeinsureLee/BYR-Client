package fyneui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (u *UI) buildSearch() fyne.CanvasObject {
	u.query = widget.NewEntry()
	u.query.PlaceHolder = "关键词，如：Go、中文、实习"
	u.query.Icon = theme.SearchIcon()
	search := func() { u.controller.Search(u.query.Text) }
	u.query.OnSubmitted = func(string) { search() }
	submit := widget.NewButtonWithIcon("搜索", theme.SearchIcon(), search)
	submit.Importance = widget.HighImportance
	stop := widget.NewButton("停止", u.controller.StopSearch)
	u.searchStatus = widget.NewLabel("离线数据搜索，复用现有跨版检索服务。")
	u.searchStatus.Wrapping = fyne.TextWrapWord
	u.searchList = widget.NewList(func() int { return len(u.searchResults) }, func() fyne.CanvasObject {
		return container.NewVBox(heading("主题"), subtle("版面 · 作者"), subtle("内容摘要"))
	}, func(index widget.ListItemID, object fyne.CanvasObject) {
		if index >= len(u.searchResults) {
			return
		}
		article := u.searchResults[index]
		box := object.(*fyne.Container)
		box.Objects[0].(*widget.Label).SetText(article.Title)
		box.Objects[1].(*widget.Label).SetText(fmt.Sprintf("%s · %s · %d 回复", article.Key.BoardID, article.Author, article.ReplyCount))
		box.Objects[2].(*widget.Label).SetText(article.Summary)
	})
	u.searchList.OnSelected = func(index widget.ListItemID) {
		if index < len(u.searchResults) {
			u.reading = true
			u.controller.OpenThread(u.searchResults[index].Key)
			u.searchList.UnselectAll()
			u.showPage(pageBrowse)
			u.browse.Refresh()
		}
	}
	toolbar := container.NewBorder(nil, nil, nil, container.NewHBox(submit, stop), u.query)
	return panel(container.NewBorder(container.NewVBox(heading("关键词检索"), toolbar, u.searchStatus), nil, nil, nil, u.searchList))
}

func (u *UI) syncSearch() {
	run := u.state.Search
	stamp := fmt.Sprintf("%s/%s/%d/%d", u.state.SearchID, run.Status, run.ArticleCount, len(run.Boards))
	if stamp == u.searchStamp {
		return
	}
	u.searchStamp = stamp
	u.searchResults = nil
	for _, board := range run.Boards {
		for _, result := range board.Articles {
			u.searchResults = append(u.searchResults, result.Article)
		}
	}
	if u.state.SearchID != "" {
		u.searchStatus.SetText(fmt.Sprintf("%s · %d 个主题 · %d 个版面 · %s", run.Status, run.ArticleCount, len(run.Boards), run.Error))
	}
	u.searchList.Refresh()
}
