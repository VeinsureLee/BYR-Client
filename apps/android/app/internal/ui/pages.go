package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func (a *App) loginPage(gtx layout.Context) layout.Dimensions {
	// The login form is a plain column: no scroll container, so nothing can
	// swallow a tap aimed at the button.
	screen := gtx.Constraints.Max
	scale := gtx.Metric.PxPerDp
	if scale <= 0 {
		scale = 1
	}
	return layout.Inset{Left: unit.Dp(20), Right: unit.Dp(20), Top: unit.Dp(12), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// Compact header row: badge and titles on one line.
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						badge := image.Pt(gtx.Dp(unit.Dp(40)), gtx.Dp(unit.Dp(40)))
						paint.FillShape(gtx.Ops, a.theme.ContrastBg,
							clip.RRect{Rect: image.Rectangle{Max: badge}, NE: 10, NW: 10, SE: 10, SW: 10}.Op(gtx.Ops))
						gtx.Constraints = layout.Exact(badge)
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							l := material.H6(a.theme, "BYR")
							l.Color = a.theme.ContrastFg
							l.Font.Weight = 700
							return l.Layout(gtx)
						})
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(material.H5(a.theme, "BYR Client").Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Caption(a.theme, "使用北邮人论坛用户名登录")
								l.Color = colorHint
								return l.Layout(gtx)
							}),
						)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.inputBox(gtx, &a.username, "论坛用户名")
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// Capture the IME action key so the keyboard can submit.
				if event, ok := a.password.Update(gtx); ok {
					if _, submit := event.(widget.SubmitEvent); submit {
						a.login()
					}
				}
				return a.inputBox(gtx, &a.password, "密码")
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dims, clicked := a.primaryButton(gtx, "login", "登 录")
				if clicked {
					a.login()
				}
				return dims
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.busy {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							loader := material.Loader(a.theme)
							loader.Color = a.theme.ContrastBg
							return loader.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Body2(a.theme, "正在登录…")
							l.Color = colorHint
							return l.Layout(gtx)
						}),
					)
				}
				if a.failed && a.message != "" {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Body2(a.theme, a.message)
						l.Color = color.NRGBA{R: 0xB3, G: 0x26, B: 0x1E, A: 0xFF}
						return l.Layout(gtx)
					})
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.CheckBox(a.theme, &a.remember, "记住登录").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Caption(a.theme, fmt.Sprintf("版本 %s · 屏幕 %.0f×%.0fdp · 共享 Go 核心", a.version, float32(screen.X)/scale, float32(screen.Y)/scale))
					l.Color = colorHint
					return l.Layout(gtx)
				})
			}),
		)
	})
}

func (a *App) login() {
	u := strings.TrimSpace(a.username.Text())
	p := a.password.Text()
	if u == "" || p == "" {
		a.fail(errors.New("请输入用户名和密码"))
		return
	}
	a.run("正在登录", func(ctx context.Context) (func(), error) {
		err := a.core.Login(ctx, u, p, a.remember.Value)
		return func() {
			a.password.SetText("")
			a.stack = []Route{{Page: PageHome}}
			a.loadHome()
		}, err
	})
}

func (a *App) button(gtx layout.Context, key, label string) (layout.Dimensions, bool) {
	btn := a.click(key)
	// Gio consumes the click event inside Layout, so Clicked must run first.
	clicked := btn.Clicked(gtx)
	dims := material.Button(a.theme, btn, label).Layout(gtx)
	return dims, clicked
}

func (a *App) textAction(gtx layout.Context, key, label string) (layout.Dimensions, bool) {
	btn := a.click(key)
	clicked := btn.Clicked(gtx)
	dims := material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(4), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.theme, label)
			l.Color = a.theme.ContrastBg
			l.TextSize = unit.Sp(13)
			return l.Layout(gtx)
		})
	})
	return dims, clicked
}

