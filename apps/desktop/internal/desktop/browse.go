package desktop

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/egoist/mygo/ui"
)

func (a *App) homeView(c *ui.Context) {
	if a.compactNavigation() {
		ui.Column(c).Fill().Gap(12).Children(func() {
			ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
				for _, s := range a.sections {
					label := s.Description
					if label == "" {
						label = string(s.Name)
					}
					tone := quiet
					if a.section.Name == s.Name || a.section.Parent == s.Name {
						tone = selected
					}
					if control(c, "分区："+label, "board", tone).Clicked() {
						a.loadSection(string(s.Name))
					}
				}
			})
			a.homeContent(c)
		})
		return
	}
	ui.Row(c).Fill().Gap(16).Children(func() {
		ui.Column(c).Width(224).Shrink(0).FillHeight().Background(c.Theme().Surface).
			Border(1, c.Theme().Border).Radius(12).Padding(12).Gap(4).Children(func() {
			ui.Text(c, "分区").FontSize(a.textSize(c, 12)).FontWeight(600).TextColor(c.Theme().TextMuted).Padding(2, 8)
			if len(a.sections) == 0 {
				emptyState(c, "board", "暂无可浏览的分区", "当前没有可显示的分区。")
			} else {
				ui.Scroll(c).FillWidth().Grow(1).Children(func() {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						// Subsections live in the board pane as navigable cards; the
						// sidebar keeps only the root sections.
						for _, s := range a.sections {
							a.sectionButton(c, s)
						}
					})
				})
			}
		})
		a.homeContent(c)
	})
}

func (a *App) homeScrollState() *ui.ScrollState {
	if a.homeScrolls == nil {
		a.homeScrolls = make(map[string]*ui.ScrollState)
	}
	key := string(a.section.Name) + "\x00" + a.boardFilter
	state := a.homeScrolls[key]
	if state == nil {
		state = &ui.ScrollState{}
		a.homeScrolls[key] = state
	}
	return state
}

func (a *App) homeContent(c *ui.Context) {
	ui.Column(c).Grow(1).MinWidth(0).FillHeight().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Wrap().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, "发现版面").FontSize(a.textSize(c, 22)).FontWeight(600)
				detail := []string{}
				if path := a.sectionPath(); path != "" {
					detail = append(detail, path)
				}
				detail = append(detail, fmt.Sprintf("%d 个版面", len(a.boards)))
				if len(a.section.Subsections) > 0 {
					detail = append(detail, fmt.Sprintf("%d 个子分区", len(a.section.Subsections)))
				}
				ui.Text(c, strings.Join(detail, " · ")).FontSize(a.textSize(c, 12)).TextColor(c.Theme().TextMuted)
			})
			if a.section.Parent != "" {
				a.action(c, "返回上级分区", "back", quiet, func() { a.loadSection(string(a.section.Parent)) })
			}
			a.action(c, "全部版面", "board", quiet, a.loadAllBoards)
			a.action(c, "推荐", "hot", tonal, func() {
				a.navigate(Route{Page: "top", Sub: "recommend"}, true)
			})
		})
		hasBoard := false
		for _, board := range a.boards {
			if has(board.Name+" "+board.Description, a.boardFilter) {
				hasBoard = true
				break
			}
		}
		if !hasBoard && len(a.section.Subsections) == 0 {
			emptyState(c, "board", "没有匹配的版面", "换一个关键词，或选择其他分区。")
			return
		}
		ui.Scroll(c).Grow(1).FillWidth().TrackScroll(a.homeScrollState()).Children(func() {
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				if len(a.section.Subsections) > 0 {
					ui.Text(c, "子分区").FontSize(a.textSize(c, 12)).FontWeight(600).TextColor(c.Theme().TextMuted).Padding(2, 4)
					ui.Column(c).FillWidth().Gap(8).Children(func() {
						for _, sub := range a.section.Subsections {
							a.sectionRow(c, sub)
						}
					})
				}
				for _, b := range a.boards {
					if has(b.Name+" "+b.Description, a.boardFilter) {
						a.boardRow(c, b)
					}
				}
			})
		})
	})
}

