package desktop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	svc "BYR-Client/internal/app"
	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
)

func (u *UI) browseContent(c *ui.Context) {
	key := "browse:" + string(u.browseBoard) + ":" + string(u.browseNode)
	ui.Column(c).Key(key).Grow(1).FillWidth().
		Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Y: 8}, Exit: &ui.Motion{Y: -6}}).
		Children(func() {
			if u.browseBoard != "" {
				u.boardContent(c)
				return
			}
			if u.browseNode == "" {
				u.forumOverview(c)
				return
			}
			u.nodeContent(c)
		})
}

func (u *UI) forumOverview(c *ui.Context) {
	t := c.Theme()
	nav := u.app.Navigation()
	if nav == nil {
		ui.Text(c, "目录尚未初始化").Padding(24)
		return
	}
	roots, err := nav.Children("")
	if err != nil {
		ui.Text(c, err.Error()).Padding(24)
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		u.requestTraffic("")
		ui.Column(c).MaxWidth(1010).Margin(0, ui.Auto).Padding(30).Gap(18).Children(func() {
			ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "浏览论坛").FontSize(u.fs(26)).Bold()
				ui.Spacer(c)
				ui.Button(c, "刷新论坛概览").FontSize(u.fs(11)).OnClick(func() {
					u.loadTraffic("", true)
					for _, root := range roots {
						u.loadLatestTopic(root.ID, true)
					}
				})
			})
			ui.Text(c, "从左侧目录进入分区，点击版面才读取该版主题列表，打开主题才读取正文与回复。首次进入会统计各版今日发帖与流量。").
				TextColor(t.TextMuted)
			if stats := u.app.CatalogStats(); len(stats.Warnings) > 0 {
				ui.Text(c, fmt.Sprintf("目录部分加载：%d 条警告。可在设置中查看详情。", len(stats.Warnings))).
					FontSize(u.fs(11)).TextColor(t.Warning)
			}
			ui.Row(c).Gap(14).Wrap().Children(func() {
				for _, node := range roots {
					u.forumCard(c, node)
				}
			})
			ui.Box(c).Height(8)
			summary := "今日统计加载中…"
			if traffic, ok := u.app.TrafficForNode(""); ok {
				summary = fmt.Sprintf("今日全站发帖 %d · 在线 %d", traffic.TodayPosts, traffic.OnlineUsers)
			}
			ui.Textf(c, "当前目录共 %d 个顶层 forum · %s", len(roots), summary).
				FontSize(u.fs(11)).TextColor(t.TextMuted)
		})
	})
}

func (u *UI) forumCard(c *ui.Context, node forum.Node) {
	t := c.Theme()
	u.requestLatestTopic(node.ID)
	traffic, known := u.app.TrafficForNode(node.ID)
	loading := u.app.TrafficLoading("") || u.app.TrafficLoading(node.ID)
	latest, latestBoard, hasLatest := u.app.LatestTopic(node.ID)
	cardWidth := float32(300)
	if u.fontScale > 1.2 {
		cardWidth = 410
	} else if u.fontScale > 1.05 {
		cardWidth = 360
	}
	card := ui.Column(c).Width(cardWidth).Padding(18).Gap(9).Radius(10).
		Background(t.Surface).Border(1, t.Border).Cursor(ui.CursorPointer).
		OnClick(func() { u.selectNode(node.ID) })
	card = u.interactive(c, card, false, t.Surface)
	card.Transition(ui.ElementTransition{Colors: true, Enter: &ui.Motion{Y: 6}, Animation: &ui.Snappy})
	card.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconFolder).FontSize(u.fs(17)).TextColor(t.Accent)
			ui.Text(c, node.Name).Bold().FontSize(u.fs(15))
			ui.Spacer(c)
		})
		if known {
			ui.Textf(c, "今日发帖 %d", traffic.TodayPosts).FontSize(u.fs(11)).Bold().
				TextColor(t.Accent).Padding(2, 8).Radius(999).Background(t.Selection).
				Shrink(0).AlignSelf(ui.Start)
		}
		if hasLatest {
			ui.Text(c, truncate(latest.Title, 60)).FontSize(u.fs(13)).Bold().MaxLines(2)
			boardName := u.app.BoardName(latestBoard)
			latestAt := latest.LastReplyAt
			if latestAt.IsZero() {
				latestAt = latest.CreatedAt
			}
			ui.Textf(c, "%s · %s · %s", latest.Author, relativeTime(latestAt), boardName).
				FontSize(u.fs(11)).TextColor(t.TextMuted).MaxLines(2)
		} else if u.app.LatestLoading(node.ID) {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在读取最新帖…").FontSize(u.fs(11)).TextColor(t.TextMuted)
			})
		} else if err := u.app.LatestError(node.ID); err != "" {
			ui.Text(c, "最新帖读取失败："+truncate(err, 40)).FontSize(u.fs(10)).TextColor(t.Warning)
		} else {
			ui.Text(c, "暂无最新帖").FontSize(u.fs(11)).TextColor(t.TextMuted)
		}
		if known {
			ui.Textf(c, "在线 %d · 主题 %d · 文章 %d",
				traffic.OnlineUsers, traffic.Threads, traffic.Posts).
				FontSize(u.fs(12)).TextColor(t.TextMuted)
		} else if loading {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在统计今日发帖…").FontSize(u.fs(11)).TextColor(t.TextMuted)
			})
		} else {
			ui.Text(c, "今日统计暂不可用").FontSize(u.fs(11)).TextColor(t.TextMuted)
		}
	})
}

