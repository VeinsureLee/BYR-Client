package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"BYR-Client/internal/ai"
	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
)

const (
	maxToolRounds = 4
	maxToolCalls  = 8
)

type ChatOptions struct {
	Scope    string
	Time     string
	Total    int
	PerBoard int
}

type Citation struct {
	Board string `json:"board"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type ToolEvent struct {
	CallID    string
	Name      string
	Phase     string // started / completed / failed
	Arguments string
	Summary   string
	Error     string
	Sources   []string
	Citations []Citation
}

type toolDefinition = ai.Tool

func (a *App) Chat(ctx context.Context, config ai.Config, question string, options ChatOptions, onDelta, onReasoning func(string) error, onTool func(ToolEvent)) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", errors.New("问题不能为空")
	}
	client := ai.New(config)
	messages := []ai.Message{
		{Role: "system", Content: chatSystemPrompt(time.Now(), options)},
		{Role: "user", Content: question},
	}
	tools := toolDefinitions()
	var full strings.Builder
	cache := make(map[string]cachedToolResult)
	toolCalls := 0
	duplicates := 0
	toolErrors := 0
	stopTools := false

	for round := 0; round < maxToolRounds && !stopTools; round++ {
		result, err := client.ChatStream(ctx, messages, tools, onDelta, onReasoning)
		full.WriteString(result.Content)
		if err != nil {
			return full.String(), err
		}
		if len(result.ToolCalls) == 0 {
			return full.String(), nil
		}
		messages = append(messages, ai.Message{
			Role: "assistant", Content: result.Content, ToolCalls: result.ToolCalls,
		})
		responded := make(map[string]bool)
		for _, call := range result.ToolCalls {
			if ctx.Err() != nil {
				return full.String(), ctx.Err()
			}
			if toolCalls >= maxToolCalls {
				stopTools = true
				break
			}
			key := toolCallKey(call.Function.Name, call.Function.Arguments)
			if cached, ok := cache[key]; ok {
				duplicates++
				payload := cached.payload()
				payload["cached"] = true
				payload["hint"] = "相同工具与参数已经执行过，不要重复调用；请直接使用上面的结果回答，或改用不同关键词/范围。"
				data, _ := json.Marshal(payload)
				messages = append(messages, ai.Message{
					Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: string(data),
				})
				responded[call.ID] = true
				if onTool != nil {
					onTool(ToolEvent{
						CallID: call.ID, Name: call.Function.Name, Phase: "completed", Arguments: call.Function.Arguments,
						Summary: "相同调用结果已复用", Sources: cached.sources, Citations: cached.citations,
					})
				}
				if duplicates >= 2 {
					stopTools = true
					break
				}
				continue
			}

			toolCalls++
			if onTool != nil {
				onTool(ToolEvent{
					CallID: call.ID, Name: call.Function.Name, Phase: "started",
					Arguments: call.Function.Arguments,
				})
			}
			value, sources, citations, summary, toolErr := a.runAITool(ctx, call.Function.Name, call.Function.Arguments)
			cache[key] = cachedToolResult{
				value: value, sources: sources, citations: citations, summary: summary, err: toolErr,
			}
			payload := value
			if toolErr != nil {
				toolErrors++
				payload = map[string]any{
					"error": toolErr.Error(),
					"hint":  "不要用相同参数重试；请更换范围/关键词，或直接说明该工具失败。",
				}
			}
			data, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				data = []byte(`{"error":"工具结果无法序列化"}`)
			}
			messages = append(messages, ai.Message{
				Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: string(data),
			})
			responded[call.ID] = true
			if onTool != nil {
				phase := "completed"
				if toolErr != nil {
					phase = "failed"
				}
				onTool(ToolEvent{
					CallID: call.ID, Name: call.Function.Name, Phase: phase, Arguments: call.Function.Arguments,
					Summary: summary, Sources: sources, Citations: citations, Error: errorString(toolErr),
				})
			}
			if toolErrors >= 3 {
				stopTools = true
				break
			}
		}
		// Every tool_call in the assistant message must have a matching tool
		// message, even when the budget stopped execution before reaching it.
		for _, call := range result.ToolCalls {
			if responded[call.ID] {
				continue
			}
			data := []byte(`{"error":"未执行：工具调用预算已用完","hint":"不要继续调用工具，请基于已有结果直接回答。"}`)
			messages = append(messages, ai.Message{
				Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: string(data),
			})
			if onTool != nil {
				onTool(ToolEvent{
					CallID: call.ID, Name: call.Function.Name, Phase: "failed", Arguments: call.Function.Arguments,
					Error: "未执行：工具调用预算已用完",
				})
			}
		}
	}

	// Do not end with a hard "too many calls" error: ask the model for a final
	// answer with tools disabled, based on what the tools already returned.
	messages = append(messages, ai.Message{
		Role: "system",
		Content: "工具预算已用完、出现重复调用或连续错误。不要再调用工具。请基于已获得的工具结果直接回答；" +
			"信息不足的地方明确说明，并保留已有来源链接。",
	})
	result, err := client.ChatStream(ctx, messages, nil, onDelta, onReasoning)
	full.WriteString(result.Content)
	if err != nil {
		return full.String(), err
	}
	return full.String(), nil
}

type cachedToolResult struct {
	value     any
	sources   []string
	citations []Citation
	summary   string
	err       error
}

func (c cachedToolResult) payload() map[string]any {
	if c.err != nil {
		return map[string]any{"error": c.err.Error()}
	}
	if value, ok := c.value.(map[string]any); ok {
		copy := make(map[string]any, len(value)+1)
		for key, item := range value {
			copy[key] = item
		}
		return copy
	}
	return map[string]any{"result": c.value}
}

func toolCallKey(name, arguments string) string {
	var value any
	if json.Unmarshal([]byte(arguments), &value) == nil {
		if data, err := json.Marshal(value); err == nil {
			return name + "|" + string(data)
		}
	}
	return name + "|" + strings.TrimSpace(arguments)
}

func chatSystemPrompt(now time.Time, options ChatOptions) string {
	scope := strings.TrimSpace(options.Scope)
	if scope == "" {
		scope = "全站"
	}
	timeRange := strings.TrimSpace(options.Time)
	if timeRange == "" {
		timeRange = "不限"
	}
	return "你是 BYR Client 的北邮人论坛助手。你可以调用工具读取论坛目录、搜索主题、读取主题回复、查询流量和最新帖。" +
		"论坛结构是 Site → Forum → SubForum* → Board → Article → Reply。" +
		"界面当前选择：范围=" + scope + "；时间范围=" + timeRange + "；全局最多帖子=" + strconv.Itoa(options.Total) + "；单版最多帖子=" + strconv.Itoa(options.PerBoard) + "。" +
		`规则：
1. 涉及论坛内容时必须先调用工具，不要编造帖子、作者、数量或链接。
2. 如果用户没有特别说明范围，优先使用界面当前范围；如果界面范围是全站，scope 留空。若问题涉及全站、多个主题或多个版面，不要只搜索一个 board，可以先用 list_boards 确认范围，再对多个 scope 分别调用 search_forum 并合并结果。
3. 单个版面没有结果不代表全站没有；必要时更换关键词、时间范围或扩大 scope 再查。
4. search_forum 当前的多个关键词是同时满足（AND）关系；搜索预算为每版约 10 条、总计约 30 条，结果不足时可以拆分关键词或分多次搜索。
5. 需要细节时使用 read_thread；需要活跃度时使用 get_traffic；需要最新内容时使用 latest_topic。
6. 工具返回的 url 是来源；回答末尾列出关键来源的标题和链接。工具失败或没有结果时明确说明，不要用猜测补齐。
7. 全站问题优先用 scope 留空的一次 search_forum；不要为每个版面分别搜索，除非用户明确要求逐版对比。不要重复相同工具和参数。
8. 搜索结果已含少量回复；只在确实需要细节时 read_thread，最多读取最相关的 1-2 篇，除非用户明确要求更多。工具失败后不要用相同参数重试。
9. 回答简洁，使用中文；需要时可分点。
当前时间：` + now.Format("2006-01-02 15:04") + "。"
}

func toolDefinitions() []ai.Tool {
	return []ai.Tool{
		{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "list_boards",
				Description: "列出论坛的 forum/sub forum/board，用于确认版块名称、路径和 board_id。问题涉及不熟悉的版面时先调用它。",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"scope":{"type":"string","description":"可选范围：forum/sub forum/board 名称或 ID；留空为全站"},
						"query":{"type":"string","description":"按名称或 board_id 过滤，可留空"},
						"limit":{"type":"integer","description":"最多返回多少个版面，默认 40，最大 100"}
					}
				}`),
			},
		},
		{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "search_forum",
				Description: "按关键词、作者和最近天数搜索论坛主题，返回主题摘要和少量回复。scope 留空表示全站，不是单一版面；全站或多版面问题请留空 scope，或对多个 scope 分别调用后合并。多个关键词是 AND 关系。每版约 10 条、总计约 30 条，结果不足时可拆分关键词或多次搜索。",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"keywords":{"type":"array","items":{"type":"string"},"description":"关键词列表，可留空"},
						"author":{"type":"string","description":"可选作者 ID"},
						"time_days":{"type":"integer","description":"最近多少天；0 表示不限"},
						"scope":{"type":"string","description":"可选范围：forum/sub forum/board 名称或 ID；留空为全站"}
					}
				}`),
			},
		},
		{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "read_thread",
				Description: "读取一个主题的正文和回复。board_id/article_id 来自 search_forum 或 latest_topic 的结果。",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"board_id":{"type":"string","description":"版块英文名或 board_id"},
						"article_id":{"type":"string","description":"主题 ID"},
						"max_replies":{"type":"integer","description":"最多返回多少条回复，默认 20，最大 50"}
					},
					"required":["board_id","article_id"]
				}`),
			},
		},
		{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "get_traffic",
				Description: "查询 forum/sub forum/board 的在线人数、今日发帖、主题数和文章数。forum/sub forum 会聚合其后代版面。",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"node":{"type":"string","description":"forum/sub forum/board 名称或 ID"}
					},
					"required":["node"]
				}`),
			},
		},
		{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "latest_topic",
				Description: "读取指定 forum/sub forum/board 的最新非置顶主题；需要全站最新内容时，可先 list_boards 再按 forum 调用。",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"node":{"type":"string","description":"forum/sub forum/board 名称或 ID"}
					},
					"required":["node"]
				}`),
			},
		},
	}
}

