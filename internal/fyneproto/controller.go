// Package fyneproto supplies the Fyne prototype with a UI-independent controller.
// It uses the existing application services and never reads saved credentials.
package fyneproto

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"BYR-Client/internal/ai"
	svc "BYR-Client/internal/app"
	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
	"BYR-Client/internal/ports"
	"BYR-Client/internal/store/memory"
)

type Options struct {
	StressReplies  int
	StreamInterval time.Duration
}

type BoardOption struct {
	ID   domain.BoardID
	Name string
	Path string
}

type ChatSnapshot struct {
	Question  string
	Answer    string
	Status    string
	Error     string
	Running   bool
	Simulated bool
	Trace     []string
}

type Snapshot struct {
	Ready          bool
	Loading        bool
	Error          string
	SourceLabel    string
	Boards         []BoardOption
	SelectedBoard  domain.BoardID
	SelectedThread domain.ArticleKey
	Board          svc.BoardSnapshot
	Thread         svc.ThreadSnapshot
	SearchID       string
	Search         svc.RunSnapshot
	Chat           ChatSnapshot
}

type operation struct {
	version uint64
	cancel  context.CancelFunc
}

type Controller struct {
	app      *svc.App
	ctx      context.Context
	cancel   context.CancelFunc
	interval time.Duration
	mu       sync.RWMutex
	state    Snapshot
	change   func()
	closed   bool
	started  bool
	board    operation
	thread   operation
	chat     operation
	wg       sync.WaitGroup
}

func NewDemo(options Options) *Controller {
	c := New(newPrototypeSource(time.Now(), options.StressReplies))
	if options.StreamInterval > 0 {
		c.interval = options.StreamInterval
	}
	return c
}

// New permits deterministic source injection in tests and later adapters.
// The prototype intentionally starts without an authenticator or persisted keys.
func New(source ports.ForumSource) *Controller {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Controller{
		app: svc.New(source, memory.New(), ports.SystemClock{}, 10*time.Minute),
		ctx: ctx, cancel: cancel, interval: 40 * time.Millisecond,
		state: Snapshot{SourceLabel: "离线演示 · 数据与会话仅保存在内存"},
	}
	c.app.SetNotify(c.emit)
	return c
}

// SetOnChange callbacks may run on any goroutine. A GUI must marshal updates
// onto its own event loop, and must not call Fyne widgets in this callback.
func (c *Controller) SetOnChange(fn func()) {
	c.mu.Lock()
	c.change = fn
	c.mu.Unlock()
}

func (c *Controller) emit() {
	c.mu.RLock()
	fn, closed := c.change, c.closed
	c.mu.RUnlock()
	if fn != nil && !closed {
		fn()
	}
}

func (c *Controller) Snapshot() Snapshot {
	c.mu.RLock()
	s := c.state
	s.Boards = append([]BoardOption(nil), s.Boards...)
	s.Board.Articles = append([]domain.Article(nil), s.Board.Articles...)
	s.Thread.Replies = append([]domain.Reply(nil), s.Thread.Replies...)
	s.Chat.Trace = append([]string(nil), s.Chat.Trace...)
	c.mu.RUnlock()
	if s.SearchID != "" {
		s.Search, _ = c.app.Run(s.SearchID)
	}
	return s
}

func (c *Controller) Start() {
	c.mu.Lock()
	if c.closed || c.started {
		c.mu.Unlock()
		return
	}
	c.started, c.state.Loading = true, true
	c.wg.Add(1)
	c.mu.Unlock()
	c.emit()
	go func() {
		defer c.wg.Done()
		err := c.app.Initialize(c.ctx)
		var options []BoardOption
		if err == nil {
			boards, _ := c.app.Navigation().Boards("", forum.BFS)
			for _, board := range boards {
				var path []string
				for _, node := range c.app.BoardPath(board) {
					path = append(path, node.Name)
				}
				options = append(options, BoardOption{ID: board, Name: c.app.BoardName(board), Path: strings.Join(path, " / ")})
			}
		}
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		c.state.Loading = false
		if err != nil {
			c.state.Error = err.Error()
		} else {
			c.state.Ready, c.state.Boards = true, options
		}
		c.mu.Unlock()
		c.emit()
		if err == nil && len(options) > 0 {
			c.SelectBoard(options[0].ID)
		}
	}()
}

// beginLocked invalidates previous results even if a source ignores cancellation.
func (c *Controller) beginLocked(op *operation) (context.Context, uint64) {
	if op.cancel != nil {
		op.cancel()
	}
	op.version++
	ctx, cancel := context.WithCancel(c.ctx)
	op.cancel = cancel
	return ctx, op.version
}

