package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	svc "BYR-Client/internal/app"
	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
)

type chatTurn struct {
	ID            int
	Role          string
	Provider      string
	Model         string
	Text          string
	Reasoning     string
	Trace         []string
	Tools         []toolTrace
	Sources       []string
	Citations     []svc.Citation
	Running       bool
	Err           string
	ReasoningOpen bool
	ToolsOpen     bool
	CiteOpen      bool
	Groups        map[string]bool
}

type toolTrace struct {
	ID        string
	Name      string
	Arguments string
	Summary   string
	Error     string
	Running   bool
	Citations []svc.Citation
}

func (u *UI) searchContent(c *ui.Context) {
	if u.sourceKey != nil {
		u.threadPane(c, *u.sourceKey, true, u.sourceFloor)
		return
	}
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		ui.Row(c).Padding(12, 24).Gap(12).Wrap().Background(t.Surface).AlignItems(ui.Center).Children(func() {
			previous := u.searchMode
			ui.Segmented(c, &u.searchMode, "关键词检索", "AI 问答")
			if previous != u.searchMode {
				u.switchSearchMode(previous)
			}
			ui.Spacer(c)
			if u.searchMode == 1 {
				ui.Icon(c, iconSparkles).FontSize(u.fs(14)).TextColor(t.Accent)
				if u.aiKeySaved {
					ui.Textf(c, "%s · %s", u.aiProvider, u.aiModel).FontSize(u.fs(12)).TextColor(t.TextMuted)
				} else {
					ui.Button(c, "配置 AI").FontSize(u.fs(12)).OnClick(func() { u.openUtility(regionSettings) })
				}
			}
		})
		ui.Column(c).Key("search-mode-" + strconv.Itoa(u.searchMode)).Grow(1).FillWidth().
			Transition(ui.ElementTransition{Colors: true, Duration: 160 * time.Millisecond, Enter: &ui.Motion{}}).Children(func() {
			if u.searchMode == 0 {
				ui.Column(c).FillWidth().Padding(16, 24).Gap(10).Background(t.Surface).Children(func() { u.keywordForm(c) })
				u.keywordResults(c)
			} else {
				u.aiChat(c)
			}
		})
	})
}

func (u *UI) keywordForm(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(12).AlignItems(ui.End).Children(func() {
		ui.Column(c).Grow(1).Gap(4).Children(func() {
			ui.Text(c, "关键词").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
			u.keywordEditor(c)
		})
		ui.Column(c).Width(210).Gap(4).Children(func() {
			ui.Text(c, "作者 ID").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
			ui.TextInput(c, &u.author).Placeholder("可选，精确 ID").Label("作者 ID")
		})
	})
	ui.Row(c).Gap(10).AlignItems(ui.End).Children(func() {
		ui.Column(c).Width(150).Gap(4).Children(func() {
			ui.Text(c, "时间范围").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
			ui.Select(c, &u.timeKind, []string{"不限", "近一周", "近一个月", "自定义"})
		})
		if u.timeKind == "自定义" {
			ui.Column(c).Width(150).Gap(4).Children(func() {
				ui.Text(c, "从").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
				ui.DateInput(c, &u.from)
			})
			ui.Column(c).Width(150).Gap(4).Children(func() {
				ui.Text(c, "至").FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
				ui.DateInput(c, &u.to)
			})
		}
		ui.Spacer(c)
		ui.Collapsible(c, "检索详情", &u.showDetails, func() {
			u.keywordDetails(c)
		})
		if u.runActive() {
			ui.Button(c, "停止").OnClick(u.stopSearch)
		} else {
			ui.PrimaryButton(c, "搜索").OnClick(u.startKeywordSearch)
		}
	})
	if u.searchError != "" {
		ui.Text(c, u.searchError).FontSize(u.fs(11)).TextColor(t.Danger)
	}
}

func (u *UI) keywordEditor(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(5).Padding(5, 8).Radius(8).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		if len(u.keywordTokens) > 0 {
			ui.Row(c).Gap(5).Wrap().Children(func() {
				for index, token := range u.keywordTokens {
					i := index
					value := token
					ui.Row(c).Gap(3).Padding(2, 7).Radius(6).Background(t.Selection).
						AlignItems(ui.Center).Children(func() {
						ui.Text(c, value).FontSize(u.fs(12))
						ui.Button(c, "×").Label("移除 "+value).Tooltip("移除 "+value).
							FontSize(u.fs(11)).Padding(0, 2).Radius(4).OnClick(func() {
							u.keywordTokens = append(u.keywordTokens[:i], u.keywordTokens[i+1:]...)
						})
					})
				}
			})
		}
		input := ui.TextInput(c, &u.keywordDraft).Placeholder("输入关键词后按空格，例如 Go 实习").
			Label("关键词").BorderWidth(0).Background(ui.Transparent)
		if input.Submitted() {
			u.addKeywordTokens([]string{u.keywordDraft})
			u.keywordDraft = ""
			c.Invalidate()
		} else if input.Changed() {
			if u.splitKeywordDraft() {
				c.Invalidate()
			}
		}
		input.OnTextKey(func(k ui.TextKey) bool {
			if k.Key == ui.KeyBackspace && k.Mods == 0 && k.Text() == "" && len(u.keywordTokens) > 0 {
				u.keywordTokens = u.keywordTokens[:len(u.keywordTokens)-1]
				c.Invalidate()
				return true
			}
			return false
		})
	})
}