func (u *UI) nodeContent(c *ui.Context) {
	t := c.Theme()
	path := u.app.Breadcrumb(u.browseNode)
	if len(path) == 0 {
		ui.Text(c, "节点不存在").Padding(24)
		return
	}
	node := path[len(path)-1]
	children, err := u.app.Children(node.ID)
	if err != nil {
		ui.Text(c, err.Error()).Padding(24)
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).MaxWidth(1010).Margin(0, ui.Auto).Padding(30).Gap(16).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, node.Name).FontSize(u.fs(24)).Bold()
				ui.Badge(c, kindLabel(node.Kind))
				boards, subForums := u.app.DirectoryCounts(node.ID)
				ui.Text(c, directoryCountText(boards, subForums)).FontSize(u.fs(12)).TextColor(t.TextMuted)
				ui.Spacer(c)
				ui.Button(c, "在此范围搜索").OnClick(func() {
					u.region = regionSearch
					u.scopeLabel = "当前浏览节点"
				})
			})
			ui.Text(c, nodeDescription(node.Name)).TextColor(t.TextMuted)
			ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
				u.trafficStrip(c, node.ID, false)
				ui.Spacer(c)
				ui.Button(c, "刷新流量").FontSize(u.fs(11)).OnClick(func() { u.loadTraffic(node.ID, true) })
			})
			ui.Divider(c)
			if len(children) == 0 {
				ui.Text(c, "该节点没有下级目录。").TextColor(t.TextMuted).Padding(16)
				return
			}
			ui.Column(c).Gap(6).Children(func() {
				for _, child := range children {
					u.childRow(c, child)
				}
			})
		})
	})
}

func (u *UI) childRow(c *ui.Context, node forum.Node) {
	t := c.Theme()
	row := ui.Row(c).Gap(12).Padding(14, 16).Radius(8).AlignItems(ui.Center).
		Background(t.Surface).Border(1, t.Border).Cursor(ui.CursorPointer).
		OnClick(func() { u.selectNode(node.ID) })
	row = u.interactive(c, row, false, t.Surface)
	row.Children(func() {
		icon := iconFolder
		if node.Kind == forum.KindBoard {
			icon = iconBrowse
		}
		ui.Icon(c, icon).FontSize(u.fs(16)).TextColor(t.TextMuted)
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, node.Name).FontSize(u.fs(16)).Bold()
			if node.Kind == forum.KindBoard {
				ui.Text(c, "版面 · 主题列表按需读取").FontSize(u.fs(12)).TextColor(t.TextMuted)
			} else {
				boards, subForums := u.app.DirectoryCounts(node.ID)
				ui.Text(c, directoryCountText(boards, subForums)).FontSize(u.fs(12)).TextColor(t.TextMuted)
			}
			u.childTrafficLine(c, node)
		})
		ui.Text(c, kindLabel(node.Kind)).FontSize(u.fs(11)).TextColor(t.TextMuted)
		ui.Icon(c, iconChevron).FontSize(u.fs(14)).TextColor(t.TextMuted)
	})
}

