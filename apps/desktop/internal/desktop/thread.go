package desktop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/egoist/mygo/ui"
)

type threadRequest struct {
	Serial, Generation uint64
	Route              Route
}

type viewportKey struct {
	Width, Height, Scale float32
}

// postModel is computed when thread data arrives. Scrolling reuses the parsed
// body and reply relationship instead of parsing every visible floor per frame.
type postModel struct {
	parts         []content.Part
	plain         string
	replyPosition int
	replyAuthor   string
}

func (a *App) loadThread() {
	a.loadThreadPage(a.route.Number)
}

func (a *App) loadThreadPage(page int) {
	if page < 1 {
		page = max(1, a.route.Number)
	}
	refresh := a.threadRefreshPending
	a.threadRefreshPending = false
	a.threadForceTop = false
	a.threadRefreshClearSub = false
	a.threadRequestPage = page
	r := a.route
	a.historyNotice = ""
	a.resetThreadResourcePreflight()
	if a.threadLoadedRequest.Name != r.Name || a.threadLoadedRequest.ID != r.ID {
		a.thread = forum.Thread{}
		a.threadModels = nil
		a.threadAnchorID = ""
		a.resetThreadResourcePreflight()
	}
	a.threadScrollPending = r.Sub != "" && !refresh
	info := a.Core.SessionInfo()
	a.threadReqSerial++
	req := threadRequest{Serial: a.threadReqSerial, Generation: info.Generation, Route: r}
	a.threadReq = req
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.serial++
	serial := a.serial
	a.busy = true
	a.message = "加载主题"
	a.noticeSerial++
	a.failed = false
	a.pageError = ""
	a.pageLoading = true
	a.retryLoad = func() {
		a.threadRefreshPending = refresh
		a.loadThreadPage(page)
	}
	account := info.Username
	go func() {
		detail, err := a.Core.ReadThread(ctx, r.Name, r.ID, forum.Page{Number: page})
		if errors.Is(err, application.ErrStaleSession) {
			err = context.Canceled
		}
		a.update(func() {
			if a.closed || serial != a.serial || req.Serial != a.threadReqSerial || a.route != r {
				return
			}
			if now := a.Core.SessionInfo(); now.Generation != req.Generation || !strings.EqualFold(now.Username, account) {
				a.busy = false
				a.pageLoading = false
				if a.message == "加载主题" {
					a.message = ""
				}
				return
			}
			a.busy = false
			a.pageLoading = false
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				a.fail(err)
				// Keep content already on screen when a refresh fails; the status
				// bar offers a same-parameter retry instead of replacing the page.
				if len(a.thread.Articles) == 0 {
					a.pageError = a.message
				} else {
					a.pageError = ""
				}
				return
			}
			if a.message == "加载主题" {
				a.message = ""
			}
			a.applyThread(detail, r, page, refresh)
			a.ensureThreadResourcePreflight()
			if refresh {
				a.notice("刷新成功")
			}
			go a.recordThreadVisit(req, detail)
		})
	}()
}

