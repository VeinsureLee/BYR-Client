// Package app implements the application services described in docs/backend:
// catalog initialization, on-demand browsing, keyword search, incremental
// content writes and activity-based refresh scheduling.
package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
	"BYR-Client/internal/ports"
	"BYR-Client/internal/refresh"
)

var (
	ErrNoCatalog = errors.New("catalog is not initialized")
	ErrNoNode    = errors.New("forum node not found")
)

type BoardSnapshot struct {
	BoardID    domain.BoardID
	Name       string
	Articles   []domain.Article
	Next       domain.Cursor
	End        bool
	ObservedAt time.Time
	Loading    bool
	Error      string
}

type ThreadSnapshot struct {
	Key        domain.ArticleKey
	Article    domain.Article
	Replies    []domain.Reply
	Next       domain.Cursor
	End        bool
	PagesRead  int
	ObservedAt time.Time
	Loading    bool
	Error      string
}

type App struct {
	source ports.ForumSource
	store  ports.ObservationStore
	auth   ports.Authenticator
	clock  ports.Clock
	idle   time.Duration

	rootCtx context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	notifyMu sync.RWMutex
	notify   func()

	mu              sync.RWMutex
	session         ports.Session
	nav             *forum.Navigation
	catalog         domain.Catalog
	catalogWarnings []string
	boardNodes      map[domain.BoardID]forum.NodeID
	boardViews      map[domain.BoardID]*BoardSnapshot
	threads         map[string]*ThreadSnapshot
	boardGen        map[domain.BoardID]uint64
	threadGen       map[string]uint64
	traffic         map[domain.BoardID]domain.Traffic
	trafficAt       map[domain.BoardID]time.Time
	trafficLoad     map[forum.NodeID]bool
	trafficErrors   map[forum.NodeID]string
	latest          map[forum.NodeID]domain.Article
	latestBoard     map[forum.NodeID]domain.BoardID
	latestLoad      map[forum.NodeID]bool
	latestErrors    map[forum.NodeID]string
	latestSem       chan struct{}
	runs            map[string]*SearchRun
	runSeq          int
	history         []*HistoryEntry
	lastDelete      string
	refresh         *refresh.Scheduler
}

type CatalogStats struct {
	Ready     bool
	Forums    int
	SubForums int
	Boards    int
	FetchedAt time.Time
	Warnings  []string
}

func New(source ports.ForumSource, store ports.ObservationStore, clock ports.Clock, idle time.Duration) *App {
	return NewWithAuth(source, store, nil, clock, idle)
}

func NewWithAuth(source ports.ForumSource, store ports.ObservationStore, auth ports.Authenticator, clock ports.Clock, idle time.Duration) *App {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	if idle <= 0 {
		idle = 10 * time.Minute
	}
	ctx, cancel := context.WithCancel(context.Background())
	session := ports.Session{SiteID: "byr", AccountID: "demo", DisplayName: "演示用户", Epoch: 1, LoggedIn: true}
	if auth != nil {
		session = ports.Session{SiteID: "byr"}
	}
	a := &App{
		source:        source,
		store:         store,
		auth:          auth,
		clock:         clock,
		session:       session,
		idle:          idle,
		rootCtx:       ctx,
		cancel:        cancel,
		boardNodes:    make(map[domain.BoardID]forum.NodeID),
		boardViews:    make(map[domain.BoardID]*BoardSnapshot),
		threads:       make(map[string]*ThreadSnapshot),
		boardGen:      make(map[domain.BoardID]uint64),
		threadGen:     make(map[string]uint64),
		traffic:       make(map[domain.BoardID]domain.Traffic),
		trafficAt:     make(map[domain.BoardID]time.Time),
		trafficLoad:   make(map[forum.NodeID]bool),
		trafficErrors: make(map[forum.NodeID]string),
		latest:        make(map[forum.NodeID]domain.Article),
		latestBoard:   make(map[forum.NodeID]domain.BoardID),
		latestLoad:    make(map[forum.NodeID]bool),
		latestErrors:  make(map[forum.NodeID]string),
		latestSem:     make(chan struct{}, 3),
		runs:          make(map[string]*SearchRun),
	}
	a.refresh = refresh.New(clock, idle)
	return a
}

