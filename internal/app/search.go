package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
)

type SearchMode string

const (
	ModeKeyword SearchMode = "keyword"
	ModeAI      SearchMode = "ai"
)

type RunStatus string

const (
	StatusQueued    RunStatus = "queued"
	StatusRunning   RunStatus = "running"
	StatusCompleted RunStatus = "completed"
	StatusStopped   RunStatus = "stopped"
	StatusFailed    RunStatus = "failed"
	StatusPartial   RunStatus = "partial"
)

type StopReason string

const (
	StopSourceEnd   StopReason = "source_end"
	StopResultLimit StopReason = "result_limit"
	StopPerBoard    StopReason = "per_board_limit"
	StopUserCancel  StopReason = "user_cancel"
	StopError       StopReason = "error"
)

type ArticleResult struct {
	Article      domain.Article
	Replies      []domain.Reply
	RepliesEnd   bool
	ThreadPages  int
	StoreCreated int
	StoreUpdated int
}

type BoardResult struct {
	BoardID  domain.BoardID
	Name     string
	Path     []forum.Node
	Articles []ArticleResult
	Accepted int
	End      bool
	Error    string
}

type SearchRun struct {
	ID              string
	Mode            SearchMode
	Query           domain.SearchQuery
	Status          RunStatus
	StopReason      StopReason
	Error           string
	Boards          []*BoardResult
	ArticleCount    int
	ReplyCount      int
	NewArticles     int
	UpdatedArticles int
	NewReplies      int
	StartedAt       time.Time
	FinishedAt      time.Time
	cancel          context.CancelFunc
	done            chan struct{}
}

type ArticleSnapshot struct {
	Article      domain.Article
	Replies      []domain.Reply
	RepliesEnd   bool
	ThreadPages  int
	StoreCreated int
	StoreUpdated int
}

type BoardSnapshotResult struct {
	BoardID  domain.BoardID
	Name     string
	Path     []forum.Node
	Articles []ArticleSnapshot
	Accepted int
	End      bool
	Error    string
}

type RunSnapshot struct {
	ID              string
	Mode            SearchMode
	Query           domain.SearchQuery
	Status          RunStatus
	StopReason      StopReason
	Error           string
	Boards          []BoardSnapshotResult
	ArticleCount    int
	ReplyCount      int
	NewArticles     int
	UpdatedArticles int
	NewReplies      int
	StartedAt       time.Time
	FinishedAt      time.Time
}

func (a *App) StartSearch(q domain.SearchQuery) (string, error) {
	return a.startSearch(q, true)
}