func (a *App) loadHome() {
	a.run("加载首页", func(ctx context.Context) (func(), error) {
		root, err := a.core.Forum.Sections(ctx)
		if err != nil {
			return nil, err
		}
		selected := a.section.Name
		if selected == "" && len(root.Sections) > 0 {
			selected = root.Sections[0].Name
		}
		var section forum.Section
		if selected != "" {
			section, err = a.core.Forum.Section(ctx, string(selected))
			if err != nil {
				return nil, err
			}
			a.resolveSubsections(ctx, &section)
		}
		return func() {
			a.sections = root.Sections
			a.section = section
			a.boards = section.Boards
		}, nil
	})
}

func (a *App) loadAllBoards() {
	a.run("加载全部版面", func(ctx context.Context) (func(), error) {
		roots, err := a.core.Forum.Sections(ctx)
		if err != nil {
			return nil, err
		}
		var out []forum.Board
		seen := map[string]bool{}
		visited := map[forum.Scalar]bool{}
		var visit func(forum.Section) error
		visit = func(s forum.Section) error {
			if visited[s.Name] {
				return nil
			}
			visited[s.Name] = true
			v, err := a.core.Forum.Section(ctx, string(s.Name))
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
		for _, root := range roots.Sections {
			if err := visit(root); err != nil {
				return nil, err
			}
		}
		return func() { a.boards = out; a.section = forum.Section{} }, nil
	})
}

func (a *App) loadSection(name string) {
	a.run("加载分区", func(ctx context.Context) (func(), error) {
		section, err := a.core.Forum.Section(ctx, name)
		if err != nil {
			return nil, err
		}
		a.resolveSubsections(ctx, &section)
		return func() { a.section = section; a.boards = section.Boards }, nil
	})
}

func (a *App) resolveSubsections(ctx context.Context, section *forum.Section) {
	for i := range section.Subsections {
		sub := &section.Subsections[i]
		if sub.Description != "" || sub.Name == "" {
			continue
		}
		if full, err := a.core.Forum.Section(ctx, string(sub.Name)); err == nil {
			*sub = full
		}
	}
}

type homeRow struct {
	kind string
	name string
	s    forum.Section
	b    forum.Board
}

func (a *App) homeRows() []homeRow {
	rows := []homeRow{{kind: "overview"}, {kind: "header", name: "分区"}}
	for _, s := range a.sections {
		rows = append(rows, homeRow{kind: "section", name: string(s.Name), s: s})
	}
	rows = append(rows, homeRow{kind: "header", name: "版面"})
	for _, sub := range a.section.Subsections {
		rows = append(rows, homeRow{kind: "sub", name: string(sub.Name), s: sub})
	}
	for _, b := range a.boards {
		rows = append(rows, homeRow{kind: "board", name: b.Name, b: b})
	}
	return rows
}

func (a *App) homePage(gtx layout.Context) layout.Dimensions {
	rows := a.homeRows()
	if len(a.sections) == 0 && a.busy {
		return a.loadingPage(gtx, "正在加载分区…")
	}
	return material.List(a.theme, &a.homeList).Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		row := rows[i]
		switch row.kind {
		case "overview":
			return a.homeOverview(gtx)
		case "header":
			return a.sectionTitle(gtx, row.name, "")
		case "section":
			return a.sectionCard(gtx, row.s)
		case "sub":
			return a.subsectionCard(gtx, row.s)
		case "board":
			return a.boardCard(gtx, row.b)
		}
		return layout.Dimensions{}
	})
}

func (a *App) homeOverview(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(20), "发现版面")
									l.Font.Weight = 700
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									detail := a.section.Description
									if detail == "" {
										detail = "选择一个分区"
									}
									detail += fmt.Sprintf(" · %d 个版面", len(a.boards))
									if n := len(a.section.Subsections); n > 0 {
										detail += fmt.Sprintf(" · %d 个子分区", n)
									}
									l := material.Label(a.theme, unit.Sp(11), detail)
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.pill(gtx, "home-all", "全部版面", "board", false, false)
							if clicked {
								a.loadAllBoards()
							}
							return dims
						}),
					)
				}),
			)
		})
	})
}