func (a *App) applyThread(detail application.ThreadDetail, r Route, requested int, refresh bool) {
	oldKey := a.threadListKey()
	v := detail.Thread
	a.thread = v
	a.threadLoadedRoute = Route{Page: "thread", Name: detail.CanonicalBoard, ID: r.ID, Number: max(1, requested)}
	a.threadLoadedRequest = Route{Page: "thread", Name: r.Name, ID: r.ID, Number: max(1, requested)}
	a.threadRequestPage = max(1, requested)
	a.prepareThreadModels(v)
	if v.Pagination.CurrentPage > 0 {
		a.route.Number = int(v.Pagination.CurrentPage)
		a.threadLoadedRoute.Number = a.route.Number
		a.threadLoadedRequest.Number = a.route.Number
		a.threadRequestPage = a.route.Number
	} else {
		a.route.Number = max(1, requested)
	}
	if refresh {
		a.threadScrollPending = false
		a.threadAnchorID = ""
		a.threadForceTop = true
		a.desktopListState(oldKey).ScrollTo(0, ui.Start)
		// When the reminder target shares this page's list key, clear it only
		// after that key has rendered once at the top. This lets the saved
		// state for the displayed route settle at row 0 before the key changes.
		if r.Sub != "" && oldKey.Number == a.route.Number {
			a.threadRefreshClearSub = true
		} else {
			a.route.Sub = ""
			a.desktopListState(a.threadListKey()).ScrollTo(0, ui.Start)
		}
	}
	if a.threadScrollPending && a.route.Sub != "" {
		for index, article := range v.Articles {
			if string(article.ID) == a.route.Sub {
				state := a.desktopListState(a.threadListKey())
				state.ScrollTo(index+1, ui.Start)
				a.threadScrollPending = false
				a.threadAnchorID = string(article.ID)
				break
			}
		}
	}
}

func (a *App) prepareThreadModels(v forum.Thread) {
	models := make(map[string]*postModel, len(v.Articles))
	positions := make(map[string]forum.Article, len(v.Articles))
	for _, article := range v.Articles {
		parts := content.Parse(article.Content, article.Attachments)
		models[string(article.ID)] = &postModel{parts: parts, plain: content.PlainParts(parts)}
		positions[string(article.ID)] = article
	}
	for _, article := range v.Articles {
		model := models[string(article.ID)]
		if model == nil {
			continue
		}
		id := string(article.ReplyID)
		if id == "" || id == "0" {
			continue
		}
		if target, ok := positions[id]; ok {
			model.replyPosition = int(target.Position)
			model.replyAuthor = target.Author.DisplayName()
		}
	}
	a.threadModels = models
}

// threadModel returns the cached floor content. Tests and callers that set a
// thread directly still work: the first use parses and caches one floor.
func (a *App) threadModel(v forum.Article) *postModel {
	if a.threadModels != nil {
		if model := a.threadModels[string(v.ID)]; model != nil {
			return model
		}
	}
	parts := content.Parse(v.Content, v.Attachments)
	model := &postModel{parts: parts, plain: content.PlainParts(parts)}
	id := string(v.ReplyID)
	if id != "" && id != "0" {
		for _, article := range a.thread.Articles {
			if string(article.ID) == id {
				model.replyPosition = int(article.Position)
				model.replyAuthor = article.Author.DisplayName()
				break
			}
		}
	}
	if a.threadModels == nil {
		a.threadModels = make(map[string]*postModel)
	}
	a.threadModels[string(v.ID)] = model
	return model
}

func (a *App) recordThreadVisit(req threadRequest, detail application.ThreadDetail) {
	err := a.Core.RecordThreadVisit(a.ctx, req.Generation, detail.CanonicalBoard, req.Route.ID, detail.Thread.Title)
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, application.ErrStaleSession) {
		return
	}
	a.update(func() {
		if a.closed || a.route.Page != "thread" || a.route.ID != req.Route.ID {
			return
		}
		// History lived outside the body read all along; keep the loaded
		// floors and report only the local persistence problem. A dedicated
		// notice survives the independent vote/bet preflight status message.
		a.historyNotice = "阅读历史保存失败：" + err.Error()
		a.failed = false
	})
}

func (a *App) cancelPageRead() bool {
	if !a.pageLoading {
		return false
	}
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.serial++
	a.busy = false
	a.pageLoading = false
	a.pageError = ""
	a.failed = false
	a.message = ""
	return true
}

func (a *App) cancelThreadPreflight() {
	if a.threadResourcesCancel != nil {
		a.threadResourcesCancel()
		a.threadResourcesCancel = nil
	}
	a.threadResourcesSerial++
	a.threadResourcesBusy = false
}