func (c *Controller) SelectBoard(board domain.BoardID) {
	c.mu.Lock()
	if c.closed || !c.state.Ready {
		c.mu.Unlock()
		return
	}
	ctx, version := c.beginLocked(&c.board)
	if c.thread.cancel != nil {
		c.thread.cancel()
	}
	c.thread.version++
	c.state.SelectedBoard, c.state.SelectedThread = board, domain.ArticleKey{}
	c.state.Board = svc.BoardSnapshot{BoardID: board, Name: c.app.BoardName(board), Loading: true}
	c.state.Thread = svc.ThreadSnapshot{}
	c.wg.Add(1)
	c.mu.Unlock()
	c.emit()
	go func() {
		defer c.wg.Done()
		err := c.app.OpenBoard(ctx, board)
		view, _ := c.app.Board(board)
		c.mu.Lock()
		if c.closed || version != c.board.version || ctx.Err() != nil {
			c.mu.Unlock()
			return
		}
		c.state.Board = view
		c.state.Board.Loading = false
		if err != nil {
			c.state.Board.Error = err.Error()
		}
		c.mu.Unlock()
		c.emit()
		if err == nil && len(view.Articles) > 0 {
			c.openThread(view.Articles[0].Key, version)
		}
	}()
}

func (c *Controller) LoadMoreBoard() {
	c.mu.Lock()
	if c.closed || c.state.Board.Loading || c.state.Board.End || c.state.SelectedBoard == "" {
		c.mu.Unlock()
		return
	}
	board := c.state.SelectedBoard
	ctx, version := c.beginLocked(&c.board)
	c.state.Board.Loading = true
	c.wg.Add(1)
	c.mu.Unlock()
	c.emit()
	go func() {
		defer c.wg.Done()
		err := c.app.LoadMoreBoard(ctx, board)
		view, _ := c.app.Board(board)
		c.mu.Lock()
		if c.closed || version != c.board.version || ctx.Err() != nil {
			c.mu.Unlock()
			return
		}
		c.state.Board = view
		c.state.Board.Loading = false
		if err != nil {
			c.state.Board.Error = err.Error()
		}
		c.mu.Unlock()
		c.emit()
	}()
}

func (c *Controller) OpenThread(key domain.ArticleKey) {
	c.openThread(key, 0)
}

func (c *Controller) openThread(key domain.ArticleKey, boardVersion uint64) {
	c.mu.Lock()
	if c.closed || !c.state.Ready || (boardVersion != 0 && (c.board.version != boardVersion || c.state.SelectedBoard != key.BoardID)) {
		c.mu.Unlock()
		return
	}
	ctx, version := c.beginLocked(&c.thread)
	c.state.SelectedThread = key
	c.state.Thread = svc.ThreadSnapshot{Key: key, Loading: true}
	c.wg.Add(1)
	c.mu.Unlock()
	c.emit()
	go func() {
		defer c.wg.Done()
		err := c.app.OpenThread(ctx, key)
		c.commitThread(ctx, version, key, err)
	}()
}

func (c *Controller) LoadMoreReplies() { c.loadReplies(false) }

// LoadAllReplies is an explicit offline stress-test action, not a default fetch.
func (c *Controller) LoadAllReplies() { c.loadReplies(true) }

func (c *Controller) loadReplies(all bool) {
	c.mu.Lock()
	if c.closed || c.state.Thread.Loading || c.state.Thread.End || c.state.SelectedThread.ArticleID == "" {
		c.mu.Unlock()
		return
	}
	key := c.state.SelectedThread
	ctx, version := c.beginLocked(&c.thread)
	c.state.Thread.Loading = true
	c.wg.Add(1)
	c.mu.Unlock()
	c.emit()
	go func() {
		defer c.wg.Done()
		for {
			err := c.app.LoadMoreReplies(ctx, key)
			view, _ := c.app.Thread(key)
			if err != nil || ctx.Err() != nil || !all || view.End {
				c.commitThread(ctx, version, key, err)
				return
			}
		}
	}()
}

func (c *Controller) commitThread(ctx context.Context, version uint64, key domain.ArticleKey, err error) {
	view, _ := c.app.Thread(key)
	c.mu.Lock()
	if c.closed || version != c.thread.version || ctx.Err() != nil {
		c.mu.Unlock()
		return
	}
	c.state.Thread = view
	c.state.Thread.Loading = false
	if err != nil {
		c.state.Thread.Error = err.Error()
	}
	c.mu.Unlock()
	c.emit()
}

func (c *Controller) Search(text string) {
	words := strings.Fields(text)
	c.mu.Lock()
	if c.closed || !c.state.Ready {
		c.mu.Unlock()
		return
	}
	previous := c.state.SearchID
	c.state.Error = ""
	c.mu.Unlock()
	if previous != "" {
		c.app.StopSearch(previous)
	}
	id, err := c.app.StartSearch(domain.SearchQuery{Keywords: words, PerBoard: 5, Total: 50, ThreadPages: 1, Concurrency: 3})
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		if id != "" {
			c.app.StopSearch(id)
		}
		return
	}
	if err != nil {
		c.state.Error = err.Error()
	} else {
		c.state.SearchID = id
	}
	c.mu.Unlock()
	c.emit()
}