func (a *App) sectionCard(gtx layout.Context, s forum.Section) layout.Dimensions {
	title := s.Description
	if title == "" {
		title = string(s.Name)
	}
	active := a.section.Name == s.Name
	btn := a.click("section:" + string(s.Name))
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.tile(gtx, active, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.iconTile(gtx, "hash", 34, colorSurface, colorAccent)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(11)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(14), title)
						l.Font.Weight = 600
						if active {
							l.Color = colorAccent
						}
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.drawIcon(gtx, "next", 15, colorMuted)
					}),
				)
			})
		})
	})
	if clicked {
		a.loadSection(string(s.Name))
	}
	return dims
}

func (a *App) subsectionCard(gtx layout.Context, s forum.Section) layout.Dimensions {
	title := s.Description
	if title == "" {
		title = "子分区 " + string(s.Name)
	}
	active := a.section.Name == s.Name
	btn := a.click("sub:" + string(s.Name))
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.tile(gtx, active, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.iconTile(gtx, "board", 34, colorSurface, colorAccent)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(11)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(14), title)
								l.Font.Weight = 600
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(11), "子分区 · 点击进入")
								l.Color = colorMuted
								return l.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.drawIcon(gtx, "next", 15, colorMuted)
					}),
				)
			})
		})
	})
	if clicked {
		a.loadSection(string(s.Name))
	}
	return dims
}

func (a *App) boardCard(gtx layout.Context, b forum.Board) layout.Dimensions {
	label := b.Description
	if label == "" {
		label = b.Name
	}
	btn := a.click("board:" + b.Name)
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.tile(gtx, false, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.iconTile(gtx, "board", 38, colorAccentSoft, colorAccent)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										l := material.Label(a.theme, unit.Sp(14), label)
										l.Font.Weight = 600
										return l.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !bool(b.IsFavorite) {
											return layout.Dimensions{}
										}
										return a.badgeChip(gtx, "已收藏", colorWarningSoft, colorWarning)
									}),
								)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(11), b.Name)
								l.Color = colorMuted
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(11),
									fmt.Sprintf("今日主题 %d · 今日发帖 %d · 在线 %d", b.ThreadsTodayCount, b.PostTodayCount, b.UserOnlineCount))
								l.Color = colorMuted
								return l.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.drawIcon(gtx, "next", 16, colorMuted)
					}),
				)
			})
		})
	})
	if clicked {
		a.openBoard(b.Name)
	}
	return dims
}

func (a *App) openBoard(name string) {
	a.push(Route{Page: PageBoard, Name: name, Number: 1})
	a.loadBoard()
}

func (a *App) loadBoard() {
	r := a.current()
	a.run("加载版面", func(ctx context.Context) (func(), error) {
		v, err := a.core.Forum.Board(ctx, r.Name, forum.Page{Number: max(1, r.Number), Size: 30})
		return func() { a.board = v }, err
	})
}

func (a *App) boardPage(gtx layout.Context) layout.Dimensions {
	if a.board.Name == "" && a.busy {
		return a.loadingPage(gtx, "正在加载版面…")
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.boardHeader(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(a.board.Articles) == 0 {
				return a.empty(gtx, "暂无主题")
			}
			return material.List(a.theme, &a.boardList).Layout(gtx, len(a.board.Articles), func(gtx layout.Context, i int) layout.Dimensions {
				return a.articleCard(gtx, "board:"+a.current().Name, a.board.Articles[i], a.current().Name)
			})
		}),
		layout.Rigid(a.pager),
	)
}