// sectionRow is a navigable subsection card in the board pane. Clicking it
// enters that subsection instead of hiding its boards behind the sidebar.
func (a *App) sectionRow(c *ui.Context, s forum.Section) {
	title := s.Description
	if title == "" {
		title = string(s.Name)
	}
	active := a.section.Name == s.Name
	e := ui.ButtonBase(c.Key("subsection-"+string(s.Name))).Label("进入子分区 "+title).FillWidth().
		Row().AlignItems(ui.Center).Padding(13, 14).Gap(10).Radius(12).Selected(active)
	bg := c.Theme().Surface
	fg := c.Theme().Text
	switch {
	case active:
		bg = accentSurface(c)
		fg = c.Theme().Accent
	case e.Hovered():
		bg = c.Theme().SurfaceHover
	}
	e.Background(bg).Border(1, c.Theme().Border).TextColor(fg).Children(func() {
		ui.Icon(c, icons["board"]).Size(18, 18).Shrink(0).TextColor(c.Theme().Accent)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, title).Bold().MaxLines(1).Ellipsis("…")
			ui.Text(c, "子分区 · 点击进入").FontSize(11).TextColor(c.Theme().TextMuted)
		})
		ui.Icon(c, icons["next"]).Size(16, 16).Shrink(0).TextColor(c.Theme().TextMuted)
	})
	if e.Clicked() {
		a.loadSection(string(s.Name))
	}
}

// sectionPath renders the loaded section's parent chain for the header, so
// the hierarchy stays visible while browsing subsections.
func (a *App) sectionPath() string {
	if a.section.Name == "" {
		return ""
	}
	labels := []string{}
	if a.section.Parent != "" {
		labels = append(labels, a.sectionDisplay(a.section.Parent))
	}
	labels = append(labels, a.sectionDisplay(a.section.Name))
	return strings.Join(labels, " > ")
}

func (a *App) sectionDisplay(name forum.Scalar) string {
	for _, s := range a.sections {
		if s.Name == name && s.Description != "" {
			return s.Description
		}
	}
	if a.section.Name == name && a.section.Description != "" {
		return a.section.Description
	}
	return string(name)
}

