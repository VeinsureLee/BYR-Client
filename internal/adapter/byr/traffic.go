package byr

import (
	"context"
	"fmt"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"

	"BYR-Client/internal/domain"
)

// ReadSectionTraffic fetches one section detail page with count=1 and parses
// the per-board traffic columns used by the reference implementation.
func (c *Client) ReadSectionTraffic(ctx context.Context, _ domain.Scope, sectionID string, boards []domain.BoardID) (map[domain.BoardID]domain.Traffic, error) {
	if err := c.requireLogin(); err != nil {
		return nil, err
	}
	sectionID = strings.TrimSpace(sectionID)
	if sectionID == "" {
		return nil, fmt.Errorf("分区标识为空")
	}
	wanted := make(map[string]domain.BoardID, len(boards))
	for _, board := range boards {
		wanted[cleanBoardID(string(board))] = board
	}
	if len(wanted) == 0 {
		return map[domain.BoardID]domain.Traffic{}, nil
	}
	path := withUID("/section/"+escapePath(sectionID)+"?count=1", c.Session().AccountID)
	body, _, err := c.do(ctx, "GET", path, nil, ajaxHeaders(c.cfg.BaseURL+"/default"), false, true)
	if err != nil {
		return nil, err
	}
	doc, err := parseHTML(body)
	if err != nil {
		return nil, fmt.Errorf("解析分区 %s 流量失败: %w", sectionID, err)
	}
	result := make(map[domain.BoardID]domain.Traffic, len(boards))
	table := findHTML(doc, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && node.Data == "table" && hasClass(node, "board-list")
	})
	if table == nil {
		return result, nil
	}
	now := time.Now()
	for _, row := range findAllTags(table, "tr") {
		nameCell := findClass(row, "title_1")
		if nameCell == nil {
			continue
		}
		link := findTag(nameCell, "a")
		ename := ""
		if link != nil {
			ename = boardFromHref(attr(link, "href"))
		}
		if ename == "" {
			ename = cleanBoardID(cleanText(textContent(nameCell)))
		}
		board, ok := wanted[cleanBoardID(ename)]
		if !ok {
			continue
		}
		result[board] = domain.Traffic{
			Known:       true,
			OnlineUsers: leadingInt(cleanText(textContent(findClass(row, "title_4")))),
			TodayPosts:  leadingInt(cleanText(textContent(findClass(row, "title_5")))),
			Threads:     leadingInt(cleanText(textContent(findClass(row, "title_6")))),
			Posts:       leadingInt(cleanText(textContent(findClass(row, "title_7")))),
			ObservedAt:  now,
		}
	}
	return result, nil
}

func cleanBoardID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "(", "")
	value = strings.ReplaceAll(value, ")", "")
	value = strings.ReplaceAll(value, "（", "")
	value = strings.ReplaceAll(value, "）", "")
	return value
}
