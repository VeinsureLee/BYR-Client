package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sort"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/forum"
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
				// Login uses explicit Enter/IME submit or the visible button.
				intent := a.applySingleLineIntent(gtx, &a.password, true)
				if intent.RejectedPaste {
					a.fail(errors.New("密码不能包含换行，请粘贴单行密码"))
				} else if intent.Submitted {
					a.login()
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
						l.Color = colorDanger
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
				dims, clicked := a.secondaryButton(gtx, "login-account-manager", "管理已保存账号")
				if clicked {
					a.openAccountManager()
				}
				return dims
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
	if a.busy {
		return
	}
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
			a.showTopRoot()
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
		return layout.Inset{Top: unit.Dp(15), Bottom: unit.Dp(15), Left: unit.Dp(8), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.theme, label)
			l.Color = a.theme.ContrastBg
			l.TextSize = unit.Sp(13)
			return l.Layout(gtx)
		})
	})
	return dims, clicked
}

// loadHomeModule dispatches the partition root to the module currently shown.
// PageHome is shared by the favorites list and the all-sections tree, so
// refresh and retry must not assume the tree endpoint.
func (a *App) loadHomeModule() {
	if a.sectionTab == "favorites" {
		a.loadFavorites()
		return
	}
	a.loadHome()
}

func (a *App) loadHome() {
	route := a.current()
	a.run("加载分区", func(ctx context.Context) (func(), error) {
		root, err := a.core.LoadSections(ctx)
		if err != nil {
			return nil, err
		}
		return func() {
			if a.current() != route {
				return
			}
			a.sections = root.Sections
			a.section = forum.Section{}
			a.boards = nil
		}, nil
	})
}

// showAllSections switches the section tab to the root section list.
func (a *App) showAllSections() {
	if a.busy {
		return
	}
	a.sectionTab = "all"
	a.stack = []Route{{Page: PageHome}}
	if len(a.sections) == 0 {
		a.loadHome()
	}
	a.window.Invalidate()
}

// showFavoriteBoards switches the section tab to the favorite board list.
func (a *App) showFavoriteBoards() {
	if a.busy {
		return
	}
	a.sectionTab = "favorites"
	a.stack = []Route{{Page: PageHome}}
	a.favoriteTab = "boards"
	a.loadFavorites()
}

// openSection enters a section. In picker mode a board tap returns to the
// composer instead of opening the board.
func (a *App) openSection(name string, pick bool) {
	sub := ""
	if pick {
		sub = "pick"
	}
	if !a.push(Route{Page: PageSection, Name: name, Sub: sub}) {
		return
	}
	a.section = forum.Section{}
	a.boards = nil
	a.loadSectionPage()
}

func (a *App) loadSectionPage() {
	r := a.current()
	if r.Page != PageSection {
		return
	}
	if r.Name == "" {
		a.run("加载分区", func(ctx context.Context) (func(), error) {
			root, err := a.core.LoadSections(ctx)
			if err != nil {
				return nil, err
			}
			return func() { a.sections = root.Sections }, nil
		})
		return
	}
	a.run("加载分区", func(ctx context.Context) (func(), error) {
		section, err := a.core.LoadSectionTree(ctx, r.Name)
		if err != nil {
			return nil, err
		}
		return func() {
			if a.current().Page == PageSection && a.current().Name == r.Name {
				a.section = section
				a.boards = section.Boards
			}
		}, nil
	})
}

func sectionTotals(s forum.Section) (threads, posts, online int) {
	for _, b := range s.Boards {
		threads += int(b.ThreadsTodayCount)
		posts += int(b.PostTodayCount)
		online += int(b.UserOnlineCount)
	}
	for _, sub := range s.Subsections {
		t, p, o := sectionTotals(sub)
		threads += t
		posts += p
		online += o
	}
	return threads, posts, online
}

func (a *App) homePage(gtx layout.Context) layout.Dimensions {
	if a.sectionTab == "favorites" {
		title := func(gtx layout.Context) layout.Dimensions {
			return a.sectionTitle(gtx, "收藏版面", fmt.Sprintf("%d 个版面", len(a.favoriteBoards)))
		}
		if len(a.favoriteBoards) == 0 {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(a.sectionActions),
				layout.Rigid(title),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.emptyOrPending(gtx, "还没有收藏版面")
				}),
			)
		}
		rows := []layout.Widget{a.sectionActions, title}
		for _, board := range a.favoriteBoards {
			board := board
			rows = append(rows, func(gtx layout.Context) layout.Dimensions {
				return a.boardCard(gtx, board)
			})
		}
		return material.List(a.theme, &a.homeList).Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
			return rows[i](gtx)
		})
	}

	if len(a.sections) == 0 {
		if a.busy {
			return a.pendingPage(gtx, "正在加载分区…")
		}
		if a.failed {
			return a.requestErrorPage(gtx)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.sectionActions),
			layout.Rigid(a.homeOverview),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "分区", "0 个分区")
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.empty(gtx, "暂无可浏览的分区")
			}),
		)
	}

	rows := []layout.Widget{a.sectionActions, a.homeOverview}
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		return a.sectionTitle(gtx, "分区", fmt.Sprintf("%d 个分区", len(a.sections)))
	})
	for _, section := range a.sections {
		section := section
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return a.sectionEntryCard(gtx, section, false)
		})
	}
	return material.List(a.theme, &a.homeList).Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		return rows[i](gtx)
	})
}