func (a *App) sectionButton(c *ui.Context, s forum.Section) {
	label := s.Description
	if label == "" {
		label = string(s.Name)
	}
	active := a.section.Name == s.Name || a.section.Parent == s.Name
	e := ui.ButtonBase(c.Key("section-"+string(s.Name))).Label(label).FillWidth().
		Row().AlignItems(ui.Center).Justify(ui.Start).Padding(7, 10).Radius(8).Selected(active)
	fg := c.Theme().Text
	switch {
	case active:
		e.Background(accentSurface(c))
		fg = c.Theme().Accent
	case e.Hovered():
		e.Background(c.Theme().SurfaceHover)
	}
	e.TextColor(fg).Children(func() {
		ui.Text(c, label).FontSize(13).MaxLines(1).Ellipsis("…")
	})
	if e.Clicked() {
		a.loadSection(string(s.Name))
	}
}
func (a *App) loadSection(name string) {
	a.loadData("加载分区", func(ctx context.Context) (func(), error) {
		v, err := a.Core.LoadSection(ctx, name)
		return func() { a.section = v; a.boards = v.Boards }, err
	})
}
func (a *App) loadAllBoards() {
	a.loadData("加载全部版面", func(ctx context.Context) (func(), error) {
		out, err := a.Core.LoadAllBoards(ctx)
		return func() {
			a.boards = out
			a.section = forum.Section{}
		}, err
	})
}
func (a *App) boardRow(c *ui.Context, b forum.Board) {
	label := b.Description + " · " + b.Name
	if b.Description == "" {
		label = b.Name
	}
	ui.Column(c.Key("board-"+b.Name)).FillWidth().Padding(13, 14).Gap(5).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			textLink(c, label, func() { a.navigate(Route{Page: "board", Name: b.Name}, true) }, a.busy)
			if bool(b.IsFavorite) {
				badge(c, "已收藏", c.Theme().Warning)
			}
		})
		ui.Row(c).FillWidth().Wrap().Gap(10).Children(func() {
			ui.Textf(c, "今日主题 %d · 在线 %d", b.ThreadsTodayCount, b.UserOnlineCount).
				FontSize(12).TextColor(c.Theme().TextMuted)
			if b.PostTodayCount > 0 {
				ui.Textf(c, "· 今日发帖 %d", b.PostTodayCount).FontSize(12).TextColor(c.Theme().TextMuted)
			}
			if len(b.Managers) > 0 {
				ui.Text(c, "· 版主 "+b.Managers[0].DisplayName()).FontSize(12).TextColor(c.Theme().TextMuted)
			}
		})
	})
}
func (a *App) boardView(c *ui.Context) {
	title := a.board.Description
	if title == "" {
		title = a.route.Name
	}
	ui.Column(c).FillWidth().Padding(14).Gap(7).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
		ui.Row(c).FillWidth().Wrap().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, title).FontSize(18).Bold().MaxLines(1).Ellipsis("…")
				ui.Textf(c, "%s · 今日主题 %d · 今日发帖 %d · 在线 %d", a.route.Name,
					a.board.ThreadsTodayCount, a.board.PostTodayCount, a.board.UserOnlineCount).
					FontSize(12).TextColor(c.Theme().TextMuted)
			})
			a.action(c, "在此版发帖", "plus", filled, func() {
				a.navigate(Route{Page: "compose", Name: a.route.Name}, true)
			})
			label := "收藏版面"
			tone := quiet
			if bool(a.board.IsFavorite) {
				label, tone = "已收藏版面", selected
			}
			a.action(c, label, "star", tone, func() {
				b := a.board
				a.run("更新版面收藏", func(ctx context.Context) (func(), error) {
					_, err := a.Core.SetFavorite(ctx, b.Name, "0", !bool(b.IsFavorite))
					return func() {
						a.board.IsFavorite = forum.Flag(!bool(b.IsFavorite))
						if bool(b.IsFavorite) {
							a.notice("已取消版面收藏")
						} else {
							a.notice("已加入版面收藏")
						}
					}, err
				})
			})
		})
		if len(a.board.Managers) > 0 {
			names := make([]string, 0, len(a.board.Managers))
			for _, m := range a.board.Managers {
				names = append(names, m.DisplayName())
			}
			ui.Text(c, "版主："+strings.Join(names, "、")).FontSize(12).TextColor(c.Theme().TextMuted)
		}
	})
	a.articleList(c, a.board.Articles)
	a.pager(c, a.board.Pagination, a.pageNumber)
}
func (a *App) boardArticleList(c *ui.Context, articles []forum.Article) {
	if len(articles) == 0 {
		a.emptyRequestState(c, "message", "暂无主题", "这里还没有可显示的主题，稍后刷新试试。")
		return
	}
	pinned := make([]forum.Article, 0, len(articles))
	normal := make([]forum.Article, 0, len(articles))
	for _, article := range articles {
		if bool(article.IsTop) {
			pinned = append(pinned, article)
		} else {
			normal = append(normal, article)
		}
	}

	key := a.articleListKey()
	expanded := a.isPinnedExpanded(key)
	rows := len(normal)
	if len(pinned) > 0 {
		rows++
		if expanded {
			rows += len(pinned)
		}
	}
	state := a.desktopListState(key)
	state.Key = func(i int) any {
		if len(pinned) > 0 {
			if i == 0 {
				return "pinned-summary"
			}
			i--
			if expanded {
				if i < len(pinned) {
					v := pinned[i]
					return string(v.Board) + "\x00" + string(v.ID)
				}
				i -= len(pinned)
			}
		}
		if i >= 0 && i < len(normal) {
			v := normal[i]
			return string(v.Board) + "\x00" + string(v.ID)
		}
		return i
	}
	ui.List(c.Key("articles-"+string(a.route.Page)+a.route.Name+strconv.Itoa(a.route.Number)), state, rows, func(i int) {
		if len(pinned) > 0 {
			if i == 0 {
				a.pinnedSummary(c, key, len(pinned))
				return
			}
			i--
			if expanded {
				if i < len(pinned) {
					a.articleRow(c, pinned[i])
					return
				}
				i -= len(pinned)
			}
		}
		a.articleRow(c, normal[i])
	}).Grow(1).FillWidth().Gap(8)
}