func (a *App) SetNotify(fn func()) {
	a.notifyMu.Lock()
	a.notify = fn
	a.notifyMu.Unlock()
}

func (a *App) changed() {
	a.notifyMu.RLock()
	fn := a.notify
	a.notifyMu.RUnlock()
	if fn != nil {
		fn()
	}
}

func (a *App) Start() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-a.rootCtx.Done():
				return
			case now := <-ticker.C:
				a.runDueRefresh(now)
			}
		}
	}()
}

func (a *App) Close() {
	a.cancel()
	a.wg.Wait()
}

func (a *App) Session() ports.Session {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.session
}

func (a *App) Authenticated() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.auth == nil || a.session.LoggedIn
}

func (a *App) Login(ctx context.Context, username, password string) (ports.Session, error) {
	if a.auth == nil {
		return a.Session(), nil
	}
	session, err := a.auth.Login(ctx, username, password)
	if err != nil {
		return ports.Session{}, err
	}
	a.resetAccountState()
	a.mu.Lock()
	a.session = session
	a.mu.Unlock()
	a.changed()
	return session, nil
}

func (a *App) RestoreSession(ctx context.Context) (ports.Session, bool, error) {
	if a.auth == nil {
		return a.Session(), true, nil
	}
	session, ok, err := a.auth.Restore(ctx)
	if err != nil || !ok {
		return ports.Session{}, ok, err
	}
	a.resetAccountState()
	a.mu.Lock()
	a.session = session
	a.mu.Unlock()
	a.changed()
	return session, true, nil
}

func (a *App) Logout(ctx context.Context) error {
	var err error
	if a.auth != nil {
		err = a.auth.Logout(ctx)
	}
	a.resetAccountState()
	a.refresh.Deactivate()
	return err
}

func (a *App) resetAccountState() {
	a.mu.Lock()
	for _, run := range a.runs {
		if run.cancel != nil {
			run.cancel()
		}
	}
	a.runs = make(map[string]*SearchRun)
	a.history = nil
	a.lastDelete = ""
	a.boardViews = make(map[domain.BoardID]*BoardSnapshot)
	a.threads = make(map[string]*ThreadSnapshot)
	a.boardGen = make(map[domain.BoardID]uint64)
	a.threadGen = make(map[string]uint64)
	a.boardNodes = make(map[domain.BoardID]forum.NodeID)
	a.traffic = make(map[domain.BoardID]domain.Traffic)
	a.trafficAt = make(map[domain.BoardID]time.Time)
	a.trafficLoad = make(map[forum.NodeID]bool)
	a.trafficErrors = make(map[forum.NodeID]string)
	a.latest = make(map[forum.NodeID]domain.Article)
	a.latestBoard = make(map[forum.NodeID]domain.BoardID)
	a.latestLoad = make(map[forum.NodeID]bool)
	a.latestErrors = make(map[forum.NodeID]string)
	a.nav = nil
	a.catalog = domain.Catalog{}
	a.catalogWarnings = nil
	a.session = ports.Session{SiteID: "byr"}
	a.mu.Unlock()
	if resetter, ok := a.store.(interface{ Reset() }); ok {
		resetter.Reset()
	}
	a.refresh.Deactivate()
	a.changed()
}

func (a *App) Initialize(ctx context.Context) error {
	a.mu.RLock()
	loggedIn := a.session.LoggedIn
	a.mu.RUnlock()
	if a.auth != nil && !loggedIn {
		return ports.ErrAuthRequired
	}
	catalog, err := a.source.ReadCatalog(ctx)
	if err != nil {
		return err
	}
	nav, err := forum.NewNavigation(catalog.Roots, catalog.Nodes)
	if err != nil {
		return fmt.Errorf("validate catalog: %w", err)
	}
	if err := a.store.SaveCatalog(catalog); err != nil {
		return err
	}
	nodes := make(map[domain.BoardID]forum.NodeID)
	for _, node := range catalog.Nodes {
		if node.Kind == forum.KindBoard {
			nodes[node.BoardID] = node.ID
		}
	}
	a.mu.Lock()
	a.catalog = catalog
	a.catalogWarnings = append([]string(nil), catalog.Warnings...)
	a.nav = nav
	a.boardNodes = nodes
	a.mu.Unlock()
	a.changed()
	return nil
}