func (a *App) sectionPage(gtx layout.Context) layout.Dimensions {
	r := a.current()
	if r.Name != "" && a.section.Name == "" && a.busy {
		return a.pendingPage(gtx, "正在加载分区…")
	}
	pick := r.Sub == "pick"
	title := "全部分区"
	detail := fmt.Sprintf("%d 个分区", len(a.sections))
	if r.Name != "" {
		title = a.section.Description
		if title == "" {
			title = r.Name
		}
		detail = fmt.Sprintf("%d 个版面", len(a.boards))
		if n := len(a.section.Subsections); n > 0 {
			detail += fmt.Sprintf(" · %d 个子分区", n)
		}
	}
	header := func(gtx layout.Context) layout.Dimensions {
		return a.sectionHeaderCard(gtx, r, title, detail, pick)
	}
	emptyText := ""
	if r.Name == "" && len(a.sections) == 0 {
		emptyText = "暂无可浏览的分区"
	} else if r.Name != "" && len(a.section.Subsections) == 0 && len(a.boards) == 0 {
		emptyText = "这个分区暂无可浏览的版面"
	}
	if emptyText != "" {
		empty := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.emptyOrPending(gtx, emptyText)
		})
		if pick {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(header), empty)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(a.sectionActions), layout.Rigid(header), empty)
	}

	rows := make([]layout.Widget, 0, len(a.sections)+len(a.section.Subsections)+len(a.boards)+2)
	if !pick {
		rows = append(rows, a.sectionActions)
	}
	rows = append(rows, header)
	if r.Name == "" {
		for _, section := range a.sections {
			section := section
			rows = append(rows, func(gtx layout.Context) layout.Dimensions {
				return a.sectionEntryCard(gtx, section, pick)
			})
		}
	} else {
		for _, sub := range a.section.Subsections {
			sub := sub
			rows = append(rows, func(gtx layout.Context) layout.Dimensions {
				return a.subsectionEntryCard(gtx, sub, pick)
			})
		}
		for _, board := range a.boards {
			board := board
			rows = append(rows, func(gtx layout.Context) layout.Dimensions {
				return a.sectionBoardCard(gtx, board, pick)
			})
		}
	}
	return material.List(a.theme, &a.sectionList).Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		return rows[i](gtx)
	})
}

func (a *App) sectionHeaderCard(gtx layout.Context, r Route, title, detail string, pick bool) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(19), title)
									l.Font.Weight = 700
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(11), detail)
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if r.Name == "" {
										return layout.Dimensions{}
									}
									threads, posts, online := sectionTotals(a.section)
									l := material.Label(a.theme, unit.Sp(11),
										fmt.Sprintf("今日主题 %d · 今日发帖 %d · 在线 %d", threads, posts, online))
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !pick {
								return layout.Dimensions{}
							}
							return a.badgeChip(gtx, "选择发帖版面", colorAccentSoft, colorAccent)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, unit.Sp(11), "选择分区后继续查看子分区与版面。")
					l.Color = colorMuted
					return l.Layout(gtx)
				}),
			)
		})
	})
}

func (a *App) selectComposeBoard(board string) {
	for i := len(a.stack) - 1; i >= 0; i-- {
		if a.stack[i].Page == PageCompose {
			a.stack = a.stack[:i+1]
			a.compose.board = board
			a.window.Invalidate()
			return
		}
	}
}

func (a *App) sectionEntryCard(gtx layout.Context, s forum.Section, pick bool) layout.Dimensions {
	title := s.Description
	if title == "" {
		title = "分区 " + string(s.Name)
	}
	btn := a.click(fmt.Sprintf("section:%s:%v", s.Name, pick))
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.tile(gtx, false, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.iconTile(gtx, "hash", 36, colorAccentSoft, colorAccent)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(15), title)
								l.Font.Weight = font.SemiBold
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(11), "查看子分区与版面")
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
		a.openSection(string(s.Name), pick)
	}
	return dims
}

func (a *App) subsectionEntryCard(gtx layout.Context, s forum.Section, pick bool) layout.Dimensions {
	title := s.Description
	if title == "" {
		title = "子分区 " + string(s.Name)
	}
	btn := a.click(fmt.Sprintf("subsection:%s:%v", s.Name, pick))
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.tile(gtx, false, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.iconTile(gtx, "board", 34, colorSurface, colorAccent)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(11)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(14), title)
								l.Font.Weight = font.SemiBold
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.theme, unit.Sp(11), "子分区")
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
		a.openSection(string(s.Name), pick)
	}
	return dims
}

func (a *App) sectionBoardCard(gtx layout.Context, b forum.Board, pick bool) layout.Dimensions {
	label := b.Description
	if label == "" {
		label = b.Name
	}
	btn := a.click(fmt.Sprintf("section-board:%s:%v", b.Name, pick))
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
								l := material.Label(a.theme, unit.Sp(14), label)
								l.Font.Weight = font.SemiBold
								return l.Layout(gtx)
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
						if pick {
							return a.badgeChip(gtx, "选此版面", colorAccentSoft, colorAccent)
						}
						return a.drawIcon(gtx, "next", 16, colorMuted)
					}),
				)
			})
		})
	})
	if clicked {
		if pick {
			a.selectComposeBoard(b.Name)
		} else {
			a.openBoard(b.Name)
		}
	}
	return dims
}

func (a *App) homeOverview(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, unit.Sp(20), "全部分区")
					l.Font.Weight = 700
					return l.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.theme, unit.Sp(11),
						fmt.Sprintf("按分区浏览 · 共 %d 个分区", len(a.sections)))
					l.Color = colorMuted
					return l.Layout(gtx)
				}),
			)
		})
	})
}