func (u *UI) boardContent(c *ui.Context) {
	t := c.Theme()
	snapshot, ok := u.app.Board(u.browseBoard)
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		ui.Row(c).Padding(12, 20).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, u.app.BoardName(u.browseBoard)).Bold().FontSize(u.fs(18))
			if ok {
				ui.Text(c, fmt.Sprintf("%d 个主题已加载", len(snapshot.Articles))).FontSize(u.fs(12)).TextColor(t.TextMuted)
				if !snapshot.ObservedAt.IsZero() {
					ui.Text(c, "· "+relativeTime(snapshot.ObservedAt)).FontSize(u.fs(12)).TextColor(t.TextMuted)
				}
			}
			ui.Spacer(c)
			if ok && snapshot.End {
				ui.Text(c, "已到本窗口末页").FontSize(u.fs(11)).TextColor(t.TextMuted)
			}
		})
		if u.browseNode != "" {
			ui.Row(c).Padding(0, 20, 6, 20).Children(func() {
				u.trafficStrip(c, u.browseNode, true)
			})
		}
		list := func() { u.boardListPane(c, snapshot, ok) }
		detail := func() {
			if u.browseThread.ArticleID == "" {
				ui.Column(c).Grow(1).Fill().Center().Gap(8).Children(func() {
					ui.Icon(c, iconBrowse).FontSize(u.fs(28)).TextColor(t.TextMuted)
					ui.Text(c, "选择一个主题").FontSize(u.fs(16)).Bold()
					ui.Text(c, "打开主题后才读取正文与所需回复。").FontSize(u.fs(12)).TextColor(t.TextMuted)
				})
				return
			}
			u.threadPane(c, u.browseThread, false, 0)
		}
		var split ui.Element
		if width, _ := c.Size(); width < 1180 {
			split = ui.SplitVertical(c, &u.splitHeight, list, detail)
		} else {
			split = ui.Split(c, &u.splitWidth, list, detail)
		}
		split.Grow(1)
		if split.Changed() {
			u.saveSettings()
		}
	})
}

func (u *UI) boardListPane(c *ui.Context, snapshot svc.BoardSnapshot, ok bool) {
	t := c.Theme()
	ui.Column(c).FillHeight().BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		if !ok || (snapshot.Loading && len(snapshot.Articles) == 0) {
			ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在读取版面列表…").FontSize(u.fs(12)).TextColor(t.TextMuted)
			})
			return
		}
		articles := snapshot.Articles
		u.boardList.Key = func(i int) any { return articles[i].Key.ArticleID }
		u.boardList.Label = func(i int) string { return articles[i].Title }
		ui.List(c, &u.boardList, len(articles), func(i int) {
			u.boardRow(c, articles[i])
		}).Grow(1).Children(func() {
			if len(articles) == 0 {
				ui.Text(c, "这个版面还没有主题").Padding(16).TextColor(t.TextMuted)
			}
		})
		ui.Row(c).Padding(8, 10).Gap(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			if snapshot.Loading {
				ui.Spinner(c)
				ui.Text(c, "加载中…").FontSize(u.fs(11)).TextColor(t.TextMuted)
			} else if snapshot.Error != "" {
				ui.Text(c, snapshot.Error).FontSize(u.fs(11)).TextColor(t.Danger).Grow(1).SingleLine()
			} else if snapshot.End {
				ui.Text(c, "已加载到本窗口末页").FontSize(u.fs(11)).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			if !snapshot.End && !snapshot.Loading {
				ui.Button(c, "加载更多主题").OnClick(func() {
					board := u.browseBoard
					application := u.app
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						_ = application.LoadMoreBoard(ctx, board)
					}()
				})
			}
		})
	})
}

func (u *UI) boardRow(c *ui.Context, article domain.Article) {
	t := c.Theme()
	selected := article.Key == u.browseThread
	row := ui.Row(c).Gap(10).Padding(11, 14).AlignItems(ui.Start).Cursor(ui.CursorPointer).
		OnClick(func() { u.openThread(article.Key) })
	row = u.interactive(c, row, selected, ui.Transparent)
	row.Children(func() {
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Row(c).Gap(7).AlignItems(ui.Start).Children(func() {
				if article.Pinned {
					ui.Badge(c, "置顶")
				}
				title := ui.Text(c, article.Title).FontSize(u.fs(14)).MaxLines(2)
				if selected {
					title.TextColor(t.Accent).Bold()
				} else {
					title.Bold()
				}
			})
			ui.Text(c, fmt.Sprintf("%s · %s · %d 回复", article.Author, relativeTime(article.CreatedAt), article.ReplyCount)).
				FontSize(u.fs(12)).TextColor(t.TextMuted).SingleLine()
		})
		if !article.LastReplyAt.IsZero() {
			ui.Column(c).Gap(2).AlignItems(ui.End).Children(func() {
				ui.Text(c, relativeTime(article.LastReplyAt)).FontSize(u.fs(11)).TextColor(t.TextMuted)
				ui.Text(c, "最后回复").FontSize(u.fs(10)).TextColor(t.TextMuted)
			})
		}
	})
}