// splitKeywordDraft turns words separated by spaces into removable tokens. The
// partial word after the last space stays in the field.
func (u *UI) splitKeywordDraft() bool {
	if !strings.ContainsAny(u.keywordDraft, " \t\n") {
		return false
	}
	fields := strings.Fields(u.keywordDraft)
	trailing := ""
	if len(u.keywordDraft) > 0 {
		last := u.keywordDraft[len(u.keywordDraft)-1]
		if last != ' ' && last != '\t' && last != '\n' && len(fields) > 0 {
			trailing = fields[len(fields)-1]
			fields = fields[:len(fields)-1]
		}
	}
	u.addKeywordTokens(fields)
	u.keywordDraft = trailing
	return true
}

func (u *UI) addKeywordTokens(values []string) {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		duplicate := false
		for _, existing := range u.keywordTokens {
			if strings.EqualFold(existing, value) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			u.keywordTokens = append(u.keywordTokens, value)
		}
	}
}

func (u *UI) keywordDetails(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(12).Wrap().Children(func() {
		u.field(c, "范围", 190, func() {
			ui.Select(c, &u.scopeLabel, u.scopeLabels())
		})
		u.field(c, "每版搜索条数", 130, func() {
			ui.NumberInput(c, &u.perBoard, 1, 500, 1)
		})
		u.field(c, "总共返回条数", 130, func() {
			ui.NumberInput(c, &u.total, 1, 2000, 1)
		})
		u.field(c, "每帖楼层页数", 130, func() {
			ui.NumberInput(c, &u.threadPages, 1, 100, 1)
		})
		u.field(c, "并发数", 110, func() {
			ui.NumberInput(c, &u.concurrency, 1, 16, 1)
		})
	})
	ui.Text(c, "范围在提交时冻结为稳定节点；未达到条数会自动续页，置顶与重复主题不占名额。").
		FontSize(u.fs(10)).TextColor(t.TextMuted)
}

func (u *UI) field(c *ui.Context, label string, width float32, fn func()) {
	t := c.Theme()
	ui.Column(c).Width(width).Gap(4).Children(func() {
		ui.Text(c, label).FontSize(u.fs(11)).Bold().TextColor(t.TextMuted)
		fn()
	})
}

func (u *UI) startKeywordSearch() {
	if strings.TrimSpace(u.keywordDraft) != "" {
		u.addKeywordTokens([]string{u.keywordDraft})
		u.keywordDraft = ""
	}
	if len(u.keywordTokens) == 0 && strings.TrimSpace(u.author) == "" {
		u.searchError = "至少填写一个关键词或作者 ID"
		return
	}
	if u.timeKind == "自定义" && !u.from.IsZero() && !u.to.IsZero() && u.from.After(u.to) {
		u.searchError = "自定义时间范围的开始日期不能晚于结束日期"
		return
	}
	query := domain.SearchQuery{
		Keywords:    append([]string(nil), u.keywordTokens...),
		Author:      strings.TrimSpace(u.author),
		Time:        u.keywordTimeRange(),
		ScopeNode:   u.scopeNodeFor(u.scopeLabel),
		PerBoard:    int(u.perBoard),
		Total:       int(u.total),
		ThreadPages: int(u.threadPages),
		Concurrency: int(u.concurrency),
	}
	id, err := u.app.StartSearch(query)
	if err != nil {
		u.searchError = err.Error()
		return
	}
	u.searchError = ""
	u.saveSearchView()
	u.resultScroll = ui.ScrollState{}
	u.collapsed = make(map[string]bool)
	u.activeRun = id
	u.activeHistory = id
	u.keywordHistory = id
	u.sourceKey = nil
	u.run, _ = u.app.Run(id)
}

func (u *UI) stopSearch() {
	if u.activeRun != "" {
		u.app.StopSearch(u.activeRun)
	}
}

func (u *UI) runActive() bool {
	return u.run.Status == svc.StatusQueued || u.run.Status == svc.StatusRunning
}

