package fyneproto

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
	"BYR-Client/internal/source/demo"
)

const prototypeBoard domain.BoardID = "b-prototype"

// StressKey identifies the offline long thread used by the prototype checks.
func StressKey() domain.ArticleKey {
	return domain.ArticleKey{SiteID: "byr", BoardID: prototypeBoard, ArticleID: "stress-thread"}
}

type prototypeSource struct {
	*demo.Source
	now      time.Time
	articles []domain.Article
	replies  map[domain.ArticleKey][]domain.Reply
}

func newPrototypeSource(now time.Time, count int) *prototypeSource {
	if count < 1000 {
		count = 1200
	}
	s := &prototypeSource{Source: demo.New(now), now: now, replies: make(map[domain.ArticleKey][]domain.Reply)}
	for i := 0; i < 80; i++ {
		key := domain.ArticleKey{SiteID: "byr", BoardID: prototypeBoard, ArticleID: domain.ArticleID(fmt.Sprintf("prototype-%03d", i))}
		title := fmt.Sprintf("跨端阅读体验 · 中文主题 %02d", i+1)
		n := 12
		if i == 0 {
			key, title, n = StressKey(), fmt.Sprintf("Fyne 原型：%d 楼中文图文压力帖", count), count
		}
		a := domain.Article{
			Key: key, Title: title, Author: "BYRPrototype", ReplyCount: n,
			CreatedAt: now.Add(-time.Duration(i) * time.Hour), LastReplyAt: now,
			Summary: "离线演示数据，用于检验中文排版、图片、引用回复、分页和移动端适配。",
			Content: "这是 Fyne 跨端原型的离线演示帖，内容不是从北邮人论坛获取。\n\n请在 Windows 和 Android 上检查中文换行、文字选择、图片比例、滚动和引用折叠。打开键盘后也应能继续输入和发送。\n\n阅读测试：中文 English 12345，标点、长句与空白段落。这里保留现有 Go 领域模型，图片按独立内容块显示；原站图文顺序保真不属于本原型已完成的能力。\n\n链接测试：https://bbs.byr.cn/\n\n下方包含可缩放图片以及可继续加载的回复。压力帖可以一次加载全部回复，再检查长列表交互。",
			Images:  []domain.Image{{URL: "demo://image/prototype-campus", Kind: domain.ImagePhoto, Width: 800, Height: 450}, {URL: "demo://emoji/prototype-smile", Kind: domain.ImageEmoji, Width: 64, Height: 64}},
		}
		s.articles = append(s.articles, a)
		for j := 0; j < n; j++ {
			r := domain.Reply{
				Key:   domain.ReplyKey{ArticleKey: key, ReplyID: domain.ReplyID(fmt.Sprintf("%s-r%04d", key.ArticleID, j))},
				Floor: j + 2, Author: fmt.Sprintf("同学%02d", j%23+1),
				Content:   fmt.Sprintf("第 %d 条演示回复：多端使用同一份 Go 业务代码。请检查中文文本是否清晰、行宽变化是否自然，以及上下滚动后回复是否保持稳定。", j+1),
				CreatedAt: now.Add(time.Duration(j) * time.Second),
			}
			if j%9 == 0 {
				r.Content += "\n\n" + strings.Repeat("长段落排版测试：字号、换行与滚动行为应在手机和桌面上分别验证。", 6)
			}
			if j > 0 && j%3 == 0 {
				parent := s.replies[key][j-1]
				r.QuoteAuthor, r.QuoteContent = parent.Author, parent.Content
			}
			if j%50 == 0 {
				r.Images = []domain.Image{{URL: fmt.Sprintf("demo://image/reply-%d", j%150), Kind: domain.ImagePhoto, Width: 800, Height: 450}}
			}
			s.replies[key] = append(s.replies[key], r)
		}
	}
	return s
}