func (a *App) runAITool(ctx context.Context, name, arguments string) (any, []string, []Citation, string, error) {
	switch name {
	case "list_boards":
		var args struct {
			Scope string `json:"scope"`
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := decodeToolArgs(arguments, &args); err != nil {
			return nil, nil, nil, "", err
		}
		return a.toolListBoards(args.Scope, args.Query, args.Limit)
	case "search_forum":
		var args struct {
			Keywords []string `json:"keywords"`
			Author   string   `json:"author"`
			TimeDays int      `json:"time_days"`
			Scope    string   `json:"scope"`
		}
		if err := decodeToolArgs(arguments, &args); err != nil {
			return nil, nil, nil, "", err
		}
		return a.toolSearchForum(ctx, args.Keywords, args.Author, args.TimeDays, args.Scope)
	case "read_thread":
		var args struct {
			BoardID    string `json:"board_id"`
			ArticleID  string `json:"article_id"`
			MaxReplies int    `json:"max_replies"`
		}
		if err := decodeToolArgs(arguments, &args); err != nil {
			return nil, nil, nil, "", err
		}
		return a.toolReadThread(ctx, args.BoardID, args.ArticleID, args.MaxReplies)
	case "get_traffic":
		var args struct {
			Node string `json:"node"`
		}
		if err := decodeToolArgs(arguments, &args); err != nil {
			return nil, nil, nil, "", err
		}
		return a.toolGetTraffic(ctx, args.Node)
	case "latest_topic":
		var args struct {
			Node string `json:"node"`
		}
		if err := decodeToolArgs(arguments, &args); err != nil {
			return nil, nil, nil, "", err
		}
		return a.toolLatestTopic(ctx, args.Node)
	default:
		return nil, nil, nil, "", fmt.Errorf("未知工具 %q", name)
	}
}

func decodeToolArgs(arguments string, target any) error {
	if strings.TrimSpace(arguments) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(arguments), target); err != nil {
		return fmt.Errorf("工具参数无效: %w", err)
	}
	return nil
}