func (u *UI) keywordTimeRange() domain.TimeRange {
	switch u.timeKind {
	case "近一周":
		return domain.TimeRange{Kind: domain.TimeWeek}
	case "近一个月":
		return domain.TimeRange{Kind: domain.TimeMonth}
	case "自定义":
		from, to := u.from, u.to
		if from.IsZero() {
			from = time.Now().AddDate(0, 0, -7)
		}
		if to.IsZero() {
			to = time.Now()
		}
		to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 0, to.Location())
		return domain.TimeRange{Kind: domain.TimeCustom, From: from, To: to}
	default:
		return domain.TimeRange{Kind: domain.TimeAll}
	}
}

func (u *UI) keywordResults(c *ui.Context) {
	t := c.Theme()
	if u.activeRun == "" {
		ui.Column(c).Grow(1).Fill().Center().Gap(10).Children(func() {
			ui.Icon(c, iconSearch).FontSize(u.fs(30)).TextColor(t.TextMuted)
			ui.Text(c, "发起一次关键词检索").FontSize(u.fs(17)).Bold()
			ui.Text(c, "输入关键词或作者 ID，查找你关心的讨论。").FontSize(u.fs(14)).TextColor(t.TextMuted)
		})
		return
	}
	run := u.run
	ui.Scroll(c).TrackScroll(&u.resultScroll).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(920).Margin(0, ui.Auto).Padding(20, 24).Gap(12).Children(func() {
			ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
				if runActive(run) {
					ui.Spinner(c)
				}
				ui.Text(c, statusText(run)).FontSize(u.fs(12)).Bold()
				if run.ArticleCount > 0 {
					ui.Badge(c, fmt.Sprintf("%d 主题", run.ArticleCount))
					ui.Badge(c, fmt.Sprintf("%d 回复", run.ReplyCount))
				}
				ui.Spacer(c)
			})
			if run.Error != "" {
				ui.Text(c, run.Error).FontSize(u.fs(11)).TextColor(t.Danger)
			}
			if run.Status == svc.StatusCompleted && run.ArticleCount == 0 {
				ui.Column(c).Padding(28).Center().Gap(6).Children(func() {
					ui.Text(c, "本轮没有符合条件的主题").Bold()
					ui.Text(c, "试试减少关键词，或扩大时间与版面范围。").FontSize(u.fs(12)).TextColor(t.TextMuted)
				})
				return
			}
			tree := buildResultTree(run.Boards)
			for _, node := range tree.Order {
				u.resultNode(c, node)
			}
			if run.Error == "" && run.ArticleCount == 0 && !runActive(run) {
				ui.Text(c, "没有读到符合条件的主题。").FontSize(u.fs(12)).TextColor(t.TextMuted)
			}
		})
	})
}

func runActive(run svc.RunSnapshot) bool {
	return run.Status == svc.StatusQueued || run.Status == svc.StatusRunning
}

type resultNode struct {
	ID       string
	Label    string
	Kind     forum.NodeKind
	Board    *svc.BoardSnapshotResult
	Children []*resultNode
	Order    []*resultNode
	byID     map[string]*resultNode
	Articles int
	Replies  int
}

func buildResultTree(boards []svc.BoardSnapshotResult) *resultNode {
	root := &resultNode{byID: make(map[string]*resultNode)}
	add := func(parent *resultNode, child *resultNode) *resultNode {
		if existing, ok := parent.byID[child.ID]; ok {
			return existing
		}
		if parent.byID == nil {
			parent.byID = make(map[string]*resultNode)
		}
		parent.byID[child.ID] = child
		parent.Children = append(parent.Children, child)
		parent.Order = append(parent.Order, child)
		return child
	}
	for i := range boards {
		board := boards[i]
		if len(board.Articles) == 0 && board.Error == "" {
			continue
		}
		current := root
		containers := board.Path
		if len(containers) > 0 {
			containers = containers[:len(containers)-1]
		}
		for _, node := range containers {
			current = add(current, &resultNode{
				ID:    string(node.ID),
				Label: node.Name,
				Kind:  node.Kind,
				byID:  make(map[string]*resultNode),
			})
		}
		label := board.Name
		if label == "" {
			label = string(board.BoardID)
		}
		leaf := add(current, &resultNode{
			ID:    "board:" + string(board.BoardID),
			Label: label,
			Kind:  forum.KindBoard,
			Board: &boards[i],
		})
		replies := 0
		for _, item := range board.Articles {
			replies += len(item.Replies)
		}
		leaf.Articles = len(board.Articles)
		leaf.Replies = replies
	}
	aggregate(root)
	return root
}

func aggregate(node *resultNode) (int, int) {
	articles, replies := node.Articles, node.Replies
	for _, child := range node.Order {
		childArticles, childReplies := aggregate(child)
		articles += childArticles
		replies += childReplies
	}
	node.Articles = articles
	node.Replies = replies
	return articles, replies
}