func (a *App) pinnedSummary(c *ui.Context, key listKey, count int) {
	expanded := a.isPinnedExpanded(key)
	icon, tone := "pin", quiet
	if expanded {
		icon, tone = "down", selected
	}
	label := fmt.Sprintf("置顶主题 · %d", count)
	e := control(c, label, icon, tone).Disabled(a.busy)
	if e.Clicked() {
		a.setPinnedExpanded(key, !expanded)
		c.Invalidate()
	}
}

func (a *App) articleRow(c *ui.Context, v forum.Article) {
	ui.Column(c.Key(string(v.ID)+v.Board)).FillWidth().Padding(12, 14).Gap(6).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Start).Children(func() {
			a.avatar(c, v.Author, 32)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Row(c).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
					if bool(v.IsTop) {
						badge(c, "置顶", c.Theme().Warning)
					}
					title := v.Title
					if title == "" {
						title = "无标题"
					}
					textLink(c, title, func() { a.openArticle(v) }, a.busy)
				})
				meta := []string{}
				if v.Author.DisplayName() != "" {
					meta = append(meta, v.Author.DisplayName())
				}
				if v.Board != "" {
					meta = append(meta, v.Board)
				}
				if v.ReplyCount > 0 {
					meta = append(meta, fmt.Sprintf("%d 回复", v.ReplyCount))
				}
				if t := formatTime(v.PostTime); t != "" {
					meta = append(meta, t)
				}
				if len(meta) > 0 {
					ui.Text(c, strings.Join(meta, " · ")).FontSize(12).
						TextColor(c.Theme().TextMuted).MaxLines(1).Ellipsis("…")
				}
			})
			ui.Icon(c, icons["next"]).Size(15, 15).TextColor(c.Theme().TextMuted).Shrink(0)
		})
	})
}

func (a *App) articleList(c *ui.Context, articles []forum.Article) {
	if a.route.Page == "board" {
		a.boardArticleList(c, articles)
		return
	}
	if len(articles) == 0 {
		a.emptyRequestState(c, "message", "暂无主题", "这里还没有可显示的主题，稍后刷新试试。")
		return
	}
	key := a.articleListKey()
	state := a.desktopListState(key)
	state.Key = func(i int) any {
		if i >= 0 && i < len(articles) {
			v := articles[i]
			return string(v.Board) + "\x00" + string(v.ID)
		}
		return i
	}
	ui.List(c.Key("articles-"+string(a.route.Page)+a.route.Name+strconv.Itoa(a.route.Number)), state, len(articles), func(i int) {
		a.articleRow(c, articles[i])
	}).Grow(1).FillWidth().Gap(8)
}
func (a *App) timelineView(c *ui.Context) {
	ui.Row(c).FillWidth().Wrap().Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, "全部帖子").FontSize(16).Bold()
			ui.Text(c, "按发帖时间倒序排列").FontSize(12).TextColor(c.Theme().TextMuted)
		})
		if a.pagination.TotalItems > 0 {
			badge(c, fmt.Sprintf("%d 条", a.pagination.TotalItems), c.Theme().Accent)
		}
	})
	if len(a.articles) == 0 {
		a.emptyRequestState(c, "clock", "时间线为空", "论坛没有返回可显示的帖子，稍后刷新试试。")
		return
	}
	a.articleList(c, a.articles)
	a.pager(c, a.pagination, a.pageNumber)
}
func (a *App) loadFavorites() {
	r := a.route
	a.loadData("加载收藏", func(ctx context.Context) (func(), error) {
		switch r.Sub {
		case "collection":
			v, err := a.Core.LoadCollection(ctx, "", forum.Page{Number: r.Number})
			return func() { a.collection = v }, err
		default:
			v, err := a.Core.LoadFavorites(ctx, "0")
			return func() { a.favorites = v }, err
		}
	})
}
func favoriteTreeEmpty(f forum.Favorite) bool {
	if len(f.Sections) > 0 || len(f.Boards) > 0 {
		return false
	}
	for _, child := range f.SubFavorites {
		if !favoriteTreeEmpty(child) {
			return false
		}
	}
	return true
}