func allScope(query string) bool {
	switch strings.ToLower(strings.TrimSpace(query)) {
	case "", "全站", "全部论坛", "all", "*":
		return true
	default:
		return false
	}
}

func (a *App) resolveNode(query string) (forum.Node, bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return forum.Node{}, false
	}
	a.mu.RLock()
	nav := a.nav
	nodes := append([]forum.Node(nil), a.catalog.Nodes...)
	a.mu.RUnlock()
	if nav == nil {
		return forum.Node{}, false
	}
	// A path such as "技术 › 编程" walks from the roots by name.
	if strings.Contains(query, "›") || strings.Contains(query, ">") {
		parts := strings.FieldsFunc(query, func(r rune) bool { return r == '›' || r == '>' })
		current := forum.NodeID("")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" || part == "全部论坛" {
				continue
			}
			children, err := nav.Children(current)
			if err != nil {
				return forum.Node{}, false
			}
			found := false
			for _, child := range children {
				if strings.EqualFold(child.Name, part) || strings.EqualFold(string(child.ID), part) {
					current = child.ID
					found = true
					break
				}
			}
			if !found {
				return forum.Node{}, false
			}
		}
		if current != "" {
			return nav.Node(current)
		}
	}
	var exact []forum.Node
	var partial []forum.Node
	for _, node := range nodes {
		switch {
		case strings.EqualFold(node.Name, query),
			strings.EqualFold(string(node.ID), query),
			node.BoardID != "" && strings.EqualFold(string(node.BoardID), query):
			exact = append(exact, node)
		case strings.Contains(strings.ToLower(node.Name), strings.ToLower(query)):
			partial = append(partial, node)
		}
	}
	if len(exact) == 1 {
		return exact[0], true
	}
	if len(exact) == 0 && len(partial) == 1 {
		return partial[0], true
	}
	return forum.Node{}, false
}