func (u *UI) resultNode(c *ui.Context, node *resultNode) {
	t := c.Theme()
	key := "group:" + node.ID
	open := !u.collapsed[key]
	label := fmt.Sprintf("%s  ·  %d 主题 · %d 回复", node.Label, node.Articles, node.Replies)
	if node.Kind == forum.KindBoard {
		label = node.Label
		if node.Articles > 0 {
			label = fmt.Sprintf("%s  ·  %d 主题 · %d 回复", node.Label, node.Articles, node.Replies)
		}
	}
	ui.Collapsible(c, label, &open, func() {
		if node.Board != nil {
			if node.Board.Error != "" {
				ui.Text(c, "该版面读取失败："+node.Board.Error).FontSize(u.fs(11)).TextColor(t.Danger).Padding(4, 10)
			}
			for _, item := range node.Board.Articles {
				u.articleResult(c, item)
			}
			return
		}
		for _, child := range node.Order {
			u.resultNode(c, child)
		}
	})
	u.collapsed[key] = !open
}

func (u *UI) articleResult(c *ui.Context, item svc.ArticleSnapshot) {
	t := c.Theme()
	article := item.Article
	row := ui.Row(c).Gap(8).Padding(7, 10).Radius(6).Cursor(ui.CursorPointer).
		OnClick(func() { u.openSource(article.Key, 0) })
	row = u.interactive(c, row, false, ui.Transparent)
	row.Key(item.Article.Key).Transition(ui.ElementTransition{Colors: true, Duration: 140 * time.Millisecond, Enter: &ui.Motion{}})
	row.Children(func() {
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Text(c, article.Title).FontSize(u.fs(13)).Bold().SingleLine()
			ui.Text(c, fmt.Sprintf("%s · %s · %d 回复 · 读取 %d 页回复",
				article.Author, relativeTime(article.CreatedAt), article.ReplyCount, item.ThreadPages)).
				FontSize(u.fs(10)).TextColor(t.TextMuted).SingleLine()
		})
		ui.Icon(c, iconChevron).FontSize(u.fs(13)).TextColor(t.TextMuted)
	})
	if len(item.Replies) == 0 {
		return
	}
	replyKey := "replies:" + string(article.Key.ArticleID)
	open := !u.collapsed[replyKey]
	ui.Collapsible(c, fmt.Sprintf("回复 · 已加载 %d 条", len(item.Replies)), &open, func() {
		for _, reply := range item.Replies {
			row := ui.Row(c).Gap(8).Padding(5, 12).AlignItems(ui.Start).Cursor(ui.CursorPointer).
				OnClick(func() { u.openSource(article.Key, reply.Floor) })
			row = u.interactive(c, row, false, ui.Transparent)
			row.Key(reply.Key).Transition(ui.ElementTransition{Colors: true, Duration: 140 * time.Millisecond, Enter: &ui.Motion{}})
			row.Children(func() {
				ui.Badge(c, fmt.Sprintf("#%d", reply.Floor))
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, reply.Author+" · "+relativeTime(reply.CreatedAt)).FontSize(u.fs(10)).TextColor(t.TextMuted)
					ui.Text(c, reply.Content).FontSize(u.fs(12))
				})
			})
		}
		if !item.RepliesEnd {
			ui.Text(c, fmt.Sprintf("仅显示前 %d 页回复，打开来源可继续读取。", item.ThreadPages)).
				FontSize(u.fs(10)).TextColor(t.TextMuted).Padding(4, 12)
		}
	})
	u.collapsed[replyKey] = !open
}

func (u *UI) aiChat(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Key("ai-conversation").TrackScroll(&u.aiScroll).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(848).Margin(0, ui.Auto).Padding(24).Gap(28).Children(func() {
			if len(u.chat) == 0 {
				ui.Column(c).Padding(12, 2).Gap(8).Children(func() {
					ui.Icon(c, iconSparkles).FontSize(u.fs(28)).TextColor(t.Accent)
					ui.Text(c, "你想了解什么？").FontSize(u.fs(24)).Bold()
					ui.Text(c, "查找论坛里的经验、观点与最新讨论，让回答附上来源。").
						FontSize(u.fs(14)).TextColor(t.TextMuted)
					ui.Text(c, "示例：计算机相关版面最近一周有哪些 Go 实习帖？请对比并给出链接。").
						FontSize(u.fs(11)).TextColor(t.TextMuted)
					if !u.aiKeySaved {
						ui.Text(c, "尚未配置 AI API：请到 设置 → AI 配置 填写 API Key。").
							FontSize(u.fs(11)).TextColor(t.Warning)
					}
				})
				return
			}
			for index := range u.chat {
				u.aiTurn(c, &u.chat[index])
			}
		})
	})
	u.aiComposer(c)
}