func (a *App) boardHeader(gtx layout.Context) layout.Dimensions {
	title := a.board.Description
	if title == "" {
		title = a.current().Name
	}
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(18), title)
									l.Font.Weight = 700
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(11),
										fmt.Sprintf("%s · 今日主题 %d · 今日发帖 %d · 在线 %d", a.current().Name,
											a.board.ThreadsTodayCount, a.board.PostTodayCount, a.board.UserOnlineCount))
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.filledPill(gtx, "board-compose", "在此版发帖", "plus")
							if clicked {
								a.openCompose(a.current().Name, "", "new")
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := "收藏版面"
							selected := bool(a.board.IsFavorite)
							if selected {
								label = "已收藏"
							}
							dims, clicked := a.pill(gtx, "board-favorite", label, "star", selected, false)
							if clicked {
								a.toggleBoardFavorite()
							}
							return dims
						}),
					)
				}),
			)
		})
	})
}

func (a *App) toggleBoardFavorite() {
	b := a.board
	a.run("更新版面收藏", func(ctx context.Context) (func(), error) {
		var err error
		if bool(b.IsFavorite) {
			_, err = a.core.Forum.RemoveFavorite(ctx, b.Name, "0")
		} else {
			_, err = a.core.Forum.AddFavorite(ctx, b.Name, "0")
		}
		return func() { a.board.IsFavorite = forum.Flag(!bool(b.IsFavorite)) }, err
	})
}

func (a *App) articleCard(gtx layout.Context, key string, v forum.Article, fallbackBoard string) layout.Dimensions {
	return a.articleTile(gtx, key, v, fallbackBoard)
}

func (a *App) openThread(board, id string) {
	if board == "" || id == "" {
		return
	}
	a.push(Route{Page: PageThread, Name: board, ID: id, Number: 1})
	a.loadThread()
}

func (a *App) loadThread() {
	r := a.current()
	a.run("加载主题", func(ctx context.Context) (func(), error) {
		v, err := a.core.Forum.Thread(ctx, r.Name, r.ID, forum.Page{Number: max(1, r.Number), Size: 30})
		if err != nil {
			return nil, err
		}
		if bool(v.Collect) {
			a.core.RememberCollection(r.Name, r.ID)
		}
		v.Collect = forum.Flag(a.core.CollectionState(r.Name, r.ID, bool(v.Collect)))
		return func() {
			a.thread = v
			if v.Pagination.CurrentPage > 0 {
				a.stack[len(a.stack)-1].Number = int(v.Pagination.CurrentPage)
			}
			_ = a.core.Local.Visit(ctx, a.core.Session().Username, local.History{Board: r.Name, ID: r.ID, Title: v.Title})
		}, nil
	})
}

func (a *App) threadPage(gtx layout.Context) layout.Dimensions {
	if a.thread.ID == "" && a.busy {
		return a.loadingPage(gtx, "正在加载主题…")
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.threadHeader(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.threadToolbar(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(a.thread.Articles) == 0 {
				return a.empty(gtx, "暂无帖子内容")
			}
			return material.List(a.theme, &a.threadList).Layout(gtx, len(a.thread.Articles), func(gtx layout.Context, i int) layout.Dimensions {
				return a.postCard(gtx, i, a.thread.Articles[i])
			})
		}),
		layout.Rigid(a.pager),
	)
}

func (a *App) threadHeader(gtx layout.Context) layout.Dimensions {
	title := a.thread.Title
	if title == "" {
		title = "帖子"
	}
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := a.thread.BoardDescription
							if label == "" {
								label = a.current().Name
							}
							dims, clicked := a.pill(gtx, "thread-board", label, "board", false, false)
							if clicked {
								a.openBoard(a.current().Name)
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if a.thread.ReplyCount > 0 {
								l := material.Label(a.theme, unit.Sp(11), fmt.Sprintf("%d 回复", a.thread.ReplyCount))
								l.Color = colorMuted
								return l.Layout(gtx)
							}
							return layout.Dimensions{}
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, unit.Sp(19), title)
					l.Font.Weight = 700
					l.MaxLines = 3
					return l.Layout(gtx)
				}),
			)
		})
	})
}

func (a *App) isThreadAuthor(v forum.Article) bool {
	root := a.thread.Author
	if root.ID == "" && len(a.thread.Articles) > 0 {
		root = a.thread.Articles[0].Author
	}
	return root.ID != "" && v.Author.ID == root.ID
}

