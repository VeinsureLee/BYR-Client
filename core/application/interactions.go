package application

import (
	"context"
	"errors"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

type ReactionKind int

const (
	Like ReactionKind = iota
	Downvote
)

// The reaction endpoint records the reaction before resolving the article
// author. Anonymous articles expose a display-only pseudo ID (IWhisper#N),
// so the response can be 0500 "未知的论坛ID" even though the reaction was
// stored. The client treats that reply as accepted for anonymous authors; a
// later thread load confirms the authoritative state.
var ErrReactionUnavailable = errors.New("该帖子暂不支持评价（论坛接口限制）")

// React is the shared interaction facade. BYR permits switching a reaction,
// but has no cancellation endpoint. Failed requests leave the original intact.
func (a *App) React(ctx context.Context, board string, article forum.Article, kind ReactionKind) (forum.Article, error) {
	if kind != Like && kind != Downvote {
		return article, errors.New("unknown reaction")
	}
	if kind == Like && bool(article.IsLiked) || kind == Downvote && bool(article.IsVotedown) {
		return article, nil
	}
	var r forum.Reaction
	var err error
	if kind == Like {
		r, err = a.forum.Like(ctx, board, string(article.ID))
	} else {
		r, err = a.forum.VoteDown(ctx, board, string(article.ID))
	}
	duplicate := false
	if err != nil {
		var api *forum.APIError
		if !errors.As(err, &api) {
			return article, err
		}
		switch {
		case api.Code == "0500" && article.Author.Anonymous():
			// The server stored the reaction but could not resolve the
			// anonymous author's account. Continue as if the response omitted
			// the fresh totals.
		case kind == Like && api.Code == "1603", kind == Downvote && api.Code == "1608":
			duplicate = true
		case api.Code == "0500":
			return article, ErrReactionUnavailable
		default:
			return article, err
		}
	}
	wasLiked, wasDown := bool(article.IsLiked), bool(article.IsVotedown)
	like, down := int(article.LikeSum), int(article.VotedownSum)
	if duplicate {
		// The server confirmed the reaction already exists. Never increment a
		// local count that the response did not include.
		if r.LikeSum != nil {
			like = int(*r.LikeSum)
		}
		if r.VotedownSum != nil {
			down = int(*r.VotedownSum)
		}
	} else {
		// Some deployments acknowledge the reaction without returning the new
		// totals. Increment locally so the button reflects the action at once;
		// a larger server count still wins if another user reacted meanwhile.
		if kind == Like {
			if !wasLiked {
				like++
			}
			if wasDown && down > 0 {
				down--
			}
			if wasDown && r.VotedownSum != nil {
				down = int(*r.VotedownSum)
			}
		} else {
			if !wasDown {
				down++
			}
			if wasLiked && like > 0 {
				like--
			}
			if wasLiked && r.LikeSum != nil {
				like = int(*r.LikeSum)
			}
		}
		if r.LikeSum != nil && int(*r.LikeSum) > like {
			like = int(*r.LikeSum)
		}
		if r.VotedownSum != nil && int(*r.VotedownSum) > down {
			down = int(*r.VotedownSum)
		}
	}
	article.LikeSum = forum.Number(like)
	article.VotedownSum = forum.Number(down)
	article.IsLiked = forum.Flag(kind == Like)
	article.IsVotedown = forum.Flag(kind == Downvote)
	return article, nil
}

type collectionKey struct{ account, board, id string }

func collectionIdentity(account, board, id string) collectionKey {
	return collectionKey{strings.ToLower(strings.TrimSpace(account)), strings.ToLower(board), id}
}

// CollectionState retains successful changes when a thread response omits its
// collection flag. It is account scoped, in memory only, and never caches errors.
func (a *App) CollectionState(board, id string, fallback bool) bool {
	return a.collectionState(a.SessionInfo().Username, board, id, fallback)
}

// RememberCollection is called for entries actually read from a collection
// page; opening one therefore immediately shows the correct selected star.
func (a *App) RememberCollection(board, id string) {
	a.rememberCollection(a.sessionSnapshot().Username, board, id, true)
}

func (a *App) rememberCollection(account, board, id string, collected bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.collections == nil {
		a.collections = make(map[collectionKey]bool)
	}
	a.collections[collectionIdentity(account, board, id)] = collected
}

func (a *App) SetCollected(ctx context.Context, board, id string, collected bool) (bool, error) {
	account := a.sessionSnapshot().Username
	var err error
	if collected {
		_, err = a.forum.AddCollection(ctx, board, id)
	} else {
		_, err = a.forum.RemoveCollection(ctx, board, id)
	}
	var api *forum.APIError
	if collected && errors.As(err, &api) && api.Code == "0318" {
		err = nil // already collected is a confirmed state, not another add
	}
	if err != nil {
		return !collected, err
	}
	a.rememberCollection(account, board, id, collected)
	return collected, nil
}