func (u *UI) aiTurn(c *ui.Context, turn *chatTurn) {
	t := c.Theme()
	if turn.Role == "user" {
		ui.Row(c).FillWidth().Justify(ui.End).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Key(turn.ID).MaxWidth(600).Padding(12, 16).Radius(12).Background(t.Selection).
				Children(func() {
					ui.Text(c, turn.Text).FontSize(u.fs(16)).Selectable().LineHeight(1.65)
				})
		})
		return
	}
	if turn.Groups == nil {
		turn.Groups = make(map[string]bool)
	}
	ui.Row(c).FillWidth().Justify(ui.Start).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Key(turn.ID).Grow(1).FillWidth().Gap(12).Children(func() {
			provider := turn.Provider
			if provider == "" {
				provider = u.aiProvider
			}
			model := turn.Model
			if model == "" {
				model = u.aiModel
			}
			ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconSparkles).FontSize(u.fs(14)).TextColor(t.Accent)
				ui.Text(c, "AI").FontSize(u.fs(12)).Bold()
				ui.Badge(c, provider)
				ui.Text(c, model).FontSize(u.fs(10)).TextColor(t.TextMuted)
				if turn.Running {
					ui.Spinner(c)
				}
				if turn.Running && u.aiStatus != "" {
					ui.Text(c, u.aiStatus).FontSize(u.fs(10)).TextColor(t.TextMuted)
				}
			})
			if turn.Reasoning != "" {
				ui.Collapsible(c, "思考过程", &turn.ReasoningOpen, func() {
					ui.Text(c, turn.Reasoning).FontSize(u.fs(11)).TextColor(t.TextMuted).
						LineHeight(1.55).Selectable()
				})
			}
			if len(turn.Tools) > 0 {
				ui.Collapsible(c, fmt.Sprintf("工具调用（%d）", len(turn.Tools)), &turn.ToolsOpen, func() {
					for _, tool := range turn.Tools {
						u.toolTraceView(c, tool)
					}
				})
				if !turn.ToolsOpen {
					last := turn.Tools[len(turn.Tools)-1]
					if last.Summary != "" {
						ui.Text(c, last.Summary).FontSize(u.fs(12)).TextColor(t.TextMuted).MaxLines(2)
					}
				}
			} else if len(turn.Trace) > 0 {
				ui.Column(c).Padding(10, 12).Gap(4).Radius(8).Background(t.Surface).Border(1, t.Border).Children(func() {
					ui.Text(c, "工具调用").Bold().FontSize(u.fs(12))
					for _, line := range turn.Trace {
						ui.Text(c, "• "+line).FontSize(u.fs(11)).TextColor(t.TextMuted)
					}
				})
			}
			if turn.Text != "" {
				previousTheme := c.Theme()
				bodyTheme := *previousTheme
				bodyTheme.FontSize = u.fs(16)
				c.SetTheme(&bodyTheme)
				if turn.Running {
					markdown.Stream(c, turn.Text, &u.md)
				} else {
					markdown.View(c, turn.Text, &u.md)
				}
				c.SetTheme(previousTheme)
			} else if turn.Running {
				ui.Text(c, "正在思考…").FontSize(u.fs(12)).TextColor(t.TextMuted)
			}
			if turn.Err != "" {
				ui.Text(c, "请求失败："+turn.Err).FontSize(u.fs(11)).TextColor(t.Danger)
			}
			if len(turn.Citations) > 0 {
				ui.Collapsible(c, fmt.Sprintf("引用主题（%d）", len(turn.Citations)), &turn.CiteOpen, func() {
					u.citationGroups(c, turn)
				})
			} else if len(turn.Sources) > 0 {
				ui.Column(c).Gap(5).Children(func() {
					ui.Text(c, "参考来源").Bold().FontSize(u.fs(12))
					for index, source := range turn.Sources {
						if index >= 8 {
							break
						}
						link := source
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, truncate(link, 92)).FontSize(u.fs(11)).TextColor(t.Accent).Grow(1).SingleLine()
							ui.Button(c, "应用内打开").FontSize(u.fs(10)).OnClick(func() { u.openLoggedInBrowser(c, link) })
							ui.Button(c, "复制").FontSize(u.fs(10)).OnClick(func() {
								c.WriteClipboard(link)
								c.Toast("链接已复制")
							})
						})
					}
				})
			}
		})
	})
}