func (c *Controller) StopSearch() {
	c.mu.RLock()
	id := c.state.SearchID
	c.mu.RUnlock()
	if id != "" {
		c.app.StopSearch(id)
	}
}

// StartChat uses a deterministic simulation unless an explicit config is given.
// Real model requests can use the same demo forum tools; API keys stay in memory.
func (c *Controller) StartChat(question string, config *ai.Config) {
	question = strings.TrimSpace(question)
	if question == "" {
		return
	}
	c.mu.Lock()
	if c.closed || !c.state.Ready {
		c.mu.Unlock()
		return
	}
	ctx, version := c.beginLocked(&c.chat)
	simulated := config == nil
	status := "正在请求模型（论坛工具使用演示数据）"
	if simulated {
		status = "正在播放离线模拟流（未调用模型）"
	}
	c.state.Chat = ChatSnapshot{Question: question, Status: status, Running: true, Simulated: simulated}
	c.wg.Add(1)
	c.mu.Unlock()
	c.emit()
	var copied ai.Config
	if config != nil {
		copied = *config
	}
	go func() {
		defer c.wg.Done()
		var err error
		if simulated {
			err = c.simulateChat(ctx, version, question)
		} else {
			_, err = c.app.Chat(ctx, copied, question, svc.ChatOptions{PerBoard: 5, Total: 20}, func(delta string) error {
				return c.appendChat(ctx, version, delta)
			}, nil, func(event svc.ToolEvent) {
				c.mu.Lock()
				if !c.closed && version == c.chat.version && ctx.Err() == nil {
					c.state.Chat.Trace = append(c.state.Chat.Trace, fmt.Sprintf("%s · %s", event.Name, event.Summary))
				}
				c.mu.Unlock()
				c.emit()
			})
		}
		c.mu.Lock()
		if c.closed || version != c.chat.version {
			c.mu.Unlock()
			return
		}
		c.state.Chat.Running = false
		switch {
		case errors.Is(err, context.Canceled):
			c.state.Chat.Status = "已停止，保留已收到内容"
		case err != nil:
			c.state.Chat.Status, c.state.Chat.Error = "请求失败", err.Error()
		case simulated:
			c.state.Chat.Status = "离线模拟完成 · 未调用模型"
		default:
			c.state.Chat.Status = "回答完成 · 论坛来源为演示数据"
		}
		c.mu.Unlock()
		c.emit()
	}()
}

func (c *Controller) simulateChat(ctx context.Context, version uint64, question string) error {
	answer := "## 离线模拟回答\n\n这段文字由原型本地生成，没有请求任何 AI 服务，也不代表真实论坛搜索结果。\n\n你的问题是：" + question + "\n\n### 可以验证的交互\n\n- 中文流式文字逐段出现。\n- 点击停止后保留已经显示的内容。\n- 返回阅读页面、调整窗口大小，再回到对话。\n- 输入中英文和多行内容，检查手机键盘遮挡。\n\n### 共享的业务核心\n\n版面列表、主题分页和关键词搜索均调用项目现有的 Go 应用服务。本原型包含一个超过千楼的离线压力帖，图片也由本地生成。\n\n### 验收提醒\n\n模拟流只能验证显示与取消行为。真实模型连接、真实论坛登录、手机后台恢复和实机性能需要分别验收。可在原型中显式填写模型配置进行真实请求；密钥不会保存到磁盘。\n\n**模拟输出结束。**"
	runes := []rune(answer)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for offset := 0; offset < len(runes); offset += 7 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.appendChat(ctx, version, string(runes[offset:min(offset+7, len(runes))])); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Controller) appendChat(ctx context.Context, version uint64, text string) error {
	c.mu.Lock()
	if c.closed || version != c.chat.version || ctx.Err() != nil {
		c.mu.Unlock()
		return context.Canceled
	}
	c.state.Chat.Answer += text
	c.mu.Unlock()
	c.emit()
	return nil
}

func (c *Controller) StopChat() {
	c.mu.Lock()
	if c.chat.cancel != nil {
		c.chat.cancel()
	}
	c.chat.version++
	if c.state.Chat.Running {
		c.state.Chat.Running = false
		c.state.Chat.Status = "已停止，保留已收到内容"
	}
	c.mu.Unlock()
	c.emit()
}

func (c *Controller) ReadImage(ctx context.Context, rawURL string) ([]byte, error) {
	return c.app.FetchImage(ctx, rawURL)
}

func (c *Controller) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed, c.change = true, nil
	c.cancel()
	c.mu.Unlock()
	c.app.SetNotify(nil)
	c.app.Close()
	c.wg.Wait()
}