func (a *App) threadToolbar(gtx layout.Context) layout.Dimensions {
	collected := bool(a.thread.Collect)
	collectLabel := "收藏文章"
	if collected {
		collectLabel = "已收藏"
	}
	liked := bool(a.thread.IsLiked)
	likeLabel := fmt.Sprintf("赞 %d", a.thread.LikeSum)
	if liked {
		likeLabel = fmt.Sprintf("已赞 %d", a.thread.LikeSum)
	}
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.filledPill(gtx, "thread-reply", "回复主题", "reply")
							if clicked {
								a.openCompose(a.current().Name, a.current().ID, "reply")
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.pill(gtx, "thread-collect", collectLabel, "star", collected, false)
							if clicked {
								a.collectThread()
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.pill(gtx, "thread-like", likeLabel, "like", liked, false)
							if clicked {
								a.likeThread()
							}
							return dims
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.pill(gtx, "thread-share", "复制链接", "link", false, false)
							if clicked {
								a.copyText(gtx, a.core.Forum.ThreadURL(a.current().Name, a.current().ID))
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.pill(gtx, "thread-open-board", "进入版面", "board", false, false)
							if clicked {
								a.openBoard(a.current().Name)
							}
							return dims
						}),
					)
				}),
			)
		})
	})
}

func (a *App) postCard(gtx layout.Context, index int, v forum.Article) layout.Dimensions {
	author := v.Author.DisplayName()
	body := strings.TrimSpace(content.Plain(v.Content))
	if body == "" {
		body = "(无内容)"
	}
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.avatar(gtx, v.Author, 34)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(a.theme, unit.Sp(14), author)
											l.Font.Weight = 600
											return l.Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if !a.isThreadAuthor(v) {
												return layout.Dimensions{}
											}
											return a.badgeChip(gtx, "楼主", colorAccentSoft, colorAccent)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if !bool(v.IsTop) {
												return layout.Dimensions{}
											}
											return a.badgeChip(gtx, "置顶", colorWarningSoft, colorWarning)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(11), fmt.Sprintf("#%d · %s", v.Position, formatTime(v.PostTime)))
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !bool(v.HiddenByVotedown) {
						return layout.Dimensions{}
					}
					return a.badgeChip(gtx, "此楼被踩折叠，正文仍可阅读", colorDangerSoft, colorDanger)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.postBody(gtx, v.Content, v.Attachments)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := fmt.Sprintf("赞 %d", v.LikeSum)
							selected := bool(v.IsLiked)
							if selected {
								label = fmt.Sprintf("已赞 %d", v.LikeSum)
							}
							dims, clicked := a.pill(gtx, fmt.Sprintf("like:%s", v.ID), label, "like", selected, false)
							if clicked {
								a.react(index, v, application.Like)
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := fmt.Sprintf("踩 %d", v.VotedownSum)
							selected := bool(v.IsVotedown)
							if selected {
								label = fmt.Sprintf("已踩 %d", v.VotedownSum)
							}
							dims, clicked := a.pill(gtx, fmt.Sprintf("down:%s", v.ID), label, "down", selected, true)
							if clicked {
								a.react(index, v, application.Downvote)
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.pill(gtx, fmt.Sprintf("reply:%s", v.ID), "回复", "reply", false, false)
							if clicked {
								a.openCompose(a.current().Name, string(v.ID), "reply")
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.pill(gtx, fmt.Sprintf("copy:%s", v.ID), "复制", "copy", false, false)
							if clicked {
								a.copyText(gtx, body)
							}
							return dims
						}),
					)
				}),
			)
		})
	})
}