func (a *App) sectionActions(gtx layout.Context) layout.Dimensions {
	selected := "favorites"
	route := a.current()
	if module, rooted := a.homeModuleForStack(); rooted {
		selected = module
	} else if route.Page == PageTop {
		selected = "top"
		if route.Name == "recommend" {
			selected = "recommend"
		}
	} else if route.Page == PageTimeline {
		selected = "timeline"
	} else if route.Page == PageSection {
		selected = "all"
	}
	items := []moduleTab{
		{Key: "all", Label: "全部分区"},
		{Key: "favorites", Label: "收藏版面"},
		{Key: "timeline", Label: "时间线"},
		{Key: "top", Label: "今日十大"},
		{Key: "recommend", Label: "版面推荐"},
	}
	return a.moduleTabs(gtx, items, selected, func(key string) {
		a.navigateSectionModule(key)
	})
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
										l.Font.Weight = font.SemiBold
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
	if !a.push(Route{Page: PageBoard, Name: name, Number: 1}) {
		return
	}
	a.board = forum.Board{Name: name}
	a.resetBoardPinnedExpanded(a.current())
	a.loadBoard()
}

func (a *App) loadBoard() {
	r := a.current()
	a.run("加载版面", func(ctx context.Context) (func(), error) {
		v, err := a.core.LoadBoard(ctx, r.Name, forum.Page{Number: max(1, r.Number), Size: 30})
		return func() {
			if a.current() != r {
				return
			}
			a.board = v
		}, err
	})
}

func (a *App) boardPage(gtx layout.Context) layout.Dimensions {
	if a.board.Name == "" && a.busy {
		return a.pendingPage(gtx, "正在加载版面…")
	}
	var pinned, normal []forum.Article
	for _, article := range a.board.Articles {
		if bool(article.IsTop) {
			pinned = append(pinned, article)
		} else {
			normal = append(normal, article)
		}
	}
	rows := make([]layout.Widget, 0, len(a.board.Articles)+1)
	if len(pinned) > 0 {
		rows = append(rows, a.pinnedGroupCard(len(pinned)))
		if a.boardPinnedExpanded(a.current()) {
			for _, article := range pinned {
				article := article
				rows = append(rows, func(gtx layout.Context) layout.Dimensions {
					return a.articleCard(gtx, "board:"+a.current().Name, article, a.current().Name)
				})
			}
		}
	}
	for _, article := range normal {
		article := article
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return a.articleCard(gtx, "board:"+a.current().Name, article, a.current().Name)
		})
	}
	route := a.current()
	state := a.mobileList(listKey{Page: PageBoard, Name: route.Name, Sub: route.Sub, Number: max(1, route.Number)})
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.boardHeader(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(rows) == 0 {
				return a.emptyOrPending(gtx, "暂无主题")
			}
			return material.List(a.theme, state).Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
				return rows[i](gtx)
			})
		}),
		layout.Rigid(a.pager),
	)
}

func (a *App) pinnedGroupCard(count int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		btn := a.click("board-pinned-toggle")
		clicked := btn.Clicked(gtx)
		dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.tile(gtx, false, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.iconTile(gtx, "alert", 34, colorWarningSoft, colorWarning)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(11)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(a.theme, unit.Sp(14), fmt.Sprintf("置顶主题 · %d", count))
									l.Font.Weight = font.SemiBold
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									label := "默认折叠，点击展开"
									if a.boardPinnedExpanded(a.current()) {
										label = "点击收起置顶主题"
									}
									l := material.Label(a.theme, unit.Sp(11), label)
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							icon := "expand_more"
							if a.boardPinnedExpanded(a.current()) {
								icon = "expand_less"
							}
							return a.drawIcon(gtx, icon, 16, colorMuted)
						}),
					)
				})
			})
		})
		if clicked {
			route := a.current()
			a.setBoardPinnedExpanded(route, !a.boardPinnedExpanded(route))
			a.window.Invalidate()
		}
		return dims
	}
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
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
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
	if a.busy {
		return
	}
	b := a.board
	a.run("更新版面收藏", func(ctx context.Context) (func(), error) {
		_, err := a.core.SetFavorite(ctx, b.Name, "0", !bool(b.IsFavorite))
		return func() {
			a.board.IsFavorite = forum.Flag(!bool(b.IsFavorite))
			if bool(b.IsFavorite) {
				a.notice("已取消收藏版面")
			} else {
				a.notice("已收藏版面")
			}
		}, err
	})
}

func (a *App) articleCard(gtx layout.Context, key string, v forum.Article, fallbackBoard string) layout.Dimensions {
	return a.articleTile(gtx, key, v, fallbackBoard)
}

func (a *App) openThread(board, id string) {
	if board == "" || id == "" {
		return
	}
	if !a.push(Route{Page: PageThread, Name: board, ID: id, Number: 1}) {
		return
	}
	a.thread = forum.Thread{}
	a.threadModels = nil
	a.threadLoadedRoute = Route{}
	a.resetThreadExpanded(a.current())
	a.loadThread()
}

func (a *App) loadThread() {
	a.threadRefreshPending = false
	a.threadRetryPage = 0
	a.threadRetryRefresh = false
	a.loadThreadPage(max(1, a.current().Number))
}