func (a *App) toolListBoards(scope, query string, limit int) (any, []string, []Citation, string, error) {
	nav := a.Navigation()
	if nav == nil {
		return nil, nil, nil, "", ErrNoCatalog
	}
	var scopeNode forum.NodeID
	if !allScope(scope) {
		node, ok := a.resolveNode(scope)
		if !ok {
			return nil, nil, nil, "", fmt.Errorf("找不到范围 %q", scope)
		}
		scopeNode = node.ID
	}
	boards, err := nav.Boards(scopeNode, forum.BFS)
	if err != nil {
		return nil, nil, nil, "", err
	}
	if limit <= 0 {
		limit = 40
	}
	if limit > 100 {
		limit = 100
	}
	query = strings.ToLower(strings.TrimSpace(query))
	type boardInfo struct {
		Name    string `json:"name"`
		BoardID string `json:"board_id"`
		NodeID  string `json:"node_id"`
		Path    string `json:"path"`
	}
	result := make([]boardInfo, 0, len(boards))
	for _, board := range boards {
		a.mu.RLock()
		nodeID := a.boardNodes[board]
		a.mu.RUnlock()
		node, ok := nav.Node(nodeID)
		if !ok {
			continue
		}
		path := node.Name
		if breadcrumb, err := nav.Breadcrumb(nodeID); err == nil {
			names := make([]string, 0, len(breadcrumb))
			for _, item := range breadcrumb {
				names = append(names, item.Name)
			}
			path = strings.Join(names, " › ")
		}
		if query != "" && !strings.Contains(strings.ToLower(path), query) &&
			!strings.Contains(strings.ToLower(string(board)), query) {
			continue
		}
		result = append(result, boardInfo{Name: node.Name, BoardID: string(board), NodeID: string(nodeID), Path: path})
		if len(result) >= limit {
			break
		}
	}
	return map[string]any{"boards": result, "count": len(result)}, nil, nil, fmt.Sprintf("列出 %d 个版面", len(result)), nil
}