func (a *App) likeThread() {
	root, ok := a.threadRoot()
	if !ok {
		return
	}
	if bool(root.IsLiked) {
		a.fail(errors.New("已赞过，服务器不支持取消赞"))
		return
	}
	a.run("提交评价", func(ctx context.Context) (func(), error) {
		updated, err := a.core.React(ctx, a.current().Name, root, application.Like)
		return func() {
			a.thread.LikeSum = updated.LikeSum
			a.thread.IsLiked = updated.IsLiked
			if len(a.thread.Articles) > 0 && string(a.thread.Articles[0].ID) == string(updated.ID) {
				a.thread.Articles[0] = updated
			}
		}, err
	})
}

func (a *App) threadRoot() (forum.Article, bool) {
	if len(a.thread.Articles) > 0 {
		return a.thread.Articles[0], true
	}
	if a.thread.ID == "" {
		return forum.Article{}, false
	}
	return forum.Article{ID: a.thread.ID, LikeSum: a.thread.LikeSum, IsLiked: a.thread.IsLiked}, true
}

func (a *App) react(index int, v forum.Article, kind application.ReactionKind) {
	if kind == application.Like && bool(v.IsLiked) {
		a.fail(errors.New("已赞过，服务器不支持取消赞"))
		return
	}
	if kind == application.Downvote && bool(v.IsVotedown) {
		a.fail(errors.New("已踩过，服务器不支持取消踩"))
		return
	}
	a.run("提交评价", func(ctx context.Context) (func(), error) {
		updated, err := a.core.React(ctx, a.current().Name, v, kind)
		return func() {
			if index < len(a.thread.Articles) {
				a.thread.Articles[index] = updated
			}
			if index == 0 {
				a.thread.LikeSum = updated.LikeSum
				a.thread.IsLiked = updated.IsLiked
				a.thread.VotedownSum = updated.VotedownSum
				a.thread.IsVotedown = updated.IsVotedown
			}
		}, err
	})
}

func (a *App) collectThread() {
	was := bool(a.thread.Collect)
	a.run("更新文章收藏", func(ctx context.Context) (func(), error) {
		_, err := a.core.SetCollected(ctx, a.current().Name, a.current().ID, !was)
		return func() { a.thread.Collect = forum.Flag(!was) }, err
	})
}

func (a *App) loadTimeline() {
	page := max(1, a.current().Number)
	a.run("加载时间线", func(ctx context.Context) (func(), error) {
		v, err := a.core.Forum.FavorPosts(ctx, forum.Page{Number: page, Size: 30})
		sort.SliceStable(v.Articles, func(i, j int) bool {
			return articleTime(v.Articles[i]) > articleTime(v.Articles[j])
		})
		return func() { a.articles = v.Articles; a.pagination = v.Pagination }, err
	})
}

func (a *App) timelinePage(gtx layout.Context) layout.Dimensions {
	if len(a.articles) == 0 && a.busy {
		return a.loadingPage(gtx, "正在加载时间线…")
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(18), "全部帖子")
									l.Font.Weight = 700
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(11), "按发帖时间倒序排列")
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if a.pagination.TotalItems > 0 {
								return a.badgeChip(gtx, fmt.Sprintf("%d 条", a.pagination.TotalItems), colorAccentSoft, colorAccent)
							}
							return layout.Dimensions{}
						}),
					)
				})
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(a.articles) == 0 {
				return a.empty(gtx, "时间线为空")
			}
			return material.List(a.theme, &a.timelineList).Layout(gtx, len(a.articles), func(gtx layout.Context, i int) layout.Dimensions {
				return a.articleTile(gtx, "timeline", a.articles[i], "")
			})
		}),
		layout.Rigid(a.pager),
	)
}

func (a *App) loadFavorites() {
	tab := a.favoriteTab
	a.run("加载收藏", func(ctx context.Context) (func(), error) {
		if tab == "collection" {
			v, err := a.core.Forum.Collection(ctx, "", forum.Page{Number: 1, Size: 30})
			return func() { a.collect = v }, err
		}
		v, err := a.core.Forum.Favorites(ctx, "0")
		return func() { a.favorite = v }, err
	})
}