func (a *App) loadThreadPage(page int) {
	if a.core == nil {
		return
	}
	if a.busy && !a.cancelRead() {
		return
	}
	r := a.current()
	if r.Page != PageThread {
		return
	}
	if page < 1 {
		page = max(1, r.Number)
	}
	refresh := a.threadRefreshPending
	a.threadRefreshPending = false
	a.threadRequestPage = page
	a.threadRetryPage = page
	a.threadRetryRefresh = refresh
	if a.threadLoadedRequest.Name != r.Name || a.threadLoadedRequest.ID != r.ID {
		a.thread = forum.Thread{}
		a.threadModels = nil
		a.threadLoadedRequest = Route{}
		a.resetThreadExpanded(r)
	}
	a.threadMoreOpen = false
	token := a.core.HistoryReadToken()
	a.threadReqSerial++
	req := threadReadRequest{Serial: a.threadReqSerial, Generation: token.Generation, Route: r, HistoryToken: token}
	a.threadReq = req
	account := token.Username
	a.runRead("加载主题", func(ctx context.Context) (func(), error) {
		detail, err := a.core.ReadThread(ctx, r.Name, r.ID, forum.Page{Number: page, Size: 30})
		if errors.Is(err, application.ErrStaleSession) {
			err = context.Canceled
		}
		if err != nil {
			return nil, err
		}
		return func() {
			if req.Serial != a.threadReqSerial || a.current() != r {
				return
			}
			now := a.core.SessionInfo()
			if now.Generation != req.Generation || !strings.EqualFold(now.Username, account) {
				return
			}
			displayPage := max(1, page)
			if detail.Thread.Pagination.CurrentPage > 0 {
				displayPage = int(detail.Thread.Pagination.CurrentPage)
			}
			a.stack[len(a.stack)-1].Number = displayPage
			if refresh {
				a.stack[len(a.stack)-1].Sub = ""
			}
			a.thread = detail.Thread
			a.prepareThreadContent(detail.Thread)
			a.threadLoadedRoute = Route{Page: PageThread, Name: detail.CanonicalBoard, ID: r.ID, Number: displayPage}
			a.threadLoadedRequest = Route{Page: PageThread, Name: r.Name, ID: r.ID, Number: displayPage}
			a.threadRequestPage = displayPage
			a.threadRetryPage = 0
			a.threadRetryRefresh = false
			a.syncThreadExtras()
			if refresh {
				key := listKey{Page: PageThread, Name: a.current().Name, ID: a.current().ID, Number: displayPage}
				state := a.mobileList(key)
				state.Position = layout.Position{}
				a.threadAnchors[key] = threadAnchor{Signature: threadArticleSignature(detail.Thread.Articles)}
				a.threadMedia[key] = a.mediaGeneration
				a.notice("刷新成功")
			} else if target := a.current().Sub; target != "" {
				for index, article := range detail.Thread.Articles {
					if string(article.ID) == target {
						key := listKey{Page: PageThread, Name: a.current().Name, ID: a.current().ID, Sub: target, Number: displayPage}
						state := a.mobileList(key)
						state.Position.First = index + 1
						state.Position.Offset = 0
						a.threadAnchors[key] = threadAnchor{Row: index + 1, ArticleID: string(article.ID), Signature: threadArticleSignature(detail.Thread.Articles)}
						break
					}
				}
			}
			go a.recordThreadVisit(req, detail)
		}, nil
	})
}

func (a *App) recordThreadVisit(req threadReadRequest, detail application.ThreadDetail) {
	err := a.core.RecordThreadVisit(a.ctx, req.HistoryToken, detail.CanonicalBoard, req.Route.ID, detail.Thread.Title)
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, application.ErrStaleSession) {
		return
	}
	a.post(func() {
		if a.closed.Load() || a.current().Page != PageThread || a.current().ID != req.Route.ID {
			return
		}
		// A local history failure must leave the loaded floors readable.
		a.message = "阅读历史保存失败：" + err.Error()
		a.failed = false
		a.noticeGeneration++
	})
}

func threadArticleSignature(articles []forum.Article) string {
	var b strings.Builder
	b.Grow(len(articles) * 8)
	for _, article := range articles {
		b.WriteString(string(article.ID))
		b.WriteByte('|')
	}
	return b.String()
}

func (a *App) threadPage(gtx layout.Context) layout.Dimensions {
	if a.thread.ID == "" && a.busy {
		return a.pendingPage(gtx, "正在加载主题…")
	}
	route := a.current()
	key := listKey{Page: PageThread, Name: route.Name, ID: route.ID, Sub: route.Sub, Number: max(1, route.Number)}
	state := a.mobileList(key)
	if a.threadAnchors == nil {
		a.threadAnchors = make(map[listKey]threadAnchor)
	}
	// A route transition clears the thread model and queues an async reload.
	// Do not treat that empty frame as new data: it would overwrite the saved
	// floor ID before the reload can restore it.
	if a.thread.ID != "" {
		signature := threadArticleSignature(a.thread.Articles)
		saved, hasAnchor := a.threadAnchors[key]
		viewport := threadViewportKey{Width: gtx.Constraints.Max.X, Scale: gtx.Metric.PxPerSp}
		savedViewport, viewportSeen := a.threadViewports[key]
		dataChanged := hasAnchor && saved.Signature != signature
		if !dataChanged {
			// Rows and IDs still describe the same data, so the list position is
			// the user's current reading point. Capture it before a viewport or
			// media change restores that point.
			saved = threadAnchor{Row: state.Position.First, Offset: state.Position.Offset, Signature: signature}
			if saved.Row > 0 && saved.Row-1 < len(a.thread.Articles) {
				saved.ArticleID = string(a.thread.Articles[saved.Row-1].ID)
			}
			a.threadAnchors[key] = saved
			hasAnchor = true
		}
		restoreAnchor := !viewportSeen || savedViewport != viewport ||
			a.threadMedia[key] != a.mediaGeneration || dataChanged
		// Restore by the old article ID before recording the new list position.
		// When rows move (a floor is inserted above), the stored row number is
		// stale but the ID and offset still identify the same reading point.
		if restoreAnchor && hasAnchor {
			row, offset := saved.Row, saved.Offset
			if saved.ArticleID != "" {
				found := false
				for index, article := range a.thread.Articles {
					if string(article.ID) == saved.ArticleID {
						row, found = index+1, true
						break
					}
				}
				if !found {
					// The target floor was deleted: keep the nearest valid row.
					row = min(max(saved.Row, 0), len(a.thread.Articles))
				}
			}
			if row < 0 {
				row = 0
			}
			state.Position.First = row
			state.Position.Offset = offset
		}
		anchor := threadAnchor{Row: state.Position.First, Offset: state.Position.Offset, Signature: signature}
		if anchor.Row > 0 && anchor.Row-1 < len(a.thread.Articles) {
			anchor.ArticleID = string(a.thread.Articles[anchor.Row-1].ID)
		}
		a.threadAnchors[key] = anchor
		a.threadViewports[key] = viewport
		a.threadMedia[key] = a.mediaGeneration
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.threadMoreMenu(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(a.thread.Articles) == 0 {
				return a.emptyOrPending(gtx, "暂无帖子内容")
			}
			rows := 1 + len(a.thread.Articles)
			return material.List(a.theme, state).Layout(gtx, rows, func(gtx layout.Context, i int) layout.Dimensions {
				if i == 0 {
					return a.compactThreadHeader(gtx)
				}
				return a.postCard(gtx, i-1, a.thread.Articles[i-1])
			})
		}),
		layout.Rigid(a.pager),
	)
}