func (a *App) favoritesView(c *ui.Context) {
	ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
		for _, v := range []struct{ key, label, icon string }{
			{"", "版面", "board"},
			{"collection", "文章收藏", "star"},
		} {
			tone := quiet
			if a.route.Sub == v.key {
				tone = selected
			}
			if control(c, v.label, v.icon, tone).Clicked() {
				a.route.Sub = v.key
				a.route.Number = 1
				a.loadFavorites()
			}
		}
	})
	switch a.route.Sub {
	case "collection":
		if len(a.collection.Entries) == 0 {
			a.emptyRequestState(c, "star", "还没有收藏文章", "在帖子阅读页点击收藏，文章就会出现在这里。")
			return
		}
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				for _, e := range a.collection.Entries {
					e := e
					ui.Column(c.Key("collection-"+string(e.ID))).FillWidth().Padding(12, 14).Gap(6).
						Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).Children(func() {
						ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
							textLink(c, e.Title, func() {
								a.Core.RememberCollection(e.Board, string(e.ID))
								a.navigate(Route{Page: "thread", Name: e.Board, ID: string(e.ID)}, true)
							}, a.busy)
							a.action(c, "取消收藏", "trash", destructive, func() {
								board, id := e.Board, string(e.ID)
								a.confirm = &confirmation{text: "取消收藏这篇文章？", run: func() {
									a.run("取消收藏", func(ctx context.Context) (func(), error) {
										_, err := a.Core.SetCollected(ctx, board, id, false)
										return func() { a.loadFavorites() }, err
									})
								}}
							})
						})
						ui.Textf(c, "%s · 回复 %d · %s", e.Board, e.ReplyCount, formatTime(e.PostTime)).
							FontSize(12).TextColor(c.Theme().TextMuted)
					})
				}
			})
		})
		a.pager(c, a.collection.Pagination, a.pageNumber)
	default:
		if favoriteTreeEmpty(a.favorites) {
			a.emptyRequestState(c, "star", "还没有收藏版面", "在版面页点击收藏，常用版面会集中在这里。")
			return
		}
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				var tree func(forum.Favorite)
				tree = func(f forum.Favorite) {
					for _, s := range f.Sections {
						label := s.Description
						if label == "" {
							label = string(s.Name)
						}
						ui.Column(c).FillWidth().Padding(12, 14).Gap(5).
							Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).Children(func() {
							textLink(c, "分区 · "+label, func() { a.navigate(Route{Page: "home", Name: string(s.Name)}, true) }, a.busy)
						})
					}
					for _, b := range f.Boards {
						a.boardRow(c, b)
					}
					for _, sub := range f.SubFavorites {
						tree(sub)
					}
				}
				tree(a.favorites)
			})
		})
	}
}