func (s *prototypeSource) ReadCatalog(ctx context.Context) (domain.Catalog, error) {
	if err := ctx.Err(); err != nil {
		return domain.Catalog{}, err
	}
	c, err := s.Source.ReadCatalog(ctx)
	if err != nil {
		return c, err
	}
	c.Roots = append([]forum.NodeID{"f-prototype"}, c.Roots...)
	c.Nodes = append([]forum.Node{
		{ID: "f-prototype", Kind: forum.KindForum, Name: "原型验证", Children: []forum.NodeID{forum.NodeID(prototypeBoard)}},
		{ID: forum.NodeID(prototypeBoard), Kind: forum.KindBoard, BoardID: prototypeBoard, Name: "跨端体验与压力测试"},
	}, c.Nodes...)
	return c, ctx.Err()
}

func (s *prototypeSource) ReadBoardPage(ctx context.Context, scope domain.Scope, board domain.BoardID, cursor domain.Cursor, limit int) (domain.BoardPage, error) {
	if err := ctx.Err(); err != nil {
		return domain.BoardPage{}, err
	}
	if board != prototypeBoard {
		p, err := s.Source.ReadBoardPage(ctx, scope, board, cursor, limit)
		if err == nil {
			err = ctx.Err()
		}
		return p, err
	}
	return s.boardPage(ctx, s.articles, cursor, limit)
}

func (s *prototypeSource) SearchBoardPage(ctx context.Context, scope domain.Scope, board domain.BoardID, query domain.BoardQuery, cursor domain.Cursor, limit int) (domain.BoardPage, error) {
	if err := ctx.Err(); err != nil {
		return domain.BoardPage{}, err
	}
	if board != prototypeBoard {
		p, err := s.Source.SearchBoardPage(ctx, scope, board, query, cursor, limit)
		if err == nil {
			err = ctx.Err()
		}
		return p, err
	}
	var items []domain.Article
	for _, a := range s.articles {
		if query.Author != "" && !strings.EqualFold(a.Author, query.Author) {
			continue
		}
		matched := len(query.Keywords) == 0
		for _, word := range query.Keywords {
			matched = matched || strings.Contains(strings.ToLower(a.Title), strings.ToLower(word))
		}
		if matched {
			items = append(items, a)
		}
	}
	return s.boardPage(ctx, items, cursor, limit)
}

func (s *prototypeSource) boardPage(ctx context.Context, articles []domain.Article, cursor domain.Cursor, limit int) (domain.BoardPage, error) {
	page, next, end, err := prototypePage(articles, cursor, limit)
	if err != nil {
		return domain.BoardPage{}, err
	}
	return domain.BoardPage{BoardID: prototypeBoard, Articles: page, Next: next, End: end, ObservedAt: s.now}, ctx.Err()
}

func (s *prototypeSource) ReadThreadPage(ctx context.Context, scope domain.Scope, key domain.ArticleKey, cursor domain.Cursor, limit int) (domain.ThreadPage, error) {
	if err := ctx.Err(); err != nil {
		return domain.ThreadPage{}, err
	}
	if key.BoardID != prototypeBoard {
		p, err := s.Source.ReadThreadPage(ctx, scope, key, cursor, limit)
		if err == nil {
			err = ctx.Err()
		}
		return p, err
	}
	for _, a := range s.articles {
		if a.Key != key {
			continue
		}
		page, next, end, err := prototypePage(s.replies[key], cursor, limit)
		if err != nil {
			return domain.ThreadPage{}, err
		}
		a.BodyLoaded = true
		return domain.ThreadPage{Article: a, Replies: page, Next: next, End: end, PagesRead: 1, ObservedAt: s.now}, ctx.Err()
	}
	return domain.ThreadPage{}, fmt.Errorf("演示主题不存在：%s", key.ArticleID)
}

func prototypePage[T any](items []T, cursor domain.Cursor, limit int) ([]T, domain.Cursor, bool, error) {
	offset := 0
	if cursor != "" {
		n, err := strconv.Atoi(string(cursor))
		if err != nil || n < 0 {
			return nil, "", false, fmt.Errorf("无效分页游标：%q", cursor)
		}
		offset = n
	}
	if limit <= 0 {
		limit = 20
	}
	if offset >= len(items) {
		return nil, "", true, nil
	}
	end := min(offset+limit, len(items))
	page := append([]T(nil), items[offset:end]...)
	if end == len(items) {
		return page, "", true, nil
	}
	return page, domain.Cursor(strconv.Itoa(end)), false, nil
}