func (a *App) compactThreadHeader(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := a.thread.BoardDescription
							if label == "" {
								label = a.threadBoard()
							}
							if label == "" {
								label = "未知版面"
							}
							dims, clicked := a.pill(gtx, "thread-board", label, "board", false, false)
							if clicked {
								a.openBoard(a.threadBoard())
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							meta := make([]string, 0, 2)
							if a.thread.ReplyCount > 0 {
								meta = append(meta, fmt.Sprintf("%d 回复", a.thread.ReplyCount))
							}
							if t := formatTime(a.thread.LastReplyTime); t != "" {
								meta = append(meta, "最后回复 "+t)
							}
							if len(meta) == 0 {
								return layout.Dimensions{}
							}
							l := material.Label(a.theme, unit.Sp(12), strings.Join(meta, " · "))
							l.Color = colorFaint
							l.MaxLines = 1
							l.Truncator = "…"
							return l.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := a.threadTitle()
					if title == "" {
						title = "帖子"
					}
					l := material.Label(a.theme, unit.Sp(22), title)
					l.Font.Weight = font.SemiBold
					l.LineHeight = unit.Sp(30)
					return l.Layout(gtx)
				}),
			)
		})
	})
}

func (a *App) threadTitle() string {
	if title := strings.TrimSpace(a.thread.Title); title != "" {
		return title
	}
	if len(a.thread.Articles) > 0 {
		return a.thread.Articles[0].Title
	}
	return ""
}

func (a *App) isThreadAuthor(v forum.Article) bool {
	root := a.thread.Author
	if root.ID == "" && len(a.thread.Articles) > 0 {
		root = a.thread.Articles[0].Author
	}
	return root.ID != "" && v.Author.ID == root.ID
}