func (u *UI) toolTraceView(c *ui.Context, tool toolTrace) {
	t := c.Theme()
	ui.Column(c).Padding(9, 11).Gap(4).Radius(7).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
			ui.Text(c, toolLabel(tool.Name)).FontSize(u.fs(12)).Bold()
			if tool.Running {
				ui.Spinner(c)
			}
			if tool.Summary != "" {
				ui.Text(c, tool.Summary).FontSize(u.fs(11)).TextColor(t.TextMuted)
			}
			if tool.Error != "" {
				ui.Text(c, "失败："+truncate(tool.Error, 120)).FontSize(u.fs(10)).TextColor(t.Danger)
			}
		})
		if tool.Arguments != "" {
			ui.Text(c, "参数").FontSize(u.fs(10)).TextColor(t.TextMuted)
			ui.Text(c, prettyToolArguments(tool.Arguments)).FontSize(u.fs(10)).
				TextColor(t.TextMuted).MaxLines(8).Selectable()
		}
	})
}

func (u *UI) citationGroups(c *ui.Context, turn *chatTurn) {
	t := c.Theme()
	grouped := make(map[string][]svc.Citation)
	order := make([]string, 0)
	for _, citation := range turn.Citations {
		board := citation.Board
		if board == "" {
			board = "未分类版面"
		}
		if _, exists := grouped[board]; !exists {
			order = append(order, board)
		}
		grouped[board] = append(grouped[board], citation)
	}
	for _, board := range order {
		group := grouped[board]
		open := turn.Groups[board]
		ui.Collapsible(c, fmt.Sprintf("%s（%d）", board, len(group)), &open, func() {
			for index, citation := range group {
				if index >= 8 {
					ui.Textf(c, "还有 %d 条引用未显示", len(group)-8).FontSize(u.fs(10)).TextColor(t.TextMuted)
					break
				}
				item := citation
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).Gap(1).Children(func() {
						ui.Text(c, truncate(item.Title, 46)).FontSize(u.fs(11)).SingleLine().Tooltip(item.Title)
						ui.Text(c, truncate(item.URL, 88)).FontSize(u.fs(9)).TextColor(t.TextMuted).SingleLine()
					})
					ui.Button(c, "应用内打开").FontSize(u.fs(10)).OnClick(func() { u.openLoggedInBrowser(c, item.URL) })
					ui.Button(c, "复制").FontSize(u.fs(10)).OnClick(func() {
						c.WriteClipboard(item.URL)
						c.Toast("链接已复制")
					})
				})
			}
		})
		turn.Groups[board] = open
	}
}

func prettyToolArguments(arguments string) string {
	var buffer bytes.Buffer
	if err := json.Indent(&buffer, []byte(arguments), "", "  "); err == nil {
		return buffer.String()
	}
	return arguments
}

func (u *UI) aiComposer(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).FillWidth().MaxWidth(848).Margin(0, ui.Auto).Padding(12, 24, 20, 24).Children(func() {
		ui.Column(c).FillWidth().Padding(16).Gap(12).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.TextArea(c, &u.aiQuestion).Placeholder("输入问题，Enter 发送，Shift+Enter 换行").Lines(1, 5).Label("AI 问题").
				FontSize(u.fs(16)).Border(0, ui.Transparent).OnTextKey(func(k ui.TextKey) bool {
				if k.Key == ui.KeyEnter && k.Mods == 0 {
					if !u.aiRunning {
						u.aiQuestion = k.Text()
						u.startAI()
						k.SetText("")
					}
					return true
				}
				return false
			})
			ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "时间范围").FontSize(u.fs(12)).TextColor(t.TextMuted)
				ui.Select(c, &u.aiTimeKind, []string{"近一周", "近一个月", "不限", "自定义"}).Width(148).Label("时间范围")
				ui.Spacer(c)
				if u.aiRunning {
					ui.Button(c, "停止").OnClick(u.stopAI)
				} else {
					ui.PrimaryButton(c, "发送").OnClick(u.startAI)
				}
			})
			if u.aiTimeKind == "自定义" {
				ui.Row(c).Gap(12).Wrap().Children(func() {
					u.field(c, "从", 150, func() { ui.DateInput(c, &u.aiFrom) })
					u.field(c, "至", 150, func() { ui.DateInput(c, &u.aiTo) })
				})
			}
			ui.Collapsible(c, "检索详情", &u.aiDetails, func() {
				ui.Row(c).Gap(12).Wrap().Children(func() {
					u.field(c, "范围", 200, func() {
						ui.Select(c, &u.aiScope, u.scopeLabels())
					})
					u.field(c, "全局最多帖子", 140, func() {
						ui.NumberInput(c, &u.aiTotal, 1, 500, 1)
					})
					u.field(c, "单板面最多帖子", 150, func() {
						ui.NumberInput(c, &u.aiPerBoard, 1, 100, 1)
					})
				})
			})
			if u.aiRunning && u.aiStatus != "" {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c)
					ui.Text(c, u.aiStatus).FontSize(u.fs(12)).TextColor(t.TextMuted)
				})
			}
		})
	})
}