func (a *App) resetThreadResourcePreflight() {
	a.cancelThreadPreflight()
	a.threadVoteAvailable = false
	a.threadBetAvailable = false
	a.threadResourcesDone = false
	a.threadResourceBoard = ""
	a.threadResourceID = ""
}

func (a *App) ensureThreadResourcePreflight() {
	if a.Core == nil || a.route.Page != "thread" || a.thread.ID == "" || a.failed {
		return
	}
	board, id := a.threadBoard(), a.route.ID
	if strings.TrimSpace(board) == "" || strings.TrimSpace(id) == "" {
		return
	}
	if a.threadResourcesBusy && a.threadResourceBoard == board && a.threadResourceID == id {
		return
	}
	if a.threadResourcesDone && a.threadResourceBoard == board && a.threadResourceID == id {
		return
	}
	generation := a.Core.SessionInfo().Generation
	a.cancelThreadPreflight()
	a.threadResourcesSerial++
	serial := a.threadResourcesSerial
	a.threadResourcesBusy = true
	a.threadResourcesDone = false
	a.threadResourceBoard = board
	a.threadResourceID = id
	ctx, cancel := context.WithCancel(a.ctx)
	a.threadResourcesCancel = cancel
	go func() {
		vote, voteErr := a.Core.Forum.Vote(ctx, id)
		bet, betErr := a.Core.Forum.Bet(ctx, id)
		a.update(func() {
			if a.closed || serial != a.threadResourcesSerial {
				return
			}
			cancel := a.threadResourcesCancel
			a.threadResourcesCancel = nil
			if cancel != nil {
				cancel()
			}
			a.threadResourcesBusy = false
			if a.Core.SessionInfo().Generation != generation ||
				a.route.Page != "thread" || a.threadResourceBoard != board || a.threadResourceID != id {
				return
			}
			a.threadVoteAvailable = voteErr == nil && vote.ID != "" && vote.ArticleID != "" && string(vote.ArticleID) == id
			a.threadBetAvailable = betErr == nil && bet.ID != "" && bet.ArticleID != "" && string(bet.ArticleID) == id
			a.threadResourcesDone = true
		})
	}()
}

func (a *App) hasLoadedThreadContent() bool {
	return a.route.Page == "thread" && a.thread.ID != "" &&
		a.threadLoadedRequest.Name == a.route.Name && a.threadLoadedRequest.ID == a.route.ID
}

func (a *App) threadView(c *ui.Context) {
	ui.Row(c).FillWidth().Grow(1).MinWidth(0).Justify(ui.Center).Children(func() {
		ui.Column(c).MaxWidth(760).Grow(1).MinWidth(0).FillHeight().Gap(12).Children(func() {
			a.threadToolbar(c)
			if len(a.thread.Articles) == 0 {
				a.threadHeader(c)
				emptyState(c, "message", "暂无帖子内容", "这个主题可能正在加载，或已被删除。")
				a.finishRefreshFrame()
				return
			}
			key := a.threadListKey()
			state := a.desktopListState(key)
			state.Key = func(i int) any {
				if i == 0 {
					return "thread-header"
				}
				if i > 0 && i-1 < len(a.thread.Articles) {
					return string(a.thread.Articles[i-1].ID)
				}
				return i
			}
			forceTop := a.threadForceTop
			if forceTop {
				a.threadAnchorID = ""
				state.ScrollTo(0, ui.Start)
			} else if first, last := state.Visible(); first >= 0 && first-1 < len(a.thread.Articles) && last >= first {
				// Remember the row at the top before a width or font change
				// reflows rows, then put that row back at the top. Row 0 is the
				// complete thread header and has no article ID.
				if first == 0 {
					a.threadAnchorID = ""
				} else {
					a.threadAnchorID = string(a.thread.Articles[first-1].ID)
				}
			}
			width, height := c.Size()
			viewport := viewportKey{Width: width, Height: height, Scale: a.fontScale}
			if !forceTop && a.threadViewport != viewport {
				if a.threadAnchorID == "" {
					state.ScrollTo(0, ui.Start)
				} else {
					for index, article := range a.thread.Articles {
						if string(article.ID) == a.threadAnchorID {
							state.ScrollTo(index+1, ui.Start)
							break
						}
					}
				}
			}
			a.threadViewport = viewport
			a.threadForceTop = false
			if a.threadScrollPending && a.route.Sub != "" {
				for index, article := range a.thread.Articles {
					if string(article.ID) == a.route.Sub {
						state.ScrollTo(index+1, ui.Start)
						a.threadScrollPending = false
						break
					}
				}
			}
			ui.List(c.Key(fmt.Sprintf("thread-%s-%s-%d", a.route.Name, a.route.ID, key.Number)), state, len(a.thread.Articles)+1, func(i int) {
				if i == 0 {
					a.threadHeader(c)
					return
				}
				a.postView(c, i-1, a.thread.Articles[i-1])
			}).FillWidth().Grow(1).Gap(12)
			a.pager(c, a.thread.Pagination, a.pageNumber)
			a.finishRefreshFrame()
		})
	})
}