func (a *App) postCard(gtx layout.Context, index int, v forum.Article) layout.Dimensions {
	author := v.Author.DisplayName()
	pinned := bool(v.IsTop)
	postKey := threadExpansionKey(a.current(), string(v.ID))
	expanded := a.expandedPosts[postKey]
	collapsed := pinned && !expanded
	model := a.threadContent(v)
	body := strings.TrimSpace(model.plain)
	if body == "" {
		body = "(无内容)"
	}
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if v.Author.Anonymous() || v.Author.ID == "" {
								return a.avatar(gtx, v.Author, 34)
							}
							btn := a.click("post-user:" + strconv.Itoa(index))
							clicked := btn.Clicked(gtx)
							dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.avatar(gtx, v.Author, 34)
							})
							if clicked {
								a.openUser(string(v.Author.ID))
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											label := func(gtx layout.Context) layout.Dimensions {
												l := material.Label(a.theme, unit.Sp(14), author)
												l.Font.Weight = font.SemiBold
												if !v.Author.Anonymous() && v.Author.ID != "" {
													l.Color = colorAccent
												}
												return l.Layout(gtx)
											}
											if v.Author.Anonymous() || v.Author.ID == "" {
												return label(gtx)
											}
											btn := a.click("post-author:" + strconv.Itoa(index))
											clicked := btn.Clicked(gtx)
											dims := btn.Layout(gtx, label)
											if clicked {
												a.openUser(string(v.Author.ID))
											}
											return dims
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
									meta := fmt.Sprintf("#%d · %s", v.Position, formatTime(v.PostTime))
									if model.replyPosition > 0 {
										meta = fmt.Sprintf("#%d · %s · 回复 #%d %s", v.Position, formatTime(v.PostTime), model.replyPosition, model.replyAuthor)
									}
									l := material.Label(a.theme, unit.Sp(12), meta)
									l.Color = colorFaint
									l.MaxLines = 1
									l.Truncator = "…"
									return l.Layout(gtx)
								}),
							)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !pinned {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !collapsed {
									return layout.Dimensions{}
								}
								return a.badgeChip(gtx, "置顶内容已折叠", colorWarningSoft, colorWarning)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := "展开置顶内容"
								icon := "expand_more"
								if !collapsed {
									label = "收起置顶内容"
									icon = "expand_less"
								}
								dims, clicked := a.pill(gtx, fmt.Sprintf("pin-toggle:%s:%d", v.ID, index), label, icon, false, false)
								if clicked {
									a.expandedPosts[postKey] = collapsed
									a.window.Invalidate()
								}
								return dims
							}),
						)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if collapsed {
						return layout.Dimensions{}
					}
					return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if collapsed || !bool(v.HiddenByVotedown) {
						return layout.Dimensions{}
					}
					return a.badgeChip(gtx, "此楼被踩折叠，正文仍可阅读", colorDangerSoft, colorDanger)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if collapsed {
						return layout.Dimensions{}
					}
					return a.postBodyContent(gtx, model)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if collapsed {
						return layout.Dimensions{}
					}
					return layout.Spacer{Height: unit.Dp(8)}.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if collapsed {
						return layout.Dimensions{}
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							selected := bool(v.IsLiked)
							description := "赞"
							if selected {
								description = "已赞"
							}
							dims, clicked := a.compactIconAction(gtx, fmt.Sprintf("like:%s", v.ID), description, "like", "like_on", int(v.LikeSum), selected, false)
							if clicked && !a.busy {
								a.react(index, v, application.Like)
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							selected := bool(v.IsVotedown)
							description := "踩"
							if selected {
								description = "已踩"
							}
							dims, clicked := a.compactIconAction(gtx, fmt.Sprintf("down:%s", v.ID), description, "down", "down_on", int(v.VotedownSum), selected, true)
							if clicked && !a.busy {
								a.react(index, v, application.Downvote)
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.compactIconAction(gtx, fmt.Sprintf("reply:%s", v.ID), "回复", "reply", "", 0, false, false)
							if clicked && !a.busy {
								a.openCompose(a.threadBoard(), string(v.ID), "reply")
							}
							return dims
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dims, clicked := a.compactIconAction(gtx, fmt.Sprintf("copy:%s", v.ID), "复制", "copy", "", 0, false, false)
							if clicked && !a.busy {
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

func (a *App) threadBoard() string {
	if board := strings.TrimSpace(a.thread.Board); board != "" {
		return board
	}
	if len(a.thread.Articles) > 0 {
		if board := strings.TrimSpace(a.thread.Articles[0].Board); board != "" {
			return board
		}
	}
	return a.current().Name
}

func (a *App) likeThread() {
	if a.busy {
		return
	}
	root, ok := a.threadRoot()
	if !ok || root.ID == "" {
		a.fail(errors.New("主题还在加载，请稍候再点赞"))
		return
	}
	if bool(root.IsLiked) {
		a.fail(errors.New("已赞过，服务器不支持取消赞"))
		return
	}
	board := a.threadBoard()
	a.run("提交评价", func(ctx context.Context) (func(), error) {
		updated, err := a.core.React(ctx, board, root, application.Like)
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
	if a.busy {
		return
	}
	if v.ID == "" {
		a.fail(errors.New("帖子还在加载，请稍候再评价"))
		return
	}
	if kind == application.Like && bool(v.IsLiked) {
		a.fail(errors.New("已赞过，服务器不支持取消赞"))
		return
	}
	if kind == application.Downvote && bool(v.IsVotedown) {
		a.fail(errors.New("已踩过，服务器不支持取消踩"))
		return
	}
	board := a.threadBoard()
	a.run("提交评价", func(ctx context.Context) (func(), error) {
		updated, err := a.core.React(ctx, board, v, kind)
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
	if a.busy {
		return
	}
	if a.thread.ID == "" {
		a.fail(errors.New("主题还在加载，请稍候再收藏"))
		return
	}
	was := bool(a.thread.Collect)
	board, id := a.threadBoard(), a.current().ID
	a.run("更新文章收藏", func(ctx context.Context) (func(), error) {
		_, err := a.core.SetCollected(ctx, board, id, !was)
		return func() {
			a.thread.Collect = forum.Flag(!was)
			if was {
				a.notice("已取消收藏文章")
			} else {
				a.notice("已收藏文章")
			}
		}, err
	})
}

func (a *App) loadTimeline() {
	page := max(1, a.current().Number)
	a.run("加载时间线", func(ctx context.Context) (func(), error) {
		v, err := a.core.LoadTimeline(ctx, forum.Page{Number: page, Size: 30})
		sort.SliceStable(v.Articles, func(i, j int) bool {
			return articleTime(v.Articles[i]) > articleTime(v.Articles[j])
		})
		return func() { a.articles = v.Articles; a.pagination = v.Pagination }, err
	})
}

func (a *App) timelinePage(gtx layout.Context) layout.Dimensions {
	if len(a.articles) == 0 && a.busy {
		return a.pendingPage(gtx, "正在加载时间线…")
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
				return a.emptyOrPending(gtx, "时间线为空")
			}
			return material.List(a.theme, &a.timelineList).Layout(gtx, len(a.articles), func(gtx layout.Context, i int) layout.Dimensions {
				return a.articleTile(gtx, "timeline", a.articles[i], "")
			})
		}),
		layout.Rigid(a.pager),
	)
}

func (a *App) openFavorites() {
	if !a.push(Route{Page: PageFavorites, Number: 1}) {
		return
	}
	a.favoriteTab = "boards"
	a.loadFavorites()
}

func (a *App) loadFavorites() {
	tab := a.favoriteTab
	page := max(1, a.current().Number)
	a.run("加载收藏", func(ctx context.Context) (func(), error) {
		if tab == "collection" {
			v, err := a.core.LoadCollection(ctx, "", forum.Page{Number: page, Size: 30})
			return func() { a.collect = v; a.pagination = v.Pagination }, err
		}
		favorite, boards, err := a.core.LoadFavoriteBoards(ctx, "0")
		if err != nil {
			return nil, err
		}
		return func() {
			a.favorite = favorite
			a.favoriteBoards = boards
			a.pagination = forum.Pagination{}
		}, nil
	})
}

func (a *App) selectFavoriteTab(tab string) {
	if a.busy || a.favoriteTab == tab {
		return
	}
	a.favoriteTab = tab
	a.stack[len(a.stack)-1].Number = 1
	a.favoriteList = widget.List{List: layout.List{Axis: layout.Vertical}}
	a.loadFavorites()
}

func (a *App) openCollectionEntry(e forum.CollectionEntry) {
	if e.Board == "" || e.ID == "" {
		return
	}
	a.core.RememberCollection(e.Board, string(e.ID))
	a.openThread(e.Board, string(e.ID))
}

func (a *App) favoritesPage(gtx layout.Context) layout.Dimensions {
	tabs := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.pill(gtx, "fav-tab:boards", "版面", "board", a.favoriteTab == "boards", false)
					if clicked {
						a.selectFavoriteTab("boards")
					}
					return dims
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims, clicked := a.pill(gtx, "fav-tab:collection", "文章收藏", "star", a.favoriteTab == "collection", false)
					if clicked {
						a.selectFavoriteTab("collection")
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
		layout.Rigid(a.pager),
	)
}

func (a *App) collectionBody(gtx layout.Context) layout.Dimensions {
	if len(a.collect.Entries) == 0 {
		return a.emptyOrPending(gtx, "还没有收藏文章")
	}
	return material.List(a.theme, &a.favoriteList).Layout(gtx, len(a.collect.Entries), func(gtx layout.Context, i int) layout.Dimensions {
		e := a.collect.Entries[i]
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						btn := a.click("collection:" + e.Board + ":" + string(e.ID))
						clicked := btn.Clicked(gtx)
						dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.avatar(gtx, e.Author, 34)
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											title := e.Title
											if title == "" {
												title = "无标题"
											}
											l := material.Label(a.theme, unit.Sp(14), title)
											l.Font.Weight = font.SemiBold
											l.MaxLines = 2
											return l.Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(a.theme, unit.Sp(11),
												fmt.Sprintf("%s · 回复 %d · %s", e.Board, e.ReplyCount, formatTime(e.PostTime)))
											l.Color = colorMuted
											return l.Layout(gtx)
										}),
									)
								}),
							)
						})
						if clicked {
							a.openCollectionEntry(e)
						}
						return dims
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dims, clicked := a.pill(gtx, "remove:"+e.Board+":"+string(e.ID), "取消收藏", "trash", true, true)
						if clicked && !a.busy {
							board, id := e.Board, string(e.ID)
							a.run("取消收藏", func(ctx context.Context) (func(), error) {
								_, err := a.core.SetCollected(ctx, board, id, false)
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
	boards := a.favoriteBoards
	if boards == nil {
		boards = flattenFavoriteBoards(a.favorite)
	}
	if len(boards) == 0 {
		return a.emptyOrPending(gtx, "还没有收藏版面")
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

func (a *App) loadSelf() {
	a.run("加载个人资料", func(ctx context.Context) (func(), error) {
		u, err := a.core.Self(ctx)
		return func() {
			a.user = u
			a.history = a.core.RecentHistory()
		}, err
	})
}

func (a *App) selfPage(gtx layout.Context) layout.Dimensions {
	if a.user.ID == "" && a.failed && !a.busy {
		return a.requestErrorPage(gtx)
	}
	u := a.user
	rows := make([]layout.Widget, 0, 4)
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
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
								detail := "ID " + string(u.ID)
								if u.ID == "" {
									detail = a.core.SessionInfo().Username
								}
								l := material.Label(a.theme, unit.Sp(11), detail)
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
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !bool(u.IsOnline) {
											return layout.Dimensions{}
										}
										return a.badgeChip(gtx, "在线", colorAccentSoft, colorAccent)
									}),
								)
							}),
						)
					}),
				)
			})
		})
	})
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.theme, unit.Sp(14), "我的社区")
						l.Font.Weight = 700
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.selfRow(gtx, "self-favorites", "star", "收藏", "版面收藏与文章收藏", func() {
							a.openFavorites()
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.selfRow(gtx, "self-messages", "mail", "私信", "私信会话与回复提醒", func() {
							a.openMessages("contacts")
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.selfRow(gtx, "self-accounts", "self", "账号管理", "添加、切换或移除保存的账号", func() {
							a.openAccountManager()
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.selfRow(gtx, "self-history", "clock", "阅读历史", fmt.Sprintf("%d 条阅读记录", len(a.history)), func() {
							a.openHistory()
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						themeName := "浅色"
						if a.core.Preferences().Theme == 2 {
							themeName = "深色"
						}
						return a.selfRow(gtx, "self-settings", "edit", "设置", "主题 "+themeName+" · 字号 · IPv6", func() {
							a.openSettings()
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.selfRow(gtx, "self-blacklist", "alert", "黑名单", "管理被屏蔽的用户", func() {
							a.openBlacklist()
						})
					}),
				)
			})
		})
	})
	if len(a.history) > 0 {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			e := a.history[0]
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.sectionTitle(gtx, "最近阅读", fmt.Sprintf("%d 条", len(a.history)))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := a.click("self-history-recent")
					clicked := btn.Clicked(gtx)
					dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(a.theme, unit.Sp(14), e.Title)
											l.Font.Weight = font.SemiBold
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
				}),
			)
		})
	}
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
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
	})
	return material.List(a.theme, &a.selfList).Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		return rows[i](gtx)
	})
}