func (a *App) toolSearchForum(ctx context.Context, keywords []string, author string, timeDays int, scope string) (any, []string, []Citation, string, error) {
	var scopeNode forum.NodeID
	if !allScope(scope) {
		node, ok := a.resolveNode(scope)
		if !ok {
			return nil, nil, nil, "", fmt.Errorf("找不到范围 %q", scope)
		}
		scopeNode = node.ID
	}
	query := domain.SearchQuery{
		Keywords:    keywords,
		Author:      strings.TrimSpace(author),
		ScopeNode:   scopeNode,
		PerBoard:    10,
		Total:       30,
		ThreadPages: 2,
		Concurrency: 5,
	}
	if timeDays > 0 {
		query.Time = domain.TimeRange{Kind: domain.TimeCustom, From: a.clock.Now().AddDate(0, 0, -timeDays)}
	} else {
		query.Time = domain.TimeRange{Kind: domain.TimeAll}
	}
	run, err := a.SearchSync(ctx, query)
	if err != nil {
		return nil, nil, nil, "", err
	}
	type replyInfo struct {
		Floor   int    `json:"floor"`
		Author  string `json:"author"`
		Content string `json:"content"`
	}
	type topicInfo struct {
		Title      string      `json:"title"`
		Author     string      `json:"author"`
		Board      string      `json:"board"`
		BoardID    string      `json:"board_id"`
		ArticleID  string      `json:"article_id"`
		URL        string      `json:"url,omitempty"`
		CreatedAt  string      `json:"created_at,omitempty"`
		ReplyCount int         `json:"reply_count"`
		Replies    []replyInfo `json:"replies,omitempty"`
	}
	topics := make([]topicInfo, 0)
	sources := make([]string, 0)
	citations := make([]Citation, 0)
	for _, board := range run.Boards {
		for _, item := range board.Articles {
			link := a.ArticleURL(item.Article.Key)
			info := topicInfo{
				Title: item.Article.Title, Author: item.Article.Author, Board: board.Name,
				BoardID: string(item.Article.Key.BoardID), ArticleID: string(item.Article.Key.ArticleID),
				URL: link, ReplyCount: item.Article.ReplyCount,
			}
			if !item.Article.CreatedAt.IsZero() {
				info.CreatedAt = item.Article.CreatedAt.Format("2006-01-02 15:04")
			}
			for index, reply := range item.Replies {
				if index >= 4 {
					break
				}
				info.Replies = append(info.Replies, replyInfo{
					Floor: reply.Floor, Author: reply.Author, Content: truncateRunes(reply.Content, 240),
				})
			}
			topics = append(topics, info)
			if link != "" {
				sources = append(sources, link)
				citations = append(citations, Citation{Board: board.Name, Title: item.Article.Title, URL: link})
			}
		}
	}
	result := map[string]any{
		"topics": topics, "count": len(topics), "stop_reason": string(run.StopReason),
	}
	if run.Error != "" {
		result["error"] = run.Error
	}
	return result, sources, citations, fmt.Sprintf("搜索到 %d 个主题", len(topics)), nil
}

