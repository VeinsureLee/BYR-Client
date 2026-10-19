// Package demo provides deterministic, in-process forum data for the first
// runnable version. It implements the same source port as the future BYR HTTP
// adapter, so the application services and UI do not depend on demo details.
package demo

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"sync"
	"time"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
)

const siteID = "byr"

type Source struct {
	mu      sync.RWMutex
	catalog domain.Catalog
	boards  map[domain.BoardID][]domain.Article
	replies map[domain.ArticleKey][]domain.Reply
	now     time.Time
}

func New(now time.Time) *Source {
	s := &Source{
		boards:  make(map[domain.BoardID][]domain.Article),
		replies: make(map[domain.ArticleKey][]domain.Reply),
		now:     now.Truncate(time.Minute),
	}
	s.catalog = buildCatalog(s.now)
	for _, node := range s.catalog.Nodes {
		if node.Kind != forum.KindBoard {
			continue
		}
		s.seedBoard(node.BoardID, node.Name)
	}
	return s
}

func (s *Source) ReadCatalog(context.Context) (domain.Catalog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.catalog
	c.Roots = append([]forum.NodeID(nil), c.Roots...)
	nodes := c.Nodes
	c.Nodes = make([]forum.Node, len(nodes))
	for i, node := range nodes {
		node.Children = append([]forum.NodeID(nil), node.Children...)
		c.Nodes[i] = node
	}
	c.FetchedAt = s.now
	return c, nil
}

func (s *Source) ReadBoardPage(_ context.Context, _ domain.Scope, board domain.BoardID, cursor domain.Cursor, limit int) (domain.BoardPage, error) {
	if limit <= 0 {
		limit = domain.DefaultBoardBatchSize
	}
	offset, err := parseCursor(cursor)
	if err != nil {
		return domain.BoardPage{}, err
	}
	s.mu.RLock()
	items := s.boards[board]
	s.mu.RUnlock()
	if items == nil {
		return domain.BoardPage{}, fmt.Errorf("board %q not found", board)
	}
	page, next, end := slicePage(items, offset, limit)
	return domain.BoardPage{
		BoardID:    board,
		Articles:   page,
		Next:       next,
		End:        end,
		ObservedAt: s.now,
	}, nil
}

func (s *Source) SearchBoardPage(_ context.Context, _ domain.Scope, board domain.BoardID, query domain.BoardQuery, cursor domain.Cursor, limit int) (domain.BoardPage, error) {
	if limit <= 0 {
		limit = domain.DefaultSearchCandidates
	}
	offset, err := parseCursor(cursor)
	if err != nil {
		return domain.BoardPage{}, err
	}
	s.mu.RLock()
	all := s.boards[board]
	s.mu.RUnlock()
	if all == nil {
		return domain.BoardPage{}, fmt.Errorf("board %q not found", board)
	}
	candidates := make([]domain.Article, 0, len(all))
	for _, a := range all {
		if query.Author != "" && !strings.EqualFold(a.Author, query.Author) {
			continue
		}
		if len(query.Keywords) > 0 && !containsAny(a.Title, query.Keywords) {
			continue
		}
		candidates = append(candidates, a)
	}
	page, next, end := slicePage(candidates, offset, limit)
	return domain.BoardPage{
		BoardID:    board,
		Articles:   page,
		Next:       next,
		End:        end,
		ObservedAt: s.now,
	}, nil
}

func (s *Source) ReadThreadPage(_ context.Context, _ domain.Scope, key domain.ArticleKey, cursor domain.Cursor, limit int) (domain.ThreadPage, error) {
	if limit <= 0 {
		limit = domain.DefaultRepliesPerPage
	}
	offset, err := parseCursor(cursor)
	if err != nil {
		return domain.ThreadPage{}, err
	}
	s.mu.RLock()
	var article domain.Article
	found := false
	for _, a := range s.boards[key.BoardID] {
		if a.Key.ArticleID == key.ArticleID {
			article, found = a, true
			break
		}
	}
	all := s.replies[key]
	s.mu.RUnlock()
	if !found {
		return domain.ThreadPage{}, fmt.Errorf("article %q not found", key.ArticleID)
	}
	article.BodyLoaded = true
	page, next, end := slicePage(all, offset, limit)
	return domain.ThreadPage{
		Article:    article,
		Replies:    page,
		Next:       next,
		End:        end,
		PagesRead:  1,
		ObservedAt: s.now,
	}, nil
}

func parseCursor(c domain.Cursor) (int, error) {
	if c == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(string(c))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid cursor %q", c)
	}
	return n, nil
}