func (a *App) selfRow(gtx layout.Context, key, icon, title, detail string, action func()) layout.Dimensions {
	btn := a.click(key)
	clicked := btn.Clicked(gtx)
	dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.iconTile(gtx, icon, 34, colorAccentSoft, colorAccent)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(11)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.theme, unit.Sp(14), title)
							l.Font.Weight = font.SemiBold
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
					return a.drawIcon(gtx, "next", 15, colorMuted)
				}),
			)
		})
	})
	if clicked && !a.busy {
		action()
	}
	return dims
}

func (a *App) logout() {
	a.run("退出登录", func(ctx context.Context) (func(), error) {
		err := a.core.Logout(ctx)
		return func() { a.resetSessionUI() }, err
	})
}

func (a *App) pager(gtx layout.Context) layout.Dimensions {
	r := a.current()
	if r.Page == PageFavorites && a.favoriteTab != "collection" {
		return layout.Dimensions{}
	}
	current := max(1, r.Number)
	total := 1
	switch r.Page {
	case PageBoard:
		total = max(1, int(a.board.Pagination.TotalPages))
	case PageThread:
		total = max(1, int(a.thread.Pagination.TotalPages))
	case PageTimeline, PageFavorites, PageBlacklist:
		total = max(1, int(a.pagination.TotalPages))
	case PageSearch:
		total = max(1, int(a.searchPagination.TotalPages))
	case PageMessages:
		if a.current().Sub == "contacts" {
			return layout.Dimensions{}
		}
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
	if n < 1 {
		n = 1
	}
	if a.current().Page == PageSearch {
		// Search keeps the committed page and results until the target page
		// succeeds, so a failed page read leaves the previous list usable.
		a.loadSearchPage(n)
		return
	}
	a.stack[len(a.stack)-1].Number = n
	route := a.current()
	a.clearRouteModel(route)
	a.resetRoutePosition(route)
	switch route.Page {
	case PageBoard:
		a.loadBoard()
	case PageThread:
		a.loadThread()
	case PageTimeline:
		a.loadTimeline()
	case PageFavorites:
		a.loadFavorites()
	case PageBlacklist:
		a.loadBlacklist()
	case PageMessages:
		a.loadMessages()
	case PageSearch:
		a.loadSearch()
	}
}

// centeredContent centers a block in the content viewport. When the block is
// taller than the viewport (large text or a soft keyboard), the whole empty
// state becomes a one-row scroll list so its text and actions stay reachable.
func (a *App) centeredContent(gtx layout.Context, content layout.Widget) layout.Dimensions {
	width := gtx.Constraints.Max.X
	viewport := gtx.Constraints.Max.Y
	measure := gtx
	measure.Source = measure.Source.Disabled()
	measure.Constraints = layout.Constraints{Max: image.Pt(width, 1<<20)}
	a.measuringEmpty = true
	macro := op.Record(gtx.Ops)
	natural := content(measure)
	macro.Stop()
	a.measuringEmpty = false
	return a.emptyList.List.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		if natural.Size.Y <= viewport {
			gtx.Constraints.Min.Y = viewport
			return layout.Center.Layout(gtx, content)
		}
		gtx.Constraints.Min.Y = natural.Size.Y
		gtx.Constraints.Max.Y = natural.Size.Y
		return content(gtx)
	})
}