func (a *App) toolReadThread(ctx context.Context, boardID, articleID string, maxReplies int) (any, []string, []Citation, string, error) {
	boardID = strings.TrimSpace(boardID)
	articleID = strings.TrimSpace(articleID)
	if boardID == "" || articleID == "" {
		return nil, nil, nil, "", fmt.Errorf("board_id 和 article_id 不能为空")
	}
	key := domain.ArticleKey{SiteID: "byr", BoardID: domain.BoardID(boardID), ArticleID: domain.ArticleID(articleID)}
	if err := a.OpenThread(ctx, key); err != nil {
		return nil, nil, nil, "", err
	}
	thread, ok := a.Thread(key)
	if !ok {
		return nil, nil, nil, "", fmt.Errorf("主题不存在")
	}
	if maxReplies <= 0 {
		maxReplies = 20
	}
	if maxReplies > 50 {
		maxReplies = 50
	}
	for len(thread.Replies) < maxReplies && !thread.End && thread.Next != "" {
		if err := a.LoadMoreReplies(ctx, key); err != nil {
			break
		}
		thread, ok = a.Thread(key)
		if !ok {
			break
		}
	}
	type replyInfo struct {
		Floor   int    `json:"floor"`
		Author  string `json:"author"`
		Created string `json:"created_at,omitempty"`
		Content string `json:"content"`
	}
	replies := make([]replyInfo, 0, maxReplies)
	for index, reply := range thread.Replies {
		if index >= maxReplies {
			break
		}
		created := ""
		if !reply.CreatedAt.IsZero() {
			created = reply.CreatedAt.Format("2006-01-02 15:04")
		}
		replies = append(replies, replyInfo{Floor: reply.Floor, Author: reply.Author, Created: created, Content: truncateRunes(reply.Content, 800)})
	}
	link := a.ArticleURL(key)
	result := map[string]any{
		"title": thread.Article.Title, "author": thread.Article.Author,
		"board": a.BoardName(key.BoardID), "board_id": boardID, "article_id": articleID,
		"url": link, "content": truncateRunes(thread.Article.Content, 6000),
		"reply_count": thread.Article.ReplyCount, "replies": replies,
	}
	sources := []string{}
	if link != "" {
		sources = append(sources, link)
	}
	citations := []Citation{{Board: a.BoardName(key.BoardID), Title: thread.Article.Title, URL: link}}
	if link == "" {
		citations = nil
	}
	return result, sources, citations, fmt.Sprintf("读取主题《%s》和 %d 条回复", thread.Article.Title, len(replies)), nil
}

func (a *App) toolGetTraffic(ctx context.Context, name string) (any, []string, []Citation, string, error) {
	nodeID := forum.NodeID("")
	nodeName := "全站"
	if !allScope(name) {
		node, ok := a.resolveNode(name)
		if !ok {
			return nil, nil, nil, "", fmt.Errorf("找不到节点 %q", name)
		}
		nodeID = node.ID
		nodeName = node.Name
	}
	if err := a.LoadTraffic(ctx, nodeID); err != nil {
		return nil, nil, nil, "", err
	}
	traffic, known := a.TrafficForNode(nodeID)
	if !known {
		return nil, nil, nil, "", fmt.Errorf("暂未读取到流量")
	}
	return map[string]any{
		"node": nodeName, "node_id": string(nodeID),
		"online_users": traffic.OnlineUsers, "today_posts": traffic.TodayPosts,
		"threads": traffic.Threads, "posts": traffic.Posts,
	}, nil, nil, fmt.Sprintf("读取 %s 的流量", nodeName), nil
}

func (a *App) toolLatestTopic(ctx context.Context, name string) (any, []string, []Citation, string, error) {
	node, ok := a.resolveNode(name)
	if !ok {
		return nil, nil, nil, "", fmt.Errorf("找不到节点 %q", name)
	}
	if err := a.LoadLatestTopic(ctx, node.ID); err != nil {
		return nil, nil, nil, "", err
	}
	article, board, found := a.LatestTopic(node.ID)
	if !found {
		return nil, nil, nil, "", fmt.Errorf("没有读到最新主题")
	}
	link := a.ArticleURL(article.Key)
	result := map[string]any{
		"title": article.Title, "author": article.Author,
		"board": a.BoardName(board), "board_id": string(article.Key.BoardID),
		"article_id": string(article.Key.ArticleID), "url": link,
	}
	sources := []string{}
	if link != "" {
		sources = append(sources, link)
	}
	citations := []Citation{{Board: a.BoardName(board), Title: article.Title, URL: link}}
	if link == "" {
		citations = nil
	}
	return result, sources, citations, "读取最新主题《" + article.Title + "》", nil
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