func (u *UI) threadPane(c *ui.Context, key domain.ArticleKey, fromSearch bool, highlightFloor int) {
	t := c.Theme()
	snapshot, ok := u.app.Thread(key)
	if !ok || (snapshot.Loading && snapshot.Article.Title == "") {
		ui.Column(c).Grow(1).Fill().Center().Gap(8).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在读取主题正文…").FontSize(u.fs(12)).TextColor(t.TextMuted)
		})
		return
	}
	if snapshot.Error != "" && snapshot.Article.Title == "" {
		ui.Column(c).Grow(1).Fill().Padding(24).Gap(8).Children(func() {
			ui.Text(c, "读取失败").Bold().TextColor(t.Danger)
			ui.Text(c, snapshot.Error).TextColor(t.TextMuted)
		})
		return
	}
	scroll := &u.threadScroll
	if fromSearch {
		scroll = &u.sourceScroll
	}
	ui.Scroll(c).TrackScroll(scroll).Grow(1).Children(func() {
		ui.Column(c).Key("thread:"+string(key.ArticleID)).MaxWidth(860).Margin(0, ui.Auto).Padding(26).Gap(12).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Y: 8}}).Children(func() {
			if fromSearch {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Badge(c, "来源原文")
					ui.Text(c, joinPath(u.app.BoardPath(key.BoardID))).FontSize(u.fs(11)).TextColor(t.TextMuted)
				})
			}
			ui.Text(c, snapshot.Article.Title).FontSize(u.fs(24)).Bold().LineHeight(1.3)
			ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
				ui.Text(c, snapshot.Article.Author).FontSize(u.fs(13)).Bold()
				ui.Text(c, relativeTime(snapshot.Article.CreatedAt)).FontSize(u.fs(12)).TextColor(t.TextMuted)
				ui.Text(c, fmt.Sprintf("%d 回复", snapshot.Article.ReplyCount)).FontSize(u.fs(12)).TextColor(t.TextMuted)
				if snapshot.Loading {
					ui.Spinner(c)
				}
			})
			ui.Row(c).Gap(8).Wrap().Justify(ui.End).AlignItems(ui.Center).Children(func() {
				u.shareArticleButtons(c, key)
			})
			ui.Divider(c)
			ui.Text(c, snapshot.Article.Content).FontSize(u.fs(14)).Selectable().LineHeight(1.7)
			u.imageCollection(c, snapshot.Article.Images, 560, 420)
			ui.Row(c).Padding(6, 0).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconReply).FontSize(u.fs(15)).TextColor(t.TextMuted)
				ui.Text(c, fmt.Sprintf("回复 · 已加载 %d 条", len(snapshot.Replies))).Bold().FontSize(u.fs(14))
				ui.Spacer(c)
				if snapshot.Error != "" {
					ui.Text(c, snapshot.Error).FontSize(u.fs(11)).TextColor(t.Danger).SingleLine()
				}
			})
			tree := buildReplyTree(snapshot.Replies)
			if highlightFloor > 0 {
				if highlightFloor != u.lastHighlightFloor {
					expandReplyPath(tree, highlightFloor, u.replyOpen)
					u.lastHighlightFloor = highlightFloor
				}
			} else {
				u.lastHighlightFloor = 0
			}
			for _, node := range tree {
				u.replyTree(c, node, 0, highlightFloor)
			}
			if len(snapshot.Replies) == 0 {
				ui.Text(c, "还没有回复。").FontSize(u.fs(12)).TextColor(t.TextMuted)
			}
			ui.Row(c).Gap(8).Padding(8, 0).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if snapshot.End {
					ui.Text(c, "已加载本窗口全部回复").FontSize(u.fs(11)).TextColor(t.TextMuted)
				} else if !snapshot.Loading {
					ui.Button(c, "加载更多回复").OnClick(func() {
						application := u.app
						go func() {
							ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
							defer cancel()
							_ = application.LoadMoreReplies(ctx, key)
						}()
					})
				}
			})
		})
	})
}