func (a *App) Navigation() *forum.Navigation {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.nav
}

func (a *App) Catalog() (domain.Catalog, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.nav == nil {
		return domain.Catalog{}, false
	}
	return a.catalog, true
}

func (a *App) CatalogStats() CatalogStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	stats := CatalogStats{
		Ready:     a.nav != nil,
		FetchedAt: a.catalog.FetchedAt,
		Warnings:  append([]string(nil), a.catalogWarnings...),
	}
	for _, node := range a.catalog.Nodes {
		switch node.Kind {
		case forum.KindForum:
			stats.Forums++
		case forum.KindSubForum:
			stats.SubForums++
		case forum.KindBoard:
			stats.Boards++
		}
	}
	return stats
}

func (a *App) BoardName(board domain.BoardID) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	nodeID, ok := a.boardNodes[board]
	if !ok {
		return string(board)
	}
	nav := a.nav
	if nav == nil {
		return string(board)
	}
	node, ok := nav.Node(nodeID)
	if !ok {
		return string(board)
	}
	return node.Name
}

func (a *App) BoardPath(board domain.BoardID) []forum.Node {
	a.mu.RLock()
	nodeID, ok := a.boardNodes[board]
	nav := a.nav
	a.mu.RUnlock()
	if !ok || nav == nil {
		return nil
	}
	path, err := nav.Breadcrumb(nodeID)
	if err != nil {
		return nil
	}
	return path
}

func (a *App) Children(node forum.NodeID) ([]forum.Node, error) {
	nav := a.Navigation()
	if nav == nil {
		return nil, ErrNoCatalog
	}
	children, err := nav.Children(node)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoNode, err)
	}
	return children, nil
}

func (a *App) Breadcrumb(node forum.NodeID) []forum.Node {
	nav := a.Navigation()
	if nav == nil {
		return nil
	}
	path, err := nav.Breadcrumb(node)
	if err != nil {
		return nil
	}
	return path
}

func (a *App) markAuthLost(err error) {
	if !errors.Is(err, ports.ErrAuthRequired) {
		return
	}
	a.mu.Lock()
	a.session.LoggedIn = false
	a.mu.Unlock()
	a.changed()
}

func (a *App) requireSession() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.auth != nil && !a.session.LoggedIn {
		return ports.ErrAuthRequired
	}
	return nil
}

func (a *App) OpenBoard(ctx context.Context, board domain.BoardID) error {
	if err := a.requireSession(); err != nil {
		return err
	}
	a.mu.Lock()
	a.boardGen[board]++
	generation := a.boardGen[board]
	a.mu.Unlock()
	a.setBoardLoading(board, true)
	page, err := a.source.ReadBoardPage(ctx, domain.Scope{SiteID: "byr"}, board, "", domain.DefaultBoardBatchSize)
	if err != nil {
		a.failBoard(board, generation, err)
		return err
	}
	for _, article := range page.Articles {
		a.store.UpsertArticle(article)
	}
	a.mu.Lock()
	if a.boardGen[board] == generation {
		a.boardViews[board] = &BoardSnapshot{
			BoardID:    board,
			Name:       a.boardNameLocked(board),
			Articles:   page.Articles,
			Next:       page.Next,
			End:        page.End,
			ObservedAt: page.ObservedAt,
		}
	}
	a.mu.Unlock()
	a.changed()
	return nil
}