func (u *UI) startAI() {
	question := strings.TrimSpace(u.aiQuestion)
	if question == "" || u.app == nil {
		return
	}
	application := u.app
	u.stopAIContext()
	if turn := u.chatTurnByID(u.activeChat); turn != nil {
		turn.Running = false
	}
	u.saveTranscript()
	u.aiRunToken++
	token := u.aiRunToken
	u.aiRunning = true
	u.aiAnswer = ""
	u.aiError = ""
	u.aiTrace = nil
	u.aiSources = nil
	u.aiStatus = ""
	u.aiQuestion = ""

	u.chatSeq++
	userID := u.chatSeq
	u.chat = append(u.chat, chatTurn{ID: userID, Role: "user", Text: question})
	u.chatSeq++
	assistantID := u.chatSeq
	u.chat = append(u.chat, chatTurn{
		ID: assistantID, Role: "assistant", Provider: u.aiProvider, Model: u.aiModel,
		Running: true, CiteOpen: false, Groups: make(map[string]bool),
	})
	u.activeChat = assistantID
	u.aiScroll.Y = math.MaxFloat32

	query := domain.SearchQuery{
		Time:        u.aiTimeRange(),
		ScopeNode:   u.scopeNodeFor(u.aiScope),
		PerBoard:    int(u.aiPerBoard),
		Total:       int(u.aiTotal),
		ThreadPages: domain.DefaultThreadPages,
		Concurrency: domain.DefaultConcurrency,
	}
	if u.aiHistoryID == "" {
		u.saveSearchView()
		u.aiHistoryID = application.AddAIHistory(question, query)
	}
	u.activeHistory = u.aiHistoryID

	config, configured := u.aiConfig()
	if !configured {
		u.aiRunning = false
		if turn := u.chatTurnByID(assistantID); turn != nil {
			turn.Text = "尚未配置 AI API。\n\n请打开“设置 → AI 配置”，选择 DeepSeek 或 Kimi 并填写 API Key，然后回到这里重新提问。"
			turn.Running = false
		}
		u.saveTranscript()
		return
	}
	u.aiStatus = "正在连接模型…"
	ctx, cancel := context.WithCancel(context.Background())
	u.aiCancel = cancel
	options := svc.ChatOptions{
		Scope: u.aiScope, Time: u.aiTimeKind,
		Total: int(u.aiTotal), PerBoard: int(u.aiPerBoard),
	}
	go func() {
		answer, err := application.Chat(ctx, config, question, options,
			func(delta string) error {
				if win := u.window(); win != nil {
					win.Update(func() {
						if u.aiRunToken != token {
							return
						}
						follow := u.aiScroll.MaxY == 0 || u.aiScroll.Y >= u.aiScroll.MaxY-24
						u.aiAnswer += delta
						if turn := u.chatTurnByID(assistantID); turn != nil {
							turn.Text += delta
						}
						if follow {
							u.aiScroll.Y = math.MaxFloat32
						}
					})
				}
				return nil
			},
			func(reasoning string) error {
				if win := u.window(); win != nil {
					win.Update(func() {
						if u.aiRunToken != token {
							return
						}
						if turn := u.chatTurnByID(assistantID); turn != nil {
							turn.Reasoning += reasoning
						}
					})
				}
				return nil
			},
			func(event svc.ToolEvent) {
				if win := u.window(); win != nil {
					win.Update(func() {
						if u.aiRunToken != token {
							return
						}
						u.applyToolEvent(event)
					})
				}
			},
		)
		if win := u.window(); win != nil {
			win.Update(func() {
				if u.aiRunToken != token {
					return
				}
				u.aiRunning = false
				u.aiCancel = nil
				u.aiStatus = ""
				if answer != "" {
					u.aiAnswer = answer
				}
				follow := u.aiScroll.MaxY == 0 || u.aiScroll.Y >= u.aiScroll.MaxY-24
				if turn := u.chatTurnByID(assistantID); turn != nil {
					if answer != "" {
						turn.Text = answer
					}
					turn.Running = false
					if err != nil && !errors.Is(err, context.Canceled) {
						turn.Err = err.Error()
					}
				}
				if err != nil && !errors.Is(err, context.Canceled) {
					u.aiError = err.Error()
				}
				if follow {
					u.aiScroll.Y = math.MaxFloat32
				}
				u.saveTranscript()
			})
		}
	}()
}

func (u *UI) chatTurnByID(id int) *chatTurn {
	for index := range u.chat {
		if u.chat[index].ID == id {
			return &u.chat[index]
		}
	}
	return nil
}