func (u *UI) replyTree(c *ui.Context, node *replyNode, depth int, highlightFloor int) {
	u.replyCard(c, node.Reply, node.Reply.Floor == highlightFloor, depth, node.ParentFloor)
	if len(node.Children) == 0 {
		return
	}
	key := string(node.Reply.Key.ReplyID)
	open, exists := u.replyOpen[key]
	if !exists {
		open = true
	}
	ui.Column(c).MarginStart(float32(min(depth, 6)) * 20).Children(func() {
		ui.Collapsible(c, fmt.Sprintf("楼中楼 · %d 条回复", len(node.Children)), &open, func() {
			for _, child := range node.Children {
				u.replyTree(c, child, depth+1, highlightFloor)
			}
		})
	})
	u.replyOpen[key] = open
}

func (u *UI) replyCard(c *ui.Context, reply domain.Reply, highlighted bool, depth int, parentFloor int) {
	t := c.Theme()
	indent := float32(depth)
	if indent > 6 {
		indent = 6
	}
	card := ui.Column(c).MarginStart(indent*20).Gap(7).Padding(14, 16).Radius(8).
		Background(t.Surface).BorderWidth(1, 1, 1, 1).BorderColor(t.Border)
	if depth > 0 {
		card.BorderWidth(1, 1, 1, 3)
		card.PaddingStart(12)
	}
	if highlighted {
		card.Border(1, t.Accent)
	}
	card.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Badge(c, fmt.Sprintf("#%d", reply.Floor))
			ui.Text(c, reply.Author).FontSize(u.fs(13)).Bold()
			ui.Text(c, relativeTime(reply.CreatedAt)).FontSize(u.fs(11)).TextColor(t.TextMuted)
			if parentFloor > 0 {
				ui.Textf(c, "回复 #%d", parentFloor).FontSize(u.fs(10)).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			u.shareReplyButton(c, reply.Key.ArticleKey, reply.Floor)
		})
		if reply.QuoteAuthor != "" {
			ui.Column(c).Padding(6, 9).Radius(6).Background(t.Background).
				BorderWidth(0, 0, 0, 2).BorderColor(t.Accent).Gap(2).Children(func() {
				quoteRef := "引用 @" + reply.QuoteAuthor
				if parentFloor > 0 {
					quoteRef += fmt.Sprintf(" · #%d", parentFloor)
				}
				ui.Text(c, quoteRef).FontSize(u.fs(10)).Bold().TextColor(t.Accent)
				if reply.QuoteContent != "" {
					ui.Text(c, truncate(reply.QuoteContent, 260)).FontSize(u.fs(11)).
						TextColor(t.TextMuted).MaxLines(4)
				}
			})
		}
		ui.Text(c, reply.Content).FontSize(u.fs(14)).Selectable().LineHeight(1.6)
		u.imageCollection(c, reply.Images, 480, 320)
	})
}

func (u *UI) imageCollection(c *ui.Context, images []domain.Image, maxWidth, maxHeight float32) {
	if len(images) == 0 {
		return
	}
	t := c.Theme()
	emojis := make([]domain.Image, 0)
	photos := make([]domain.Image, 0)
	for _, image := range images {
		if image.Kind == domain.ImageEmoji {
			emojis = append(emojis, image)
		} else {
			photos = append(photos, image)
		}
	}
	if len(emojis) > 0 {
		ui.Row(c).Gap(4).Wrap().Padding(4, 6).Radius(6).Background(t.Surface).
			Border(1, t.Border).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "表情").FontSize(u.fs(9)).TextColor(t.TextMuted)
			for _, image := range emojis {
				u.imageView(c, image, 20, 20, true)
			}
		})
	}
	if len(emojis) > 0 && len(photos) > 0 {
		ui.Box(c).Height(8)
	}
	for _, image := range photos {
		u.imageView(c, image, maxWidth, maxHeight, true)
	}
}

func (u *UI) requestLatestTopic(node forum.NodeID) {
	u.loadLatestTopic(node, false)
}

