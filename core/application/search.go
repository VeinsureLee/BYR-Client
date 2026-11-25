package application

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

// SearchSource identifies where a search result was produced. Keeping this in
// the shared application layer lets both clients present the same result
// semantics without knowing the transport details.
type SearchSource string

const SearchSourceRemote SearchSource = "remote"

// SearchRequest is the cross-platform article-search contract. A blank Board
// means full-site search. The remote endpoint requires a concrete board and
// returns no results for an empty filter, so the application resolves the
// forum's board tree and queries each board.
type SearchRequest struct {
	Keyword string
	Author  string
	Board   string
	Page    int
	Size    int
}

// SearchResult is deliberately independent of UI pagination widgets.
type SearchResult struct {
	Articles   []forum.Article
	Pagination forum.Pagination
	Source     SearchSource
	// Truncated reports that the merged result window was clipped at the
	// application-layer fetch cap, not that a board failed.
	Truncated bool
	// Incomplete reports that at least one board query failed. Articles and
	// pagination describe only the boards that succeeded.
	Incomplete   bool
	FailedBoards int
}

// Search validates and executes the shared remote article-search use case.
// Empty filters are rejected here so desktop and Android cannot accidentally
// issue an unbounded request with different behavior.
func (a *App) Search(ctx context.Context, request SearchRequest) (SearchResult, error) {
	keyword := strings.TrimSpace(request.Keyword)
	author := strings.TrimSpace(request.Author)
	if keyword == "" && author == "" {
		return SearchResult{}, errors.New("请输入搜索关键词或作者")
	}
	page := request.Page
	if page < 1 {
		page = 1
	}
	size := request.Size
	if size == 0 {
		size = 30
	}
	if size < 1 || size > 100 {
		return SearchResult{}, errors.New("搜索结果页大小必须在 1 到 100 之间")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (SearchResult, error) {
		if board := strings.TrimSpace(request.Board); board != "" {
			return a.searchBoard(requestCtx, keyword, author, board, page, size)
		}
		return a.searchAllBoards(requestCtx, keyword, author, page, size)
	})
}

func (a *App) searchBoard(ctx context.Context, keyword, author, board string, page, size int) (SearchResult, error) {
	v, err := a.forum.SearchArticles(ctx, forum.Search{
		Board:  board,
		Title:  keyword,
		Author: author,
		Page:   forum.Page{Number: page, Size: size},
	})
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{
		Articles:   v.Articles,
		Pagination: v.Pagination,
		Source:     SearchSourceRemote,
	}, nil
}

type boardSearchResult struct {
	board    string
	articles []forum.Article
	total    int
}

// searchAllBoards queries every board because the remote search endpoint
// requires a concrete board filter. Results are merged by post time, but only
// the window needed for the requested page is retained so mobile clients do
// not accumulate hundreds of boards worth of articles.
func (a *App) searchAllBoards(ctx context.Context, keyword, author string, page, size int) (SearchResult, error) {
	boards, err := a.searchableBoards(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	if len(boards) == 0 {
		return SearchResult{Source: SearchSourceRemote}, nil
	}

	fetchSize := page * size
	truncated := false
	if fetchSize > 100 {
		fetchSize = 100
		truncated = true
	}
	if fetchSize < 1 {
		fetchSize = 1
	}

	results, failed, firstErr, authErr := searchBoards(ctx, a.forum, boards, forum.Search{
		Title:  keyword,
		Author: author,
		Page:   forum.Page{Number: 1, Size: fetchSize},
	})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return SearchResult{}, ctxErr
	}
	// Authentication is not a partial-board failure: an expired session would
	// hide every missing board and must win over earlier 5xx responses.
	if authErr != nil {
		return SearchResult{}, authErr
	}
	if len(results) == 0 && firstErr != nil {
		return SearchResult{}, firstErr
	}

	articles := make([]forum.Article, 0, fetchSize)
	seen := make(map[string]bool)
	totalItems := 0
	for _, result := range results {
		totalItems += result.total
		for _, article := range result.articles {
			if article.Board == "" {
				article.Board = result.board
			}
			id := article.GroupID
			if id == "" {
				id = article.ID
			}
			key := article.Board + "\x00" + string(id)
			if id == "" {
				key += "\x00" + article.Title + "\x00" + string(article.Author.ID)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			articles = append(articles, article)
		}
		// Keep only the newest window; later boards can only push older items
		// further down, so dropping them here is safe.
		if len(articles) > fetchSize*2 {
			sortArticles(articles)
			articles = articles[:fetchSize]
		}
	}
	sortArticles(articles)
	if len(articles) > fetchSize {
		articles = articles[:fetchSize]
	}
	if totalItems < len(articles) {
		totalItems = len(articles)
	}
	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + size - 1) / size
	}
	start := (page - 1) * size
	if start > len(articles) {
		start = len(articles)
	}
	end := start + size
	if end > len(articles) {
		end = len(articles)
	}
	return SearchResult{
		Articles: articles[start:end],
		Pagination: forum.Pagination{
			TotalPages:   forum.Number(totalPages),
			CurrentPage:  forum.Number(page),
			ItemsPerPage: forum.Number(size),
			TotalItems:   forum.Number(totalItems),
		},
		Source:       SearchSourceRemote,
		Truncated:    truncated,
		Incomplete:   failed > 0,
		FailedBoards: failed,
	}, nil
}