func (a *App) finishRefreshFrame() {
	a.threadForceTop = false
	if a.threadRefreshClearSub {
		a.threadRefreshClearSub = false
		a.route.Sub = ""
		a.threadAnchorID = ""
		a.desktopListState(a.threadListKey()).ScrollTo(0, ui.Start)
	}
}

func (a *App) threadHeader(c *ui.Context) {
	canonicalBoard := a.threadBoard()
	board := strings.TrimSpace(a.thread.BoardDescription)
	if board == "" {
		board = canonicalBoard
	}
	if canonicalBoard != "" && canonicalBoard != board {
		board = board + " · " + canonicalBoard
	}
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if control(c, "版面："+board, "board", quiet).Clicked() {
				a.navigate(Route{Page: "board", Name: a.threadBoard()}, true)
			}
			if bool(a.thread.IsTop) {
				badge(c, "置顶", c.Theme().Warning)
			}
			if a.thread.ReplyCount > 0 {
				meta := []string{fmt.Sprintf("%d 回复", a.thread.ReplyCount)}
				if last := formatTime(a.thread.LastReplyTime); last != "" {
					meta = append(meta, "最后回复 "+last)
				}
				ui.Text(c, strings.Join(meta, " · ")).
					FontSize(a.textSize(c, 12)).FixedLineHeight(a.textSize(c, 18)).TextColor(c.Theme().TextMuted)
			}
		})
		title := a.threadTitle()
		if title != "" {
			ui.Text(c, title).FontSize(a.textSize(c, 26)).FontWeight(600).
				FixedLineHeight(a.textSize(c, 34)).FillWidth().Selectable()
		}
	})
}

func (a *App) threadTitle() string {
	if strings.TrimSpace(a.thread.Title) != "" {
		return a.thread.Title
	}
	if len(a.thread.Articles) > 0 {
		return a.thread.Articles[0].Title
	}
	return ""
}

func (a *App) threadToolbar(c *ui.Context) {
	collected := bool(a.thread.Collect)
	collectLabel := "收藏文章"
	if collected {
		collectLabel = "已收藏文章"
	}
	ui.Row(c).FillWidth().Wrap().Gap(8).Padding(8, 12).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).
		AlignItems(ui.Center).Children(func() {
		collectTone := quiet
		if collected {
			collectTone = selected
		}
		a.action(c, collectLabel, "star", collectTone, a.collect)
		a.action(c, "复制链接", "link", quiet, func() {
			link := a.Core.Forum.ThreadURL(a.threadBoard(), a.route.ID)
			a.run("复制链接", func(ctx context.Context) (func(), error) {
				if a.device == nil {
					return nil, nil
				}
				err := a.device.Copy(link)
				return func() { a.notice("帖子链接已复制") }, err
			})
		})
		if a.threadVoteAvailable {
			a.action(c, "查看投票", "poll", quiet, func() {
				a.navigate(Route{Page: "vote", ID: a.route.ID}, true)
			})
		}
		if a.threadBetAvailable {
			a.action(c, "查看竞猜", "bet", quiet, func() {
				a.navigate(Route{Page: "bet", ID: a.route.ID}, true)
			})
		}
	})
}