func (u *UI) loadLatestTopic(node forum.NodeID, force bool) {
	if u.app == nil {
		return
	}
	if u.app.LatestLoading(node) {
		return
	}
	if !force {
		if _, _, known := u.app.LatestTopic(node); known {
			return
		}
		if u.app.LatestError(node) != "" {
			return
		}
	}
	application := u.app
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_ = application.LoadLatestTopic(ctx, node)
	}()
}

func (u *UI) requestTraffic(node forum.NodeID) {
	u.loadTraffic(node, false)
}

func (u *UI) loadTraffic(node forum.NodeID, force bool) {
	if u.app == nil {
		return
	}
	if u.app.TrafficLoading(node) {
		return
	}
	if !force {
		if _, known := u.app.TrafficForNode(node); known {
			return
		}
		if u.app.TrafficError(node) != "" {
			return
		}
	}
	application := u.app
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_ = application.LoadTraffic(ctx, node)
	}()
}

func (u *UI) trafficStrip(c *ui.Context, node forum.NodeID, compact bool) {
	t := c.Theme()
	if u.app == nil {
		return
	}
	loading := u.app.TrafficLoading(node)
	message := u.app.TrafficError(node)
	traffic, known := u.app.TrafficForNode(node)
	if loading && !known {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在读取流量…").FontSize(u.fs(12)).TextColor(t.TextMuted)
		})
		return
	}
	if !known {
		if message != "" {
			ui.Text(c, "流量读取失败："+truncate(message, 100)).FontSize(u.fs(11)).TextColor(t.Warning)
		} else if !compact {
			ui.Text(c, "暂无流量统计").FontSize(u.fs(11)).TextColor(t.TextMuted)
		}
		return
	}
	ui.Row(c).Gap(14).Wrap().AlignItems(ui.Center).Children(func() {
		u.trafficStat(c, "在线", traffic.OnlineUsers)
		u.trafficStat(c, "今日发帖", traffic.TodayPosts)
		u.trafficStat(c, "主题", traffic.Threads)
		u.trafficStat(c, "文章", traffic.Posts)
		if !traffic.ObservedAt.IsZero() && !compact {
			ui.Text(c, "更新于 "+relativeTime(traffic.ObservedAt)).FontSize(u.fs(10)).TextColor(t.TextMuted)
		}
	})
	if message != "" {
		ui.Text(c, "部分流量读取失败："+truncate(message, 100)).FontSize(u.fs(10)).TextColor(t.Warning)
	}
}

func (u *UI) trafficStat(c *ui.Context, label string, value int) {
	t := c.Theme()
	ui.Row(c).Gap(4).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, label).FontSize(u.fs(12)).TextColor(t.TextMuted).SingleLine().Shrink(0)
		ui.Textf(c, "%d", value).FontSize(u.fs(14)).Bold().SingleLine().Shrink(0)
	})
}

func (u *UI) childTrafficLine(c *ui.Context, node forum.Node) {
	if u.app == nil {
		return
	}
	t := c.Theme()
	var traffic domain.Traffic
	var known bool
	if node.Kind == forum.KindBoard {
		traffic, known = u.app.BoardTraffic(node.BoardID)
	} else {
		traffic, known = u.app.TrafficForNode(node.ID)
	}
	if !known {
		if u.app.TrafficLoading(node.ID) {
			ui.Text(c, "流量读取中…").FontSize(u.fs(11)).TextColor(t.TextMuted)
		}
		return
	}
	ui.Textf(c, "在线 %d · 今日发帖 %d · 主题 %d · 文章 %d",
		traffic.OnlineUsers, traffic.TodayPosts, traffic.Threads, traffic.Posts).
		FontSize(u.fs(10)).TextColor(t.TextMuted)
}

func directoryCountText(boards, subForums int) string {
	parts := make([]string, 0, 2)
	if boards > 0 {
		parts = append(parts, fmt.Sprintf("%d 个版面", boards))
	}
	if subForums > 0 {
		parts = append(parts, fmt.Sprintf("%d 个二级目录", subForums))
	}
	if len(parts) == 0 {
		return "暂无下级内容"
	}
	return strings.Join(parts, " · ")
}

func kindLabel(kind forum.NodeKind) string {
	switch kind {
	case forum.KindForum:
		return "forum"
	case forum.KindSubForum:
		return "sub forum"
	case forum.KindBoard:
		return "board"
	default:
		return string(kind)
	}
}

func nodeDescription(name string) string {
	return "「" + name + "」下的内容按需读取；展开目录不会批量抓取所有版面。"
}