func (a *App) LoadMoreBoard(ctx context.Context, board domain.BoardID) error {
	if err := a.requireSession(); err != nil {
		return err
	}
	a.mu.RLock()
	existing := a.boardViews[board]
	generation := a.boardGen[board]
	a.mu.RUnlock()
	if existing == nil {
		return a.OpenBoard(ctx, board)
	}
	if existing.End || existing.Next == "" {
		return nil
	}
	a.setBoardLoading(board, true)
	page, err := a.source.ReadBoardPage(ctx, domain.Scope{SiteID: "byr"}, board, existing.Next, domain.DefaultBoardBatchSize)
	if err != nil {
		a.failBoard(board, generation, err)
		return err
	}
	for _, article := range page.Articles {
		a.store.UpsertArticle(article)
	}
	a.mu.Lock()
	current := a.boardViews[board]
	if current != nil && a.boardGen[board] == generation {
		current.Articles = append(current.Articles, page.Articles...)
		current.Next = page.Next
		current.End = page.End
		current.ObservedAt = page.ObservedAt
		current.Loading = false
		current.Error = ""
	}
	a.mu.Unlock()
	a.changed()
	return nil
}

func (a *App) Board(board domain.BoardID) (BoardSnapshot, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	snapshot := a.boardViews[board]
	if snapshot == nil {
		return BoardSnapshot{}, false
	}
	copy := *snapshot
	copy.Articles = append([]domain.Article(nil), snapshot.Articles...)
	return copy, true
}

func (a *App) OpenThread(ctx context.Context, key domain.ArticleKey) error {
	if err := a.requireSession(); err != nil {
		return err
	}
	threadKey := threadMapKey(key)
	a.mu.Lock()
	a.threadGen[threadKey]++
	generation := a.threadGen[threadKey]
	a.mu.Unlock()
	a.setThreadLoading(key, true)
	page, err := a.source.ReadThreadPage(ctx, domain.Scope{SiteID: "byr"}, key, "", domain.DefaultRepliesPerPage)
	if err != nil {
		a.failThread(key, generation, err)
		return err
	}
	a.store.UpsertArticle(page.Article)
	for _, reply := range page.Replies {
		a.store.UpsertReply(reply)
	}
	a.mu.Lock()
	if a.threadGen[threadKey] == generation {
		a.threads[threadKey] = &ThreadSnapshot{
			Key:        key,
			Article:    page.Article,
			Replies:    page.Replies,
			Next:       page.Next,
			End:        page.End,
			PagesRead:  page.PagesRead,
			ObservedAt: page.ObservedAt,
		}
	}
	a.mu.Unlock()
	a.changed()
	return nil
}

func (a *App) LoadMoreReplies(ctx context.Context, key domain.ArticleKey) error {
	if err := a.requireSession(); err != nil {
		return err
	}
	threadKey := threadMapKey(key)
	a.mu.RLock()
	existing := a.threads[threadKey]
	generation := a.threadGen[threadKey]
	a.mu.RUnlock()
	if existing == nil {
		return a.OpenThread(ctx, key)
	}
	if existing.End || existing.Next == "" {
		return nil
	}
	a.setThreadLoading(key, true)
	page, err := a.source.ReadThreadPage(ctx, domain.Scope{SiteID: "byr"}, key, existing.Next, domain.DefaultRepliesPerPage)
	if err != nil {
		a.failThread(key, generation, err)
		return err
	}
	for _, reply := range page.Replies {
		a.store.UpsertReply(reply)
	}
	a.mu.Lock()
	current := a.threads[threadKey]
	if current != nil && a.threadGen[threadKey] == generation {
		current.Replies = append(current.Replies, page.Replies...)
		current.Next = page.Next
		current.End = page.End
		current.PagesRead += page.PagesRead
		current.ObservedAt = page.ObservedAt
		current.Loading = false
		current.Error = ""
	}
	a.mu.Unlock()
	a.changed()
	return nil
}

func (a *App) Thread(key domain.ArticleKey) (ThreadSnapshot, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	snapshot := a.threads[threadMapKey(key)]
	if snapshot == nil {
		return ThreadSnapshot{}, false
	}
	copy := *snapshot
	copy.Replies = append([]domain.Reply(nil), snapshot.Replies...)
	return copy, true
}

func (a *App) ActivateBoard(board domain.BoardID) {
	a.refresh.Activate(refresh.Target{Kind: refresh.BoardTarget, BoardID: board})
	a.changed()
}