func (a *App) empty(gtx layout.Context, title string) layout.Dimensions {
	icon, detail := emptyCopy(title)
	// Fill the content viewport horizontally before centering. Gio's Center
	// only expands to the incoming minimum, and a Flexed child receives a zero
	// horizontal minimum, so leaving it unset puts the block at the left edge.
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return a.centeredContent(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(24), Right: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.drawIcon(gtx, icon, 32, colorMuted)
					})
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					l := material.Label(a.theme, unit.Sp(17), title)
					l.Font.Weight = font.SemiBold
					l.Alignment = text.Middle
					l.Color = a.theme.Palette.Fg
					l.MaxLines = 2
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					l := material.Body2(a.theme, detail)
					l.Alignment = text.Middle
					l.Color = colorMuted
					l.MaxLines = 3
					return l.Layout(gtx)
				}),
			)
		})
	})
}

func emptyCopy(title string) (icon, detail string) {
	switch title {
	case "还没有阅读记录":
		return "clock", "打开主题后，会在这里显示最近阅读记录。"
	case "黑名单为空":
		return "user", "将用户加入黑名单后，会在这里显示。"
	case "还没有收藏文章":
		return "star", "在主题阅读页收藏文章后，会显示在这里。"
	case "还没有收藏版面":
		return "star", "在版面页收藏常用版面后，会显示在这里。"
	case "没有找到用户":
		return "user", "检查完整用户名或论坛 ID 后重试。"
	case "没有找到主题":
		return "search", "换一个标题关键词试试。"
	case "暂无主题":
		return "message", "这里还没有可以显示的主题。"
	case "输入关键词开始搜索":
		return "search", "全站标题或人物查找结果会显示在这里。"
	case "输入完整论坛 ID 或用户名查找人物":
		return "user", "输入完整论坛 ID 或用户名，然后点击查找人物。"
	case "AI 搜索尚未配置":
		return "alert", "可以改用全站搜索或人物查找。"
	case "选择消息模块":
		return "mail", "在上方选择私信或提醒。"
	case "还没有私信":
		return "mail", "给论坛用户发送私信后，会在这里显示。"
	case "信箱是空的":
		return "mail", "收到的站内信会显示在这里。"
	case "还没有会话消息":
		return "mail", "发送第一封私信开始聊天。"
	case "暂未提醒":
		return "clock", "有人回复或 @ 你时，会显示在这里。"
	case "时间线为空":
		return "clock", "论坛没有返回可显示的帖子，稍后刷新试试。"
	case "暂无可浏览的分区":
		return "board", "当前没有可显示的分区。"
	case "这个分区暂无可浏览的版面":
		return "board", "选择其他分区，或稍后重新读取。"
	default:
		return "alert", "内容会显示在这里。"
	}
}

func threadID(v forum.Article) string {
	if v.GroupID != "" {
		return string(v.GroupID)
	}
	return string(v.ID)
}

func articleTime(v forum.Article) int64 {
	n, err := strconv.ParseInt(string(v.PostTime), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