func (a *App) threadRoot() (forum.Article, bool) {
	if len(a.thread.Articles) > 0 {
		return a.thread.Articles[0], true
	}
	if a.thread.ID == "" {
		return forum.Article{}, false
	}
	return forum.Article{
		ID: a.thread.ID, IsLiked: a.thread.IsLiked, LikeSum: a.thread.LikeSum,
		IsVotedown: a.thread.IsVotedown, VotedownSum: a.thread.VotedownSum,
	}, true
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
	return a.route.Name
}

func (a *App) collect() {
	board, id := a.threadBoard(), a.route.ID
	was := bool(a.thread.Collect)
	a.run("更新文章收藏", func(ctx context.Context) (func(), error) {
		_, err := a.Core.SetCollected(ctx, board, id, !was)
		return func() {
			a.thread.Collect = forum.Flag(!was)
			if was {
				a.notice("已取消文章收藏")
			} else {
				a.notice("已加入文章收藏")
			}
		}, err
	})
}

func (a *App) postView(c *ui.Context, index int, v forum.Article) {
	manage := bool(v.IsAdmin) || strings.EqualFold(string(v.Author.ID), a.Core.Session().Username)
	model := a.threadModel(v)
	postState := a.expandedPostState(a.threadListKey())
	pinned := bool(v.IsTop)
	expanded := postState[string(v.ID)]
	collapsed := pinned && !expanded
	ui.Column(c.Key("post-"+string(v.ID))).FillWidth().Padding(16).Gap(10).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).Children(func() {
		ui.Row(c).FillWidth().Gap(11).AlignItems(ui.Start).Children(func() {
			a.avatar(c, v.Author, 38)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
					if v.Author.Anonymous() {
						ui.Text(c, v.Author.DisplayName()).Bold()
					} else {
						a.authorName(c, v.Author)
					}
					if a.isThreadAuthor(v) {
						badge(c, "楼主", c.Theme().Accent)
					}
					if bool(v.IsTop) {
						badge(c, "置顶", c.Theme().Warning)
					}
				})
				meta := []string{fmt.Sprintf("#%d", v.Position)}
				if t := formatTime(v.PostTime); t != "" {
					meta = append(meta, t)
				}
				if model.replyPosition > 0 {
					meta = append(meta, fmt.Sprintf("回复 #%d %s", model.replyPosition, model.replyAuthor))
				}
				ui.Text(c, strings.Join(meta, " · ")).FontSize(a.textSize(c, 12)).
					FixedLineHeight(a.textSize(c, 18)).TextColor(c.Theme().TextMuted).MaxLines(1).Ellipsis("…")
			})
		})
		if pinned {
			if collapsed {
				a.action(c, "展开置顶内容", "next", quiet, func() {
					postState[string(v.ID)] = true
					c.Invalidate()
				})
				return
			}
			a.action(c, "收起置顶内容", "down", selected, func() {
				postState[string(v.ID)] = false
				c.Invalidate()
			})
		}
		if bool(v.HiddenByVotedown) {
			ui.Row(c).Gap(6).Children(func() {
				badge(c, "此楼被踩折叠", c.Theme().Danger)
				ui.Text(c, "正文仍可阅读").FontSize(a.textSize(c, 12)).TextColor(c.Theme().TextMuted)
			})
		}
		a.bodyPartsView(c, model.parts)
		ui.Divider(c)
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			a.reactionDisabled(c, fmt.Sprintf("赞 %d", v.LikeSum), "like", bool(v.IsLiked), false, a.busy, func() {
				a.react(index, v, application.Like)
			})
			a.reactionDisabled(c, fmt.Sprintf("踩 %d", v.VotedownSum), "down", bool(v.IsVotedown), true, a.busy, func() {
				a.react(index, v, application.Downvote)
			})
			a.action(c, "回复此楼", "reply", quiet, func() {
				a.navigate(Route{Page: "compose", Name: a.threadBoard(), ID: string(v.ID), Sub: "reply"}, true)
			})
			a.action(c, "复制正文", "copy", quiet, func() { a.copyPost(v) })
			if manage {
				a.action(c, "编辑", "edit", quiet, func() {
					a.navigate(Route{Page: "compose", Name: a.threadBoard(), ID: string(v.ID), Sub: "edit"}, true)
				})
				a.action(c, "删除", "trash", destructive, func() {
					board, id := a.threadBoard(), string(v.ID)
					a.confirm = &confirmation{text: "确定删除这篇帖子？此操作无法撤销。", run: func() {
						a.run("删除帖子", func(ctx context.Context) (func(), error) {
							_, err := a.Core.Forum.DeleteArticle(ctx, board, id)
							return func() { a.loadThread() }, err
						})
					}}
				})
			}
		})
	})
}