func (a *App) ActivateThread(key domain.ArticleKey) {
	a.refresh.Activate(refresh.Target{Kind: refresh.ThreadTarget, BoardID: key.BoardID, Article: key})
	a.changed()
}

func (a *App) Touch() {
	a.refresh.Touch()
}

func (a *App) SetApplicationActive(active bool) {
	a.refresh.SetApplicationActive(active)
	a.changed()
}

func (a *App) RefreshState() refresh.Lease {
	return a.refresh.Lease()
}

func (a *App) runDueRefresh(now time.Time) {
	trigger, ok := a.refresh.Due(now)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(a.rootCtx, 30*time.Second)
	defer cancel()
	var err error
	switch trigger.Target.Kind {
	case refresh.BoardTarget:
		err = a.OpenBoard(ctx, trigger.Target.BoardID)
	case refresh.ThreadTarget:
		err = a.OpenThread(ctx, trigger.Target.Article)
	}
	a.refresh.Finish(trigger, err == nil, a.clock.Now())
	a.changed()
}

func (a *App) Stats() ports.StoreStats {
	return a.store.Stats()
}

func (a *App) ArticleURL(key domain.ArticleKey) string {
	if provider, ok := a.source.(ports.URLProvider); ok {
		return provider.ArticleURL(key)
	}
	return ""
}

func (a *App) BrowserCookies() []ports.Cookie {
	if provider, ok := a.source.(ports.CookieProvider); ok {
		return provider.Cookies()
	}
	return nil
}

func (a *App) FetchImage(ctx context.Context, rawURL string) ([]byte, error) {
	if err := a.requireSession(); err != nil {
		return nil, err
	}
	reader, ok := a.source.(ports.ImageReader)
	if !ok {
		return nil, errors.New("当前数据源不支持图片读取")
	}
	return reader.ReadImage(ctx, rawURL)
}

func (a *App) setBoardLoading(board domain.BoardID, loading bool) {
	a.mu.Lock()
	view := a.boardViews[board]
	if view == nil {
		view = &BoardSnapshot{BoardID: board, Name: a.boardNameLocked(board)}
		a.boardViews[board] = view
	}
	view.Loading = loading
	if loading {
		view.Error = ""
	}
	a.mu.Unlock()
	a.changed()
}

func (a *App) failBoard(board domain.BoardID, generation uint64, err error) {
	a.markAuthLost(err)
	a.mu.Lock()
	if a.boardGen[board] == generation {
		view := a.boardViews[board]
		if view == nil {
			view = &BoardSnapshot{BoardID: board, Name: a.boardNameLocked(board)}
			a.boardViews[board] = view
		}
		view.Loading = false
		view.Error = err.Error()
	}
	a.mu.Unlock()
	a.changed()
}

func (a *App) setThreadLoading(key domain.ArticleKey, loading bool) {
	threadKey := threadMapKey(key)
	a.mu.Lock()
	view := a.threads[threadKey]
	if view == nil {
		view = &ThreadSnapshot{Key: key}
		a.threads[threadKey] = view
	}
	view.Loading = loading
	if loading {
		view.Error = ""
	}
	a.mu.Unlock()
	a.changed()
}

func (a *App) failThread(key domain.ArticleKey, generation uint64, err error) {
	a.markAuthLost(err)
	threadKey := threadMapKey(key)
	a.mu.Lock()
	if a.threadGen[threadKey] == generation {
		view := a.threads[threadKey]
		if view == nil {
			view = &ThreadSnapshot{Key: key}
			a.threads[threadKey] = view
		}
		view.Loading = false
		view.Error = err.Error()
	}
	a.mu.Unlock()
	a.changed()
}

func (a *App) boardNameLocked(board domain.BoardID) string {
	nodeID, ok := a.boardNodes[board]
	if !ok || a.nav == nil {
		return string(board)
	}
	node, ok := a.nav.Node(nodeID)
	if !ok {
		return string(board)
	}
	return node.Name
}

func threadMapKey(key domain.ArticleKey) string {
	return key.SiteID + "|" + string(key.BoardID) + "|" + string(key.ArticleID)
}