func (u *UI) transcriptMarkdown() string {
	var builder strings.Builder
	for _, turn := range u.chat {
		if turn.Role == "user" {
			builder.WriteString("**你：** ")
			builder.WriteString(turn.Text)
			builder.WriteString("\n\n")
			continue
		}
		builder.WriteString("**AI：**\n\n")
		if turn.Text != "" {
			builder.WriteString(turn.Text)
		}
		if turn.Err != "" {
			builder.WriteString("\n\n请求失败：")
			builder.WriteString(turn.Err)
		}
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String())
}

func (u *UI) saveTranscript() {
	if u.app == nil || u.aiHistoryID == "" {
		return
	}
	u.app.SetAIAnswer(u.aiHistoryID, u.transcriptMarkdown())
}

func (u *UI) applyToolEvent(event svc.ToolEvent) {
	turn := u.chatTurnByID(u.activeChat)
	if turn != nil && turn.Groups == nil {
		turn.Groups = make(map[string]bool)
	}
	switch event.Phase {
	case "started":
		u.aiStatus = "正在调用工具：" + toolLabel(event.Name) + "…"
		if turn != nil {
			turn.Tools = append(turn.Tools, toolTrace{
				ID: event.CallID, Name: event.Name, Arguments: event.Arguments, Running: true,
			})
		}
	case "completed", "failed":
		u.aiStatus = ""
		line := "工具 · " + toolLabel(event.Name)
		if event.Summary != "" {
			line += "：" + event.Summary
		}
		if event.Error != "" {
			line += "（失败：" + truncate(event.Error, 120) + "）"
		}
		u.aiTrace = append(u.aiTrace, line)
		if len(u.aiTrace) > 8 {
			u.aiTrace = u.aiTrace[len(u.aiTrace)-8:]
		}
		if turn != nil {
			trace := (*toolTrace)(nil)
			for index := range turn.Tools {
				if event.CallID != "" && turn.Tools[index].ID == event.CallID {
					trace = &turn.Tools[index]
					break
				}
			}
			if trace == nil && len(turn.Tools) > 0 {
				trace = &turn.Tools[len(turn.Tools)-1]
			}
			if trace == nil {
				turn.Tools = append(turn.Tools, toolTrace{ID: event.CallID, Name: event.Name})
				trace = &turn.Tools[len(turn.Tools)-1]
			}
			trace.Name = event.Name
			trace.Arguments = event.Arguments
			trace.Summary = event.Summary
			trace.Error = event.Error
			trace.Running = false
			trace.Citations = append([]svc.Citation(nil), event.Citations...)
			turn.Trace = append([]string(nil), u.aiTrace...)
			for _, citation := range event.Citations {
				duplicate := false
				for _, existing := range turn.Citations {
					if existing.URL == citation.URL {
						duplicate = true
						break
					}
				}
				if !duplicate && citation.URL != "" {
					turn.Citations = append(turn.Citations, citation)
				}
			}
		}
		for _, source := range event.Sources {
			if source == "" {
				continue
			}
			duplicate := false
			for _, existing := range u.aiSources {
				if existing == source {
					duplicate = true
					break
				}
			}
			if !duplicate {
				u.aiSources = append(u.aiSources, source)
			}
		}
		if len(u.aiSources) > 20 {
			u.aiSources = u.aiSources[:20]
		}
		if turn != nil {
			turn.Sources = append([]string(nil), u.aiSources...)
		}
	}
}

func toolLabel(name string) string {
	switch name {
	case "search_forum":
		return "搜索论坛"
	case "read_thread":
		return "读取主题"
	case "list_boards":
		return "列出版面"
	case "get_traffic":
		return "读取流量"
	case "latest_topic":
		return "读取最新帖"
	default:
		return name
	}
}

func (u *UI) aiTimeRange() domain.TimeRange {
	switch u.aiTimeKind {
	case "近一周":
		return domain.TimeRange{Kind: domain.TimeWeek}
	case "近一个月":
		return domain.TimeRange{Kind: domain.TimeMonth}
	case "自定义":
		from, to := u.aiFrom, u.aiTo
		if from.IsZero() {
			from = time.Now().AddDate(0, -1, 0)
		}
		if to.IsZero() {
			to = time.Now()
		}
		to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 0, to.Location())
		return domain.TimeRange{Kind: domain.TimeCustom, From: from, To: to}
	default:
		return domain.TimeRange{Kind: domain.TimeAll}
	}
}

func (u *UI) stopAI() {
	u.stopAIContext()
	u.aiRunToken++
	u.aiRunning = false
	u.aiStatus = ""
	if turn := u.chatTurnByID(u.activeChat); turn != nil {
		turn.Running = false
	}
	u.saveTranscript()
}

func (u *UI) stopAIContext() {
	if u.aiCancel != nil {
		u.aiCancel()
		u.aiCancel = nil
	}
}