func slicePage[T any](items []T, offset, limit int) ([]T, domain.Cursor, bool) {
	if offset >= len(items) {
		return []T{}, "", true
	}
	end := min(offset+limit, len(items))
	page := append([]T(nil), items[offset:end]...)
	if end >= len(items) {
		return page, "", true
	}
	return page, domain.Cursor(strconv.Itoa(end)), false
}

func containsAny(s string, words []string) bool {
	lower := strings.ToLower(s)
	for _, word := range words {
		if strings.Contains(lower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func buildCatalog(now time.Time) domain.Catalog {
	nodes := []forum.Node{
		{ID: "f-campus", Kind: forum.KindForum, Name: "校园生活", Children: []forum.NodeID{"b-campus", "b-lost", "b-canteen"}},
		{ID: "b-campus", Kind: forum.KindBoard, Name: "校园活动", BoardID: "b-campus"},
		{ID: "b-lost", Kind: forum.KindBoard, Name: "失物招领", BoardID: "b-lost"},
		{ID: "b-canteen", Kind: forum.KindBoard, Name: "食堂", BoardID: "b-canteen"},

		{ID: "f-study", Kind: forum.KindForum, Name: "学术交流", Children: []forum.NodeID{"b-math", "s-grad"}},
		{ID: "b-math", Kind: forum.KindBoard, Name: "数学", BoardID: "b-math"},
		{ID: "s-grad", Kind: forum.KindSubForum, Name: "研究生", Children: []forum.NodeID{"b-kaoyan", "b-paper"}},
		{ID: "b-kaoyan", Kind: forum.KindBoard, Name: "考研", BoardID: "b-kaoyan"},
		{ID: "b-paper", Kind: forum.KindBoard, Name: "论文", BoardID: "b-paper"},

		{ID: "f-tech", Kind: forum.KindForum, Name: "技术", Children: []forum.NodeID{"s-prog", "b-ai"}},
		{ID: "s-prog", Kind: forum.KindSubForum, Name: "编程", Children: []forum.NodeID{"b-go", "b-front", "b-linux"}},
		{ID: "b-go", Kind: forum.KindBoard, Name: "Go", BoardID: "b-go"},
		{ID: "b-front", Kind: forum.KindBoard, Name: "前端", BoardID: "b-front"},
		{ID: "b-linux", Kind: forum.KindBoard, Name: "Linux", BoardID: "b-linux"},
		{ID: "b-ai", Kind: forum.KindBoard, Name: "人工智能", BoardID: "b-ai"},

		{ID: "f-job", Kind: forum.KindForum, Name: "就业", Children: []forum.NodeID{"b-intern", "b-autumn", "b-career"}},
		{ID: "b-intern", Kind: forum.KindBoard, Name: "实习", BoardID: "b-intern"},
		{ID: "b-autumn", Kind: forum.KindBoard, Name: "秋招", BoardID: "b-autumn"},
		{ID: "b-career", Kind: forum.KindBoard, Name: "求职", BoardID: "b-career"},

		{ID: "f-life", Kind: forum.KindForum, Name: "休闲", Children: []forum.NodeID{"b-photo", "b-music", "b-game"}},
		{ID: "b-photo", Kind: forum.KindBoard, Name: "摄影", BoardID: "b-photo"},
		{ID: "b-music", Kind: forum.KindBoard, Name: "音乐", BoardID: "b-music"},
		{ID: "b-game", Kind: forum.KindBoard, Name: "游戏", BoardID: "b-game"},

		{ID: "f-sport", Kind: forum.KindForum, Name: "体育", Children: []forum.NodeID{"b-basketball", "b-run"}},
		{ID: "b-basketball", Kind: forum.KindBoard, Name: "篮球", BoardID: "b-basketball"},
		{ID: "b-run", Kind: forum.KindBoard, Name: "跑步", BoardID: "b-run"},

		{ID: "f-market", Kind: forum.KindForum, Name: "交易", Children: []forum.NodeID{"b-secondhand"}},
		{ID: "b-secondhand", Kind: forum.KindBoard, Name: "二手市场", BoardID: "b-secondhand"},

		{ID: "f-mood", Kind: forum.KindForum, Name: "情感", Children: []forum.NodeID{"b-mood"}},
		{ID: "b-mood", Kind: forum.KindBoard, Name: "心情", BoardID: "b-mood"},

		{ID: "f-site", Kind: forum.KindForum, Name: "站务", Children: []forum.NodeID{"b-notice"}},
		{ID: "b-notice", Kind: forum.KindBoard, Name: "公告", BoardID: "b-notice"},

		{ID: "f-alumni", Kind: forum.KindForum, Name: "校友", Children: []forum.NodeID{"b-alumni"}},
		{ID: "b-alumni", Kind: forum.KindBoard, Name: "校友会", BoardID: "b-alumni"},
	}
	roots := []forum.NodeID{"f-campus", "f-study", "f-tech", "f-job", "f-life", "f-sport", "f-market", "f-mood", "f-site", "f-alumni"}
	return domain.Catalog{Roots: roots, Nodes: nodes, FetchedAt: now}
}

var authors = []string{"studentA", "alice", "bob", "coder", "runner", "lily", "max", "north", "turing", "yuki", "byte", "echo"}

var topics = []string{
	"Go 后端实习内推，base 北京",
	"秋招面经：后端开发三面复盘",
	"考研数学复习经验分享",
	"食堂新窗口测评：麻辣香锅",
	"校园活动志愿者招募",
	"摄影：秋天的校园角落",
	"Linux 环境配置踩坑记录",
	"求推荐晚上跑步的路线",
	"二手显示器出售，九成新",
	"心情：最近有点焦虑怎么办",
	"求组队参加周末篮球赛",
	"音乐节志愿者经验分享",
	"游戏本选购求建议",
	"论文写作工具与排版经验",
	"人工智能课程项目交流",
	"前端实习面经整理",
	"校友返校活动报名",
	"站务公告：版面规则更新",
	"失物招领：捡到钥匙一串",
	"求职简历修改建议",
}

var boardTopics = map[domain.BoardID][]string{
	"b-go":         {"Go 并发实践：从 goroutine 到 errgroup", "Go 实习内推，后端方向", "Go 项目复盘：短链接服务"},
	"b-front":      {"前端实习面经：React 与工程化", "TypeScript 类型体操入门", "前端性能优化清单"},
	"b-linux":      {"Linux 环境配置踩坑记录", "从零配置开发服务器", "常用 shell 脚本整理"},
	"b-kaoyan":     {"考研数学复习经验分享", "考研时间规划与择校", "复试英语准备方法"},
	"b-intern":     {"Go 后端实习内推，base 北京", "暑期实习选择求建议", "实习面试常见问题"},
	"b-autumn":     {"秋招面经：后端开发三面复盘", "秋招投递节奏记录", "offer 比较：平台还是业务"},
	"b-canteen":    {"食堂新窗口测评：麻辣香锅", "食堂早餐时间调整", "哪个窗口最适合自习"},
	"b-photo":      {"摄影：秋天的校园角落", "校园夜景拍摄参数分享", "二手镜头求推荐"},
	"b-mood":       {"心情：最近有点焦虑怎么办", "如何平衡科研与生活", "深夜碎碎念"},
	"b-notice":     {"站务公告：版面规则更新", "关于搜索功能的说明", "新版客户端意见征集"},
	"b-secondhand": {"二手显示器出售，九成新", "出闲置机械键盘", "求购自行车一辆"},
}

func (s *Source) seedBoard(board domain.BoardID, name string) {
	n := 42
	list := make([]domain.Article, 0, n)
	for i := 0; i < n; i++ {
		title := topics[(i*3+int(len(board)))%len(topics)]
		if own := boardTopics[board]; len(own) > 0 {
			title = own[i%len(own)]
		}
		pinned := i%17 == 3
		if pinned {
			title = "[置顶] " + title
		} else {
			title = fmt.Sprintf("%s（%s #%02d）", title, name, i+1)
		}
		created := s.now.Add(-time.Duration((i*11+len(board)*7)%120)*24*time.Hour - time.Duration((i*3)%12)*time.Hour)
		replyCount := 4 + (i*5+len(board))%18
		lastReply := created.Add(time.Duration(replyCount/3+1) * 6 * time.Hour)
		if lastReply.After(s.now) {
			lastReply = s.now
		}
		key := domain.ArticleKey{SiteID: siteID, BoardID: board, ArticleID: domain.ArticleID(fmt.Sprintf("%s-a%03d", board, i+1))}
		var images []domain.Image
		if i%13 == 0 {
			images = append(images, domain.Image{
				URL: fmt.Sprintf("demo://image/%s-%d", board, i+1), Kind: domain.ImagePhoto,
			})
			if i == 0 {
				images = append(images, domain.Image{
					URL: fmt.Sprintf("demo://emoji/%s-%d", board, i+1), Kind: domain.ImageEmoji,
				})
			}
		}
		list = append(list, domain.Article{
			Key:         key,
			Title:       title,
			Author:      authors[(i+len(board))%len(authors)],
			Pinned:      pinned,
			ReplyCount:  replyCount,
			CreatedAt:   created,
			LastReplyAt: lastReply,
			Summary:     fmt.Sprintf("%s 的讨论帖，来自 %s 版面。这里是由演示数据生成的摘要，真实内容将在接入 BYR 爬虫后替换。", title, name),
			Content:     fmt.Sprintf("这是「%s」的主帖正文。\n\n当前版本使用内存演示数据，用来验证浏览、搜索、增量入库和界面交互。真实论坛的网络读取、登录会话、GBK 解码与分页解析会在后续适配器中实现。\n\n欢迎在回复中继续讨论。", title),
			Images:      images,
			BodyLoaded:  false,
		})
		replies := make([]domain.Reply, 0, replyCount)
		for j := 0; j < replyCount; j++ {
			rkey := domain.ReplyKey{ArticleKey: key, ReplyID: domain.ReplyID(fmt.Sprintf("%s-r%03d", key.ArticleID, j+1))}
			content := fmt.Sprintf("回复 #%d：%s", j+1, replyText(title, j))
			replies = append(replies, domain.Reply{
				Key:       rkey,
				Floor:     j + 1,
				Author:    authors[(i+j+3)%len(authors)],
				Content:   content,
				CreatedAt: created.Add(time.Duration(j+1) * 3 * time.Hour),
			})
		}
		for j := 2; j < len(replies); j += 4 {
			parent := replies[j-2]
			replies[j].QuoteAuthor = parent.Author
			replies[j].QuoteContent = parent.Content
		}
		s.replies[key] = replies
	}
	s.boards[board] = list
}

func (s *Source) ArticleURL(key domain.ArticleKey) string {
	return "https://bbs.byr.cn/article/" + string(key.BoardID) + "/" + string(key.ArticleID)
}

func (s *Source) ReadSectionTraffic(_ context.Context, _ domain.Scope, _ string, boards []domain.BoardID) (map[domain.BoardID]domain.Traffic, error) {
	result := make(map[domain.BoardID]domain.Traffic, len(boards))
	for _, board := range boards {
		seed := 0
		for _, r := range string(board) {
			seed = seed*131 + int(r)
		}
		if seed < 0 {
			seed = -seed
		}
		result[board] = domain.Traffic{
			Known:       true,
			OnlineUsers: 4 + seed%48,
			TodayPosts:  10 + (seed/7)%180,
			Threads:     180 + (seed/13)%1200,
			Posts:       2400 + (seed/17)%12000,
			ObservedAt:  s.now,
		}
	}
	return result, nil
}

func (s *Source) ReadImage(_ context.Context, rawURL string) ([]byte, error) {
	if !strings.HasPrefix(rawURL, "demo://") {
		return nil, fmt.Errorf("unsupported demo image %q", rawURL)
	}
	width, height := 800, 450
	if strings.HasPrefix(rawURL, "demo://emoji/") {
		width, height = 64, 64
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	seed := 0
	for _, r := range rawURL {
		seed = seed*31 + int(r)
	}
	if seed < 0 {
		seed = -seed
	}
	base := []color.RGBA{{R: 53, G: 95, B: 192}, {R: 93, G: 67, B: 176}, {R: 30, G: 126, B: 154}}[seed%3]
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			f := float64(x+y) / float64(width+height)
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(float64(base.R)*(1-f*0.45) + 235*f*0.45),
				G: uint8(float64(base.G)*(1-f*0.45) + 240*f*0.45),
				B: uint8(float64(base.B)*(1-f*0.45) + 250*f*0.45),
				A: 255,
			})
		}
	}
	for i := 0; i < 7; i++ {
		cx := (seed*17 + i*97) % width
		cy := (seed*29 + i*61) % height
		radius := 24 + (seed+i*31)%90
		for y := max(0, cy-radius); y < min(height, cy+radius); y++ {
			for x := max(0, cx-radius); x < min(width, cx+radius); x++ {
				dx, dy := x-cx, y-cy
				if dx*dx+dy*dy <= radius*radius {
					c := img.RGBAAt(x, y)
					c.R = min(255, c.R+35)
					c.G = min(255, c.G+35)
					c.B = min(255, c.B+35)
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func replyText(title string, i int) string {
	replies := []string{
		"感谢分享，信息很有用。",
		"同问，蹲一个后续。",
		"我上周也遇到过类似情况，可以私信交流。",
		"补充一点：最好先确认一下时间和地点。",
		"这个方向最近确实比较热门。",
		"楼主整理得很清楚，收藏了。",
		"建议再对比几个选择，不要只看薪资。",
		"已报名，到时候见。",
		"我这边有一份资料，需要的话可以发出来。",
		"先标记一下，晚点仔细看。",
	}
	return replies[(i+len(title))%len(replies)]
}