func (a *App) favoritesPage(gtx layout.Context) layout.Dimensions {
	tabs := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.pill(gtx, "fav-tab:boards", "版面", "board", a.favoriteTab == "boards", false)
					if clicked {
						a.favoriteTab = "boards"
						a.loadFavorites()
					}
					return dims
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.pill(gtx, "fav-tab:collection", "文章收藏", "star", a.favoriteTab == "collection", false)
					if clicked {
						a.favoriteTab = "collection"
						a.loadFavorites()
					}
					return dims
				}),
			)
		})
	}
	var body layout.Widget
	if a.favoriteTab == "collection" {
		body = a.collectionBody
	} else {
		body = a.boardFavoritesBody
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(tabs),
		layout.Flexed(1, body),
	)
}

func (a *App) collectionBody(gtx layout.Context) layout.Dimensions {
	if len(a.collect.Entries) == 0 {
		return a.empty(gtx, "还没有收藏文章")
	}
	return material.List(a.theme, &a.favoriteList).Layout(gtx, len(a.collect.Entries), func(gtx layout.Context, i int) layout.Dimensions {
		e := a.collect.Entries[i]
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.avatar(gtx, e.Author, 34)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := a.click("collection:" + string(e.ID))
								clicked := btn.Clicked(gtx)
								dims := material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(14), e.Title)
									l.Font.Weight = 600
									l.MaxLines = 2
									return l.Layout(gtx)
								})
								if clicked {
									a.core.RememberCollection(e.Board, string(e.ID))
									a.openThread(e.Board, string(e.ID))
								}
								return dims
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(11),
									fmt.Sprintf("%s · 回复 %d · %s", e.Board, e.ReplyCount, formatTime(e.PostTime)))
								l.Color = colorMuted
								return l.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dims, clicked := a.pill(gtx, "remove:"+string(e.ID), "取消收藏", "trash", true, true)
						if clicked {
							a.run("取消收藏", func(ctx context.Context) (func(), error) {
								_, err := a.core.SetCollected(ctx, e.Board, string(e.ID), false)
								return func() { a.loadFavorites() }, err
							})
						}
						return dims
					}),
				)
			})
		})
	})
}

func (a *App) boardFavoritesBody(gtx layout.Context) layout.Dimensions {
	boards := flattenFavoriteBoards(a.favorite)
	if len(boards) == 0 {
		return a.empty(gtx, "还没有收藏版面")
	}
	return material.List(a.theme, &a.favoriteList).Layout(gtx, len(boards), func(gtx layout.Context, i int) layout.Dimensions {
		return a.boardCard(gtx, boards[i])
	})
}

func flattenFavoriteBoards(f forum.Favorite) []forum.Board {
	out := append([]forum.Board(nil), f.Boards...)
	for _, sub := range f.SubFavorites {
		out = append(out, flattenFavoriteBoards(sub)...)
	}
	return out
}

func (a *App) searchPage(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(20), "搜索论坛")
							l.Font.Weight = 700
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(12), "全站搜索与 AI 搜索正在开发中，当前暂不可用。")
							l.Color = colorMuted
							return l.Layout(gtx)
						}),
					)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.comingSoonCard(gtx, "全站搜索", "按标题、作者、版面检索全站主题。", "search")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.comingSoonCard(gtx, "AI 搜索", "用自然语言描述你想找的内容，由 AI 帮你定位帖子。", "hot")
		}),
	)
}

func (a *App) comingSoonCard(gtx layout.Context, title, detail, icon string) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.iconTile(gtx, icon, 42, colorAccentSoft, colorAccent)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(15), title)
									l.Font.Weight = 600
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.badgeChip(gtx, "敬请期待", colorWarningSoft, colorWarning)
								}),
							)
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
	})
}

func (a *App) loadSelf() {
	a.run("加载个人资料", func(ctx context.Context) (func(), error) {
		u, err := a.core.Forum.Self(ctx)
		return func() {
			a.user = u
			a.history = a.core.Local.History(a.core.Session().Username)
		}, err
	})
}

