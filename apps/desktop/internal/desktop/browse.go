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
	ui.Row(c).Fill().Gap(16).Children(func() {
		ui.Column(c).Width(236).Shrink(0).FillHeight().Background(c.Theme().Surface).
			Border(1, c.Theme().Border).Radius(14).Padding(12).Gap(4).Children(func() {
			ui.Text(c, "分区").FontSize(12).Bold().TextColor(c.Theme().TextMuted).Padding(2, 8)
			ui.Scroll(c).FillWidth().Grow(1).Children(func() {
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					// Subsections live in the board pane as navigable cards; the
					// sidebar keeps only the root sections.
					for _, s := range a.sections {
						a.sectionButton(c, s)
					}
				})
			})
		})
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Gap(12).Children(func() {
			ui.Row(c).FillWidth().Wrap().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, "发现版面").FontSize(22).Bold()
					detail := []string{}
					if path := a.sectionPath(); path != "" {
						detail = append(detail, path)
					}
					detail = append(detail, fmt.Sprintf("%d 个版面", len(a.boards)))
					if len(a.section.Subsections) > 0 {
						detail = append(detail, fmt.Sprintf("%d 个子分区", len(a.section.Subsections)))
					}
					ui.Text(c, strings.Join(detail, " · ")).FontSize(12).TextColor(c.Theme().TextMuted)
				})
				if a.section.Parent != "" {
					a.action(c, "返回上级分区", "back", quiet, func() { a.loadSection(string(a.section.Parent)) })
				}
				a.action(c, "全部版面", "board", quiet, a.loadAllBoards)
				a.action(c, "推荐", "hot", tonal, func() {
					a.navigate(Route{Page: "top", Sub: "recommend"}, true)
				})
			})
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(10).Children(func() {
					if len(a.section.Subsections) > 0 {
						ui.Text(c, "子分区").FontSize(12).Bold().TextColor(c.Theme().TextMuted).Padding(2, 4)
						ui.Column(c).FillWidth().Gap(8).Children(func() {
							for _, sub := range a.section.Subsections {
								a.sectionRow(c, sub)
							}
						})
					}
					n := 0
					for _, b := range a.boards {
						if has(b.Name+" "+b.Description, a.boardFilter) {
							a.boardRow(c, b)
							n++
						}
					}
					if n == 0 && len(a.section.Subsections) == 0 {
						emptyState(c, "board", "没有匹配的版面", "换一个关键词，或选择其他分区。")
					}
				})
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
		v, err := a.Core.Forum.Section(ctx, name)
		if err != nil {
			return nil, err
		}
		a.resolveSubsections(ctx, &v)
		return func() { a.section = v; a.boards = v.Boards }, nil
	})
}
func (a *App) loadAllBoards() {
	a.loadData("加载全部版面", func(ctx context.Context) (func(), error) {
		var out []forum.Board
		seen := map[string]bool{}
		visited := map[forum.Scalar]bool{}
		var visit func(forum.Section) error
		visit = func(s forum.Section) error {
			if visited[s.Name] {
				return nil
			}
			visited[s.Name] = true
			v, err := a.Core.Forum.Section(ctx, string(s.Name))
			if err != nil {
				return err
			}
			for _, b := range v.Boards {
				if !seen[b.Name] {
					seen[b.Name] = true
					out = append(out, b)
				}
			}
			for _, sub := range v.Subsections {
				if err := visit(sub); err != nil {
					return err
				}
			}
			return nil
		}
		roots, err := a.Core.Forum.Sections(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range roots.Sections {
			if err = visit(s); err != nil {
				return nil, err
			}
		}
		return func() {
			a.boards = out
			a.section = forum.Section{}
		}, nil
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
					var err error
					if bool(b.IsFavorite) {
						_, err = a.Core.Forum.RemoveFavorite(ctx, b.Name, "0")
					} else {
						_, err = a.Core.Forum.AddFavorite(ctx, b.Name, "0")
					}
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
func (a *App) articleList(c *ui.Context, articles []forum.Article) {
	if len(articles) == 0 {
		emptyState(c, "message", "暂无主题", "这里还没有可显示的主题，稍后刷新试试。")
		return
	}
	key := listKey{Page: a.route.Page, Name: a.route.Name, Sub: a.route.Sub, Number: a.route.Number}
	if a.route.Page == "search" {
		key.Query = strings.TrimSpace(a.searchQuery)
	}
	state := a.desktopListState(key)
	state.Key = func(i int) any {
		if i >= 0 && i < len(articles) {
			v := articles[i]
			return string(v.Board) + "\x00" + string(v.ID)
		}
		return i
	}
	ui.List(c.Key("articles-"+string(a.route.Page)+a.route.Name+strconv.Itoa(a.route.Number)), state, len(articles), func(i int) {
		v := articles[i]
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
		emptyState(c, "clock", "时间线为空", "论坛没有返回可显示的帖子，稍后刷新试试。")
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
			v, err := a.Core.Forum.Collection(ctx, "", forum.Page{Number: r.Number})
			return func() { a.collection = v }, err
		default:
			v, err := a.Core.Forum.Favorites(ctx, "0")
			return func() { a.favorites = v }, err
		}
	})
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
			emptyState(c, "star", "还没有收藏文章", "在帖子阅读页点击收藏，文章就会出现在这里。")
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
				if len(a.favorites.Boards)+len(a.favorites.Sections)+len(a.favorites.SubFavorites) == 0 {
					emptyState(c, "star", "还没有收藏版面", "在版面页点击收藏，常用版面会集中在这里。")
				}
			})
		})
	}
}
