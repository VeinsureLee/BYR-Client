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
		r, err = a.Forum.Like(ctx, board, string(article.ID))
	} else {
		r, err = a.Forum.VoteDown(ctx, board, string(article.ID))
	}
	duplicate := false
	if err != nil {
		var api *forum.APIError
		if !errors.As(err, &api) || (kind == Like && api.Code != "1603") || (kind == Downvote && api.Code != "1608") {
			return article, err
		}
		duplicate = true
	}
	wasLiked, wasDown := bool(article.IsLiked), bool(article.IsVotedown)
	article.IsLiked = forum.Flag(kind == Like)
	article.IsVotedown = forum.Flag(kind == Downvote)
	if r.LikeSum != nil {
		article.LikeSum = *r.LikeSum
	} else if !duplicate {
		if kind == Like {
			article.LikeSum++
		} else if wasLiked && article.LikeSum > 0 {
			article.LikeSum--
		}
	}
	if r.VotedownSum != nil {
		article.VotedownSum = *r.VotedownSum
	} else if !duplicate {
		if kind == Downvote {
			article.VotedownSum++
		} else if wasDown && article.VotedownSum > 0 {
			article.VotedownSum--
		}
	}
	return article, nil
}

type collectionKey struct{ account, board, id string }

func collectionIdentity(account, board, id string) collectionKey {
	return collectionKey{strings.ToLower(strings.TrimSpace(account)), strings.ToLower(board), id}
}

// CollectionState retains successful changes when a thread response omits its
// collection flag. It is account scoped, in memory only, and never caches errors.
func (a *App) CollectionState(board, id string, fallback bool) bool {
	account := a.Session().Username
	a.mu.RLock()
	defer a.mu.RUnlock()
	if value, known := a.collections[collectionIdentity(account, board, id)]; known {
		return value
	}
	return fallback
}

// RememberCollection is called for entries actually read from a collection
// page; opening one therefore immediately shows the correct selected star.
func (a *App) RememberCollection(board, id string) {
	a.rememberCollection(a.Session().Username, board, id, true)
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
	account := a.Session().Username
	var err error
	if collected {
		_, err = a.Forum.AddCollection(ctx, board, id)
	} else {
		_, err = a.Forum.RemoveCollection(ctx, board, id)
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