func (a *App) selfPage(gtx layout.Context) layout.Dimensions {
	u := a.user
	return material.List(a.theme, &a.selfList).Layout(gtx, 4, func(gtx layout.Context, i int) layout.Dimensions {
		switch i {
		case 0:
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.avatar(gtx, u, 56)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(20), u.DisplayName())
									l.Font.Weight = 700
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(11), "ID "+string(u.ID))
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.badgeChip(gtx, fmt.Sprintf("发帖 %d", u.PostCount), colorAccentSoft, colorAccent)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.badgeChip(gtx, fmt.Sprintf("积分 %d", u.Score), colorWarningSoft, colorWarning)
										}),
									)
								}),
							)
						}),
					)
				})
			})
		case 1:
			return a.sectionTitle(gtx, "最近阅读", fmt.Sprintf("%d 条", len(a.history)))
		case 2:
			if len(a.history) == 0 {
				return a.empty(gtx, "还没有阅读历史")
			}
			e := a.history[0]
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				btn := a.click("history:" + e.Board + ":" + e.ID)
				clicked := btn.Clicked(gtx)
				dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										l := material.Label(a.theme, unit.Sp(14), e.Title)
										l.Font.Weight = 600
										return l.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										l := material.Label(a.theme, unit.Sp(11), e.Board)
										l.Color = colorMuted
										return l.Layout(gtx)
									}),
								)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.drawIcon(gtx, "next", 16, colorMuted)
							}),
						)
					})
				})
				if clicked {
					a.openThread(e.Board, e.ID)
				}
				return dims
			})
		default:
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.pill(gtx, "logout", "退出登录", "back", true, true)
					if clicked {
						a.logout()
					}
					return dims
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(10), "BYR Client Android · "+a.version+" · 共享 Go 核心")
						l.Color = colorMuted
						return l.Layout(gtx)
					})
				}),
			)
		}
	})
}

func (a *App) logout() {
	a.run("退出登录", func(ctx context.Context) (func(), error) {
		err := a.core.Logout(ctx)
		return func() { a.resetSessionUI() }, err
	})
}

func (a *App) pager(gtx layout.Context) layout.Dimensions {
	r := a.current()
	current := max(1, r.Number)
	total := 1
	switch r.Page {
	case PageBoard:
		total = max(1, int(a.board.Pagination.TotalPages))
	case PageThread:
		total = max(1, int(a.thread.Pagination.TotalPages))
	case PageTimeline:
		total = max(1, int(a.pagination.TotalPages))
	}
	return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if current <= 1 {
					return layout.Dimensions{}
				}
				dims, clicked := a.textAction(gtx, "pager-prev:"+strconv.Itoa(current), "上一页")
				if clicked {
					a.changePage(current - 1)
				}
				return dims
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Caption(a.theme, fmt.Sprintf("第 %d / %d 页", current, total)).Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if current >= total {
					return layout.Dimensions{}
				}
				dims, clicked := a.textAction(gtx, "pager-next:"+strconv.Itoa(current), "下一页")
				if clicked {
					a.changePage(current + 1)
				}
				return dims
			}),
		)
	})
}

func (a *App) changePage(n int) {
	a.stack[len(a.stack)-1].Number = n
	switch a.current().Page {
	case PageBoard:
		a.loadBoard()
	case PageThread:
		a.loadThread()
	case PageTimeline:
		a.loadTimeline()
	}
}

func (a *App) empty(gtx layout.Context, text string) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Body2(a.theme, text)
		l.Color = color.NRGBA{R: 0x6C, G: 0x7C, B: 0x93, A: 0xFF}
		return l.Layout(gtx)
	})
}

func threadID(v forum.Article) string {
	if v.GroupID != "" {
		return string(v.GroupID)
	}
	return string(v.ID)
}

func articleTime(v forum.Article) int64 {
	n, _ := strconv.ParseInt(string(v.PostTime), 10, 64)
	return n
}
