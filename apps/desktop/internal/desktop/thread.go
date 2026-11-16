package desktop

import (
	"context"
	"fmt"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
	"github.com/egoist/mygo/ui"
)

func (a *App) loadThread() {
	r := a.route
	a.thread = forum.Thread{}
	account := a.Core.Session().Username
	a.loadData("加载主题", func(ctx context.Context) (func(), error) {
		v, err := a.Core.Forum.Thread(ctx, r.Name, r.ID, forum.Page{Number: r.Number})
		if err != nil {
			return nil, err
		}
		historyErr := a.Core.Local.Visit(ctx, account, local.History{Board: r.Name, ID: r.ID, Title: v.Title})
		return func() {
			if bool(v.Collect) {
				a.Core.RememberCollection(r.Name, r.ID)
			}
			v.Collect = forum.Flag(a.Core.CollectionState(r.Name, r.ID, bool(v.Collect)))
			a.thread = v
			if v.Pagination.CurrentPage > 0 {
				a.route.Number = int(v.Pagination.CurrentPage)
			}
			if historyErr != nil {
				a.fail(historyErr)
			}
		}, nil
	})
}

func (a *App) threadView(c *ui.Context) {
	a.threadHeader(c)
	a.threadToolbar(c)
	if len(a.thread.Articles) == 0 {
		emptyState(c, "message", "暂无帖子内容", "这个主题可能正在加载，或已被删除。")
		return
	}
	ui.List(c.Key(fmt.Sprintf("thread-%s-%s-%d", a.route.Name, a.route.ID, a.route.Number)), nil, len(a.thread.Articles), func(i int) {
		a.postView(c, i, a.thread.Articles[i])
	}).FillWidth().Grow(1).Gap(12)
	a.pager(c, a.thread.Pagination, a.pageNumber)
}

func (a *App) threadHeader(c *ui.Context) {
	board := strings.TrimSpace(a.thread.BoardDescription)
	if board == "" {
		board = a.route.Name
	}
	if a.thread.Board != "" && a.thread.Board != a.route.Name {
		board = board + " · " + a.thread.Board
	}
	ui.Column(c).FillWidth().Gap(7).Children(func() {
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if control(c, "版面："+board, "board", quiet).Clicked() {
				a.navigate(Route{Page: "board", Name: a.route.Name}, true)
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
					FontSize(12).TextColor(c.Theme().TextMuted)
			}
		})
		title := a.threadTitle()
		if title != "" {
			ui.Text(c, title).FontSize(22).Bold().MaxLines(3)
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
	root, hasRoot := a.threadRoot()
	collected := bool(a.thread.Collect)
	collectLabel := "收藏文章"
	if collected {
		collectLabel = "已收藏文章"
	}
	ui.Row(c).FillWidth().Wrap().Gap(6).Padding(9, 10).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(12).
		AlignItems(ui.Center).Children(func() {
		a.action(c, "回复主题", "reply", filled, func() {
			a.navigate(Route{Page: "compose", Name: a.route.Name, ID: a.route.ID, Sub: "reply"}, true)
		})
		if hasRoot {
			likeLabel := "点赞"
			if bool(root.IsLiked) {
				likeLabel = "已点赞"
			}
			if root.LikeSum > 0 {
				likeLabel = fmt.Sprintf("%s %d", likeLabel, root.LikeSum)
			}
			a.reactionDisabled(c, likeLabel, "like", bool(root.IsLiked), false, a.busy, func() {
				a.reactThread(root, application.Like)
			})
		}
		collectTone := quiet
		if collected {
			collectTone = selected
		}
		a.action(c, collectLabel, "star", collectTone, a.collect)
		a.action(c, "复制链接", "link", quiet, func() {
			link := a.Core.Forum.ThreadURL(a.route.Name, a.route.ID)
			a.run("复制链接", func(ctx context.Context) (func(), error) {
				if a.device == nil {
					return nil, nil
				}
				err := a.device.Copy(link)
				return func() { a.notice("帖子链接已复制") }, err
			})
		})
		a.action(c, "进入版面", "board", quiet, func() {
			a.navigate(Route{Page: "board", Name: a.route.Name}, true)
		})
		a.action(c, "查看投票", "poll", quiet, func() {
			a.navigate(Route{Page: "vote", ID: a.route.ID}, true)
		})
		a.action(c, "查看竞猜", "bet", quiet, func() {
			a.navigate(Route{Page: "bet", ID: a.route.ID}, true)
		})
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

func (a *App) collect() {
	board, id := a.route.Name, a.route.ID
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
	targetPosition, targetAuthor := a.replyTarget(v)
	ui.Column(c.Key("post-"+string(v.ID))).FillWidth().Padding(16).Gap(10).
		Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(14).Children(func() {
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
				if targetPosition > 0 {
					meta = append(meta, fmt.Sprintf("回复 #%d %s", targetPosition, targetAuthor))
				}
				ui.Text(c, strings.Join(meta, " · ")).FontSize(12).
					TextColor(c.Theme().TextMuted).MaxLines(1).Ellipsis("…")
			})
		})
		if bool(v.HiddenByVotedown) {
			ui.Row(c).Gap(6).Children(func() {
				badge(c, "此楼被踩折叠", c.Theme().Danger)
				ui.Text(c, "正文仍可阅读").FontSize(12).TextColor(c.Theme().TextMuted)
			})
		}
		a.bodyView(c, v.Content, v.Attachments)
		ui.Divider(c)
		ui.Row(c).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
			a.reactionDisabled(c, fmt.Sprintf("赞 %d", v.LikeSum), "like", bool(v.IsLiked), false, a.busy, func() {
				a.react(index, v, application.Like)
			})
			a.reactionDisabled(c, fmt.Sprintf("踩 %d", v.VotedownSum), "down", bool(v.IsVotedown), true, a.busy, func() {
				a.react(index, v, application.Downvote)
			})
			a.action(c, "回复此楼", "reply", quiet, func() {
				a.navigate(Route{Page: "compose", Name: a.route.Name, ID: string(v.ID), Sub: "reply"}, true)
			})
			a.action(c, "复制正文", "copy", quiet, func() { a.copyPost(v) })
			if manage {
				a.action(c, "编辑", "edit", quiet, func() {
					a.navigate(Route{Page: "compose", Name: a.route.Name, ID: string(v.ID), Sub: "edit"}, true)
				})
				a.action(c, "删除", "trash", destructive, func() {
					board, id := a.route.Name, string(v.ID)
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

func (a *App) replyTarget(v forum.Article) (int, string) {
	id := string(v.ReplyID)
	if id == "" || id == "0" {
		return 0, ""
	}
	for _, p := range a.thread.Articles {
		if string(p.ID) == id {
			return int(p.Position), p.Author.DisplayName()
		}
	}
	return 0, ""
}

func (a *App) copyPost(v forum.Article) {
	text := strings.TrimSpace(content.Plain(v.Content))
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
	board := a.route.Name
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
	board := a.route.Name
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