func (a *App) startSearch(q domain.SearchQuery, recordHistory bool) (string, error) {
	if err := a.requireSession(); err != nil {
		return "", err
	}
	q = q.Normalized()
	if q.Empty() {
		return "", errors.New("至少填写一个关键词或作者 ID")
	}
	nav := a.Navigation()
	if nav == nil {
		return "", ErrNoCatalog
	}
	boards, err := nav.Boards(q.ScopeNode, forum.BFS)
	if err != nil {
		return "", fmt.Errorf("解析检索范围: %w", err)
	}
	if len(boards) == 0 {
		return "", errors.New("检索范围内没有版面")
	}
	now := a.clock.Now()
	q.Time = resolveTimeRange(q.Time, now)

	a.mu.Lock()
	a.runSeq++
	id := fmt.Sprintf("run-%d-%d", now.UnixNano(), a.runSeq)
	ctx, cancel := context.WithCancel(a.rootCtx)
	run := &SearchRun{
		ID:        id,
		Mode:      ModeKeyword,
		Query:     q,
		Status:    StatusQueued,
		StartedAt: now,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
	a.runs[id] = run
	if recordHistory {
		a.history = append(a.history, &HistoryEntry{
			ID:        id,
			Mode:      ModeKeyword,
			Title:     searchTitle(q),
			Query:     q,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	a.mu.Unlock()
	a.changed()

	go a.executeSearch(ctx, id, q, boards)
	return id, nil
}

// SearchSync runs one bounded keyword search for an AI tool call and waits for
// its committed result. It does not create a keyword history entry.
func (a *App) SearchSync(ctx context.Context, q domain.SearchQuery) (RunSnapshot, error) {
	id, err := a.startSearch(q, false)
	if err != nil {
		return RunSnapshot{}, err
	}
	a.mu.RLock()
	run := a.runs[id]
	var done chan struct{}
	if run != nil {
		done = run.done
	}
	a.mu.RUnlock()
	if done == nil {
		return RunSnapshot{}, fmt.Errorf("搜索任务未创建")
	}
	select {
	case <-done:
	case <-ctx.Done():
		a.StopSearch(id)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		return RunSnapshot{}, ctx.Err()
	}
	a.mu.Lock()
	snapshot := RunSnapshot{}
	if run := a.runs[id]; run != nil {
		snapshot = snapshotRun(run)
		delete(a.runs, id)
	}
	a.mu.Unlock()
	return snapshot, nil
}

func (a *App) StopSearch(id string) {
	a.mu.Lock()
	run := a.runs[id]
	var cancel context.CancelFunc
	if run != nil {
		cancel = run.cancel
	}
	a.mu.Unlock()
	if cancel != nil {
		cancel()
		a.changed()
	}
}

func (a *App) Run(id string) (RunSnapshot, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	run := a.runs[id]
	if run == nil {
		return RunSnapshot{}, false
	}
	return snapshotRun(run), true
}

func snapshotRun(run *SearchRun) RunSnapshot {
	snapshot := RunSnapshot{
		ID:              run.ID,
		Mode:            run.Mode,
		Query:           run.Query,
		Status:          run.Status,
		StopReason:      run.StopReason,
		Error:           run.Error,
		ArticleCount:    run.ArticleCount,
		ReplyCount:      run.ReplyCount,
		NewArticles:     run.NewArticles,
		UpdatedArticles: run.UpdatedArticles,
		NewReplies:      run.NewReplies,
		StartedAt:       run.StartedAt,
		FinishedAt:      run.FinishedAt,
	}
	snapshot.Query.Keywords = append([]string(nil), run.Query.Keywords...)
	snapshot.Boards = make([]BoardSnapshotResult, 0, len(run.Boards))
	for _, board := range run.Boards {
		item := BoardSnapshotResult{
			BoardID:  board.BoardID,
			Name:     board.Name,
			Path:     append([]forum.Node(nil), board.Path...),
			Accepted: board.Accepted,
			End:      board.End,
			Error:    board.Error,
		}
		item.Articles = make([]ArticleSnapshot, 0, len(board.Articles))
		for _, article := range board.Articles {
			item.Articles = append(item.Articles, ArticleSnapshot{
				Article:      article.Article,
				Replies:      append([]domain.Reply(nil), article.Replies...),
				RepliesEnd:   article.RepliesEnd,
				ThreadPages:  article.ThreadPages,
				StoreCreated: article.StoreCreated,
				StoreUpdated: article.StoreUpdated,
			})
		}
		snapshot.Boards = append(snapshot.Boards, item)
	}
	return snapshot
}

func (a *App) executeSearch(ctx context.Context, id string, q domain.SearchQuery, boards []domain.BoardID) {
	a.mu.Lock()
	if run := a.runs[id]; run != nil {
		run.Status = StatusRunning
	}
	a.mu.Unlock()
	a.changed()

	results := make([]*BoardResult, len(boards))
	for i, board := range boards {
		path := a.BoardPath(board)
		name := a.BoardName(board)
		results[i] = &BoardResult{BoardID: board, Name: name, Path: path}
	}
	a.mu.Lock()
	if run := a.runs[id]; run != nil {
		run.Boards = results
	}
	a.mu.Unlock()

	budget := &searchBudget{limit: q.Total}
	var anyError atomic.Bool
	var limitHit atomic.Bool
	var perBoardHit atomic.Int32
	var errOnce sync.Once
	var firstError error

	sem := make(chan struct{}, q.Concurrency)
	var wg sync.WaitGroup
	for i, board := range boards {
		wg.Add(1)
		go func(index int, board domain.BoardID) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			a.searchOneBoard(ctx, id, q, board, results[index], budget, &anyError, &limitHit, &perBoardHit, &errOnce, &firstError)
		}(i, board)
	}
	wg.Wait()

	now := a.clock.Now()
	a.mu.Lock()
	run := a.runs[id]
	if run != nil {
		run.FinishedAt = now
		if firstError != nil {
			run.Error = firstError.Error()
		}
		switch {
		case ctx.Err() != nil:
			run.Status = StatusStopped
			run.StopReason = StopUserCancel
		case limitHit.Load():
			run.Status = StatusCompleted
			run.StopReason = StopResultLimit
		case perBoardHit.Load() == int32(len(boards)):
			run.Status = StatusCompleted
			run.StopReason = StopPerBoard
		case anyError.Load():
			if run.ArticleCount == 0 {
				run.Status = StatusFailed
			} else {
				run.Status = StatusPartial
			}
			run.StopReason = StopError
		default:
			run.Status = StatusCompleted
			run.StopReason = StopSourceEnd
		}
	}
	var done chan struct{}
	if run != nil {
		done = run.done
	}
	a.mu.Unlock()
	if done != nil {
		close(done)
	}
	a.changed()
}

type searchBudget struct {
	mu    sync.Mutex
	limit int
	count int
	hit   bool
}

func (b *searchBudget) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count >= b.limit {
		b.hit = true
		return false
	}
	b.count++
	return true
}

func (a *App) searchOneBoard(
	ctx context.Context,
	id string,
	q domain.SearchQuery,
	board domain.BoardID,
	result *BoardResult,
	budget *searchBudget,
	anyError *atomic.Bool,
	limitHit *atomic.Bool,
	perBoardHit *atomic.Int32,
	errOnce *sync.Once,
	firstError *error,
) {
	seen := make(map[domain.ArticleID]bool)
	accepted := 0
	cursor := domain.Cursor("")
	query := domain.BoardQuery{
		Keywords: q.Keywords,
		Author:   q.Author,
		From:     q.Time.From,
		To:       q.Time.To,
	}
	for {
		if ctx.Err() != nil {
			return
		}
		page, err := a.source.SearchBoardPage(ctx, domain.Scope{SiteID: "byr"}, board, query, cursor, domain.DefaultSearchCandidates)
		if err != nil {
			a.markAuthLost(err)
			if ctx.Err() == nil {
				a.setBoardError(id, result, err)
				anyError.Store(true)
				errOnce.Do(func() { *firstError = err })
			}
			return
		}
		stopBoard := false
		for _, candidate := range page.Articles {
			if seen[candidate.Key.ArticleID] {
				continue
			}
			seen[candidate.Key.ArticleID] = true
			if !matchesQuery(candidate, q) {
				continue
			}
			if accepted >= q.PerBoard {
				perBoardHit.Add(1)
				stopBoard = true
				break
			}
			if !budget.take() {
				limitHit.Store(true)
				stopBoard = true
				break
			}

			body, replies, repliesEnd, pages, threadErr := a.fetchThread(ctx, q, candidate)
			if threadErr != nil && ctx.Err() == nil {
				a.setBoardError(id, result, threadErr)
				anyError.Store(true)
				errOnce.Do(func() { *firstError = threadErr })
			}
			created, updated := a.store.UpsertArticle(body)
			newReplies := 0
			for _, reply := range replies {
				if inserted, _ := a.store.UpsertReply(reply); inserted {
					newReplies++
				}
			}
			accepted++

			a.mu.Lock()
			run := a.runs[id]
			if run != nil {
				entry := ArticleResult{
					Article:      body,
					Replies:      replies,
					RepliesEnd:   repliesEnd,
					ThreadPages:  pages,
					StoreCreated: boolInt(created),
					StoreUpdated: boolInt(updated),
				}
				result.Articles = append(result.Articles, entry)
				result.Accepted = accepted
				run.ArticleCount++
				run.ReplyCount += len(replies)
				if created {
					run.NewArticles++
				}
				if updated {
					run.UpdatedArticles++
				}
				run.NewReplies += newReplies
			}
			a.mu.Unlock()
			a.changed()

			if accepted >= q.PerBoard {
				perBoardHit.Add(1)
				stopBoard = true
				break
			}
		}
		a.mu.Lock()
		result.Accepted = accepted
		a.mu.Unlock()
		if stopBoard {
			return
		}
		if page.End || page.Next == "" {
			a.mu.Lock()
			result.End = true
			a.mu.Unlock()
			a.changed()
			return
		}
		if page.Next == cursor {
			err := fmt.Errorf("board %s repeated cursor %q", board, page.Next)
			a.setBoardError(id, result, err)
			anyError.Store(true)
			errOnce.Do(func() { *firstError = err })
			return
		}
		cursor = page.Next
	}
}

func (a *App) fetchThread(ctx context.Context, q domain.SearchQuery, candidate domain.Article) (domain.Article, []domain.Reply, bool, int, error) {
	body := candidate
	var replies []domain.Reply
	cursor := domain.Cursor("")
	pages := 0
	for pages < q.ThreadPages {
		page, err := a.source.ReadThreadPage(ctx, domain.Scope{SiteID: "byr"}, candidate.Key, cursor, domain.DefaultRepliesPerPage)
		if err != nil {
			a.markAuthLost(err)
			return body, replies, true, pages, err
		}
		if pages == 0 || page.Article.Content != "" || page.Article.Author != "" {
			body = page.Article
		}
		replies = append(replies, page.Replies...)
		pages++
		if page.End || page.Next == "" {
			return body, replies, true, pages, nil
		}
		if page.Next == cursor {
			return body, replies, false, pages, fmt.Errorf("article %s repeated cursor %q", candidate.Key.ArticleID, page.Next)
		}
		cursor = page.Next
	}
	return body, replies, false, pages, nil
}

func (a *App) setBoardError(id string, result *BoardResult, err error) {
	a.mu.Lock()
	if run := a.runs[id]; run != nil {
		result.Error = err.Error()
	}
	a.mu.Unlock()
	a.changed()
}

func matchesQuery(article domain.Article, q domain.SearchQuery) bool {
	if article.Pinned {
		return false
	}
	if !q.Time.Contains(article.CreatedAt) {
		return false
	}
	if q.Author != "" && !strings.EqualFold(article.Author, q.Author) {
		return false
	}
	if len(q.Keywords) > 0 {
		title := strings.ToLower(article.Title)
		for _, keyword := range q.Keywords {
			if !strings.Contains(title, strings.ToLower(keyword)) {
				return false
			}
		}
	}
	return true
}

func resolveTimeRange(r domain.TimeRange, now time.Time) domain.TimeRange {
	switch r.Kind {
	case domain.TimeWeek:
		r.From = now.AddDate(0, 0, -7)
		r.To = time.Time{}
	case domain.TimeMonth:
		r.From = now.AddDate(0, -1, 0)
		r.To = time.Time{}
	case domain.TimeCustom:
		// The caller supplies both bounds.
	case domain.TimeAll:
		r.From = time.Time{}
		r.To = time.Time{}
	default:
		r = domain.TimeRange{Kind: domain.TimeAll}
	}
	return r
}

func searchTitle(q domain.SearchQuery) string {
	title := strings.Join(q.Keywords, " ")
	if q.Author != "" {
		if title == "" {
			return "@" + q.Author
		}
		title += " @" + q.Author
	}
	if title == "" {
		title = "搜索"
	}
	return title
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