func (a *App) authorName(c *ui.Context, u forum.User) {
	e := ui.ButtonBase(c).Label("查看用户 " + u.DisplayName()).Padding(0).Radius(6).Disabled(a.busy)
	if e.Hovered() {
		e.TextColor(c.Theme().Accent)
	} else {
		e.TextColor(c.Theme().Text)
	}
	e.Children(func() { ui.Text(c, u.DisplayName()).Bold() })
	if e.Clicked() {
		a.navigate(Route{Page: "user", Name: string(u.ID)}, true)
	}
}

func (a *App) isThreadAuthor(v forum.Article) bool {
	root := a.thread.Author
	if root.ID == "" && len(a.thread.Articles) > 0 {
		root = a.thread.Articles[0].Author
	}
	return root.ID != "" && v.Author.ID == root.ID
}

func (a *App) copyPost(v forum.Article) {
	text := strings.TrimSpace(a.threadModel(v).plain)
	if text == "" {
		text = "(无正文)"
	}
	if v.Author.DisplayName() != "" {
		text = v.Author.DisplayName() + "：\n" + text
	}
	a.run("复制正文", func(ctx context.Context) (func(), error) {
		if a.device == nil {
			return nil, nil
		}
		err := a.device.Copy(text)
		return func() { a.notice("楼层正文已复制") }, err
	})
}

func (a *App) reactThread(v forum.Article, kind application.ReactionKind) {
	if kind == application.Like && bool(v.IsLiked) {
		a.notice("已赞过，服务器不支持取消赞")
		return
	}
	board := a.threadBoard()
	a.run("提交评价", func(ctx context.Context) (func(), error) {
		updated, err := a.Core.React(ctx, board, v, kind)
		return func() {
			a.thread.LikeSum = updated.LikeSum
			a.thread.IsLiked = updated.IsLiked
			a.thread.VotedownSum = updated.VotedownSum
			a.thread.IsVotedown = updated.IsVotedown
			if len(a.thread.Articles) > 0 {
				if string(a.thread.Articles[0].ID) == string(updated.ID) {
					a.thread.Articles[0] = updated
				}
			}
		}, err
	})
}

func (a *App) react(index int, v forum.Article, kind application.ReactionKind) {
	if kind == application.Like && bool(v.IsLiked) {
		a.notice("已赞过，服务器不支持取消赞")
		return
	}
	if kind == application.Downvote && bool(v.IsVotedown) {
		a.notice("已踩过，服务器不支持取消踩")
		return
	}
	board := a.threadBoard()
	a.run("提交评价", func(ctx context.Context) (func(), error) {
		updated, err := a.Core.React(ctx, board, v, kind)
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
