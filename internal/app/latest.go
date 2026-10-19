package app

import (
	"context"
	"fmt"
	"sync"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
)

const latestBoardsPerNode = 3

type latestCandidate struct {
	article domain.Article
	board   domain.BoardID
	err     error
}

// LoadLatestTopic reads the first page of up to three boards below a directory
// node, keeps the first non-pinned topic of each and returns the newest one.
// It is a lightweight overview signal, not a complete forum-wide scan.
func (a *App) LoadLatestTopic(ctx context.Context, node forum.NodeID) error {
	if err := a.requireSession(); err != nil {
		return err
	}
	a.mu.Lock()
	if a.latestLoad[node] {
		a.mu.Unlock()
		return nil
	}
	a.latestLoad[node] = true
	a.latestErrors[node] = ""
	nav := a.nav
	a.mu.Unlock()
	a.changed()

	if nav == nil {
		return a.finishLatest(node, domain.Article{}, "", fmt.Errorf("目录尚未初始化"))
	}
	boards, err := nav.Boards(node, forum.BFS)
	if err != nil || len(boards) == 0 {
		return a.finishLatest(node, domain.Article{}, "", fmt.Errorf("该节点没有可读取的版面"))
	}
	if len(boards) > latestBoardsPerNode {
		boards = boards[:latestBoardsPerNode]
	}
	results := make(chan latestCandidate, len(boards))
	var wg sync.WaitGroup
	for _, board := range boards {
		wg.Add(1)
		go func(board domain.BoardID) {
			defer wg.Done()
			select {
			case a.latestSem <- struct{}{}:
			case <-ctx.Done():
				results <- latestCandidate{board: board, err: ctx.Err()}
				return
			}
			defer func() { <-a.latestSem }()
			page, err := a.source.ReadBoardPage(ctx, domain.Scope{SiteID: "byr"}, board, "", domain.DefaultBoardBatchSize)
			if err != nil {
				a.markAuthLost(err)
				results <- latestCandidate{board: board, err: err}
				return
			}
			for _, article := range page.Articles {
				if !article.Pinned {
					a.store.UpsertArticle(article)
					results <- latestCandidate{article: article, board: board}
					return
				}
			}
			results <- latestCandidate{board: board, err: fmt.Errorf("版面 %s 没有非置顶主题", board)}
		}(board)
	}
	wg.Wait()
	close(results)

	var best domain.Article
	var bestBoard domain.BoardID
	lastErr := error(nil)
	for item := range results {
		if item.err != nil {
			lastErr = item.err
			continue
		}
		at := item.article.LastReplyAt
		if at.IsZero() {
			at = item.article.CreatedAt
		}
		bestAt := best.LastReplyAt
		if bestAt.IsZero() {
			bestAt = best.CreatedAt
		}
		if best.Key.ArticleID == "" || at.After(bestAt) {
			best = item.article
			bestBoard = item.board
		}
	}
	if best.Key.ArticleID == "" {
		if lastErr == nil {
			lastErr = fmt.Errorf("没有读到非置顶主题")
		}
		return a.finishLatest(node, domain.Article{}, "", lastErr)
	}
	return a.finishLatest(node, best, bestBoard, nil)
}

func (a *App) finishLatest(node forum.NodeID, article domain.Article, board domain.BoardID, err error) error {
	a.mu.Lock()
	a.latestLoad[node] = false
	if err == nil {
		a.latest[node] = article
		a.latestBoard[node] = board
		a.latestErrors[node] = ""
	} else {
		a.latestErrors[node] = err.Error()
	}
	a.mu.Unlock()
	a.changed()
	return err
}

func (a *App) LatestTopic(node forum.NodeID) (domain.Article, domain.BoardID, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	article, ok := a.latest[node]
	if !ok {
		return domain.Article{}, "", false
	}
	return article, a.latestBoard[node], true
}

func (a *App) LatestLoading(node forum.NodeID) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.latestLoad[node]
}

func (a *App) LatestError(node forum.NodeID) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.latestErrors[node]
}