func (a *App) searchableBoards(ctx context.Context) ([]string, error) {
	root, err := a.forum.Sections(ctx)
	if err != nil {
		return nil, err
	}
	pending := make([]string, 0, len(root.Sections))
	for _, section := range root.Sections {
		pending = append(pending, string(section.Name))
	}
	seenSections := make(map[string]bool)
	seenBoards := make(map[string]bool)
	boards := make([]string, 0)
	for len(pending) > 0 {
		name := pending[0]
		pending = pending[1:]
		if name == "" || seenSections[name] {
			continue
		}
		seenSections[name] = true
		section, err := a.forum.Section(ctx, name)
		if err != nil {
			return nil, err
		}
		for _, board := range section.Boards {
			if board.Name != "" && !seenBoards[board.Name] {
				seenBoards[board.Name] = true
				boards = append(boards, board.Name)
			}
		}
		for _, subsection := range section.Subsections {
			if subsection.Name != "" && !seenSections[string(subsection.Name)] {
				pending = append(pending, string(subsection.Name))
			}
		}
	}
	return boards, nil
}

// searchBoards tolerates individual board failures so one unavailable board
// does not discard every other board's results. Authentication failures are
// propagated because continuing without a valid session would be misleading.
type articleSearchClient interface {
	SearchArticles(context.Context, forum.Search) (forum.SearchResults, error)
}

func searchBoards(ctx context.Context, client articleSearchClient, boards []string, query forum.Search) ([]boardSearchResult, int, error, error) {
	const workerCount = 6
	workers := workerCount
	if len(boards) < workers {
		workers = len(boards)
	}
	jobs := make(chan string)
	results := make([]boardSearchResult, 0, len(boards))
	var mu sync.Mutex
	var firstErr, authErr error
	failed := 0
	var wg sync.WaitGroup
	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-searchCtx.Done():
				return
			case board, ok := <-jobs:
				if !ok {
					return
				}
				q := query
				q.Board = board
				v, err := client.SearchArticles(searchCtx, q)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					if errors.Is(err, forum.ErrUnauthenticated) {
						mu.Lock()
						if authErr == nil {
							authErr = err
						}
						mu.Unlock()
						cancel()
						return
					}
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					failed++
					mu.Unlock()
					continue
				}
				mu.Lock()
				results = append(results, boardSearchResult{board: board, articles: v.Articles, total: int(v.Pagination.TotalItems)})
				mu.Unlock()
			}
		}
	}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}
	for _, board := range boards {
		select {
		case jobs <- board:
		case <-searchCtx.Done():
		}
		if searchCtx.Err() != nil {
			break
		}
	}
	close(jobs)
	wg.Wait()
	return results, failed, firstErr, authErr
}

func sortArticles(articles []forum.Article) {
	sort.SliceStable(articles, func(i, j int) bool {
		left, right := articleTime(articles[i]), articleTime(articles[j])
		if left == right {
			return string(articles[i].ID) > string(articles[j].ID)
		}
		return left > right
	})
}

func articleTime(article forum.Article) int64 {
	value, err := strconv.ParseInt(string(article.PostTime), 10, 64)
	if err != nil {
		return 0
	}
	return value
}
