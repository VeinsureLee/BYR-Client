package application

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func (a *App) Self(ctx context.Context) (forum.User, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.User, error) {
		return a.forum.Self(requestCtx)
	})
}

func (a *App) LoadBlacklist(ctx context.Context, page int) (forum.Blacklist, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Blacklist, error) {
		return a.forum.Blacklist(requestCtx, page)
	})
}

func (a *App) AddBlacklist(ctx context.Context, username string) (forum.Status, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return forum.Status{}, errors.New("请输入用户名")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Status, error) {
		return a.forum.AddBlacklist(requestCtx, username)
	})
}

func (a *App) RemoveBlacklist(ctx context.Context, username string) (forum.Status, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return forum.Status{}, errors.New("用户名不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Status, error) {
		return a.forum.RemoveBlacklist(requestCtx, username)
	})
}

func (a *App) LoadVote(ctx context.Context, id string) (forum.Vote, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return forum.Vote{}, errors.New("投票标识不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Vote, error) {
		return a.forum.Vote(requestCtx, id)
	})
}

func (a *App) SubmitVote(ctx context.Context, id, choice string) (forum.Vote, error) {
	id, choice = strings.TrimSpace(id), strings.TrimSpace(choice)
	if id == "" || choice == "" {
		return forum.Vote{}, errors.New("请选择投票选项")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Vote, error) {
		return a.forum.SubmitVote(requestCtx, id, choice)
	})
}

func (a *App) LoadBet(ctx context.Context, id string) (forum.Bet, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return forum.Bet{}, errors.New("竞猜标识不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Bet, error) {
		return a.forum.Bet(requestCtx, id)
	})
}

func (a *App) SubmitBet(ctx context.Context, id, item string, score int) (forum.Bet, error) {
	id, item = strings.TrimSpace(id), strings.TrimSpace(item)
	if id == "" || item == "" || score <= 0 {
		return forum.Bet{}, errors.New("请选择竞猜选项并填写有效积分")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Bet, error) {
		return a.forum.SubmitBet(requestCtx, id, item, score)
	})
}

func (a *App) ReadArticle(ctx context.Context, board, id string) (forum.Article, error) {
	board, id = strings.TrimSpace(board), strings.TrimSpace(id)
	if board == "" || id == "" {
		return forum.Article{}, errors.New("版面和主题标识不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Article, error) {
		return a.forum.Article(requestCtx, board, id)
	})
}

func (a *App) LoadAttachments(ctx context.Context, board, id string) (forum.Attachment, error) {
	board, id = strings.TrimSpace(board), strings.TrimSpace(id)
	if board == "" || id == "" {
		return forum.Attachment{}, errors.New("版面和主题标识不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Attachment, error) {
		return a.forum.Attachments(requestCtx, board, id)
	})
}

func (a *App) DeleteArticle(ctx context.Context, board, id string) (forum.Article, error) {
	board, id = strings.TrimSpace(board), strings.TrimSpace(id)
	if board == "" || id == "" {
		return forum.Article{}, errors.New("版面和主题标识不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Article, error) {
		return a.forum.DeleteArticle(requestCtx, board, id)
	})
}

func (a *App) UploadAttachment(ctx context.Context, board, id, name string, source io.Reader) (forum.Attachment, error) {
	board, name = strings.TrimSpace(board), strings.TrimSpace(name)
	id = strings.TrimSpace(id)
	if board == "" || name == "" || source == nil {
		return forum.Attachment{}, errors.New("上传附件信息不完整")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Attachment, error) {
		return a.forum.Upload(requestCtx, board, id, name, source)
	})
}

func (a *App) DeleteAttachment(ctx context.Context, board, id string) (forum.Attachment, error) {
	board, id = strings.TrimSpace(board), strings.TrimSpace(id)
	if board == "" || id == "" {
		return forum.Attachment{}, errors.New("版面和附件标识不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Attachment, error) {
		return a.forum.DeleteAttachment(requestCtx, board, id)
	})
}

func (a *App) Version(ctx context.Context, beta bool) (forum.Versions, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Versions, error) {
		return a.forum.Version(requestCtx, beta)
	})
}

// CheckWebLogin is the explicit, diagnostic web-login flow. It uses the supplied
// test credentials and deliberately does not read or alter the active OAuth session.
func (a *App) CheckWebLogin(ctx context.Context, username, password string) (forum.WebLogin, error) {
	return a.forum.CheckWebLogin(ctx, username, password)
}

func (a *App) ThreadURL(board, id string) string { return a.forum.ThreadURL(board, id) }
