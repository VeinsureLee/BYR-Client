package byr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
)

const (
	rootSections = "list-section"
	maxTreeDepth = 16
)

var (
	hrefPattern = regexp.MustCompile(`href\s*=\s*["']([^"']*)["']`)
	namePattern = regexp.MustCompile(`(?s)>([^<]+)<`)
)

type ajaxEntry struct {
	Text string `json:"t"`
	ID   any    `json:"id"`
}

type rawNode struct {
	ID       string
	Name     string
	Href     string
	BoardID  string
	Section  bool
	Children []rawNode
}

func (c *Client) ReadCatalog(ctx context.Context) (domain.Catalog, error) {
	if err := c.requireLogin(); err != nil {
		return domain.Catalog{}, err
	}
	session := c.Session()
	warnings := make([]string, 0)
	stack := make(map[string]bool)
	raw, err := c.readChildren(ctx, rootSections, 0, session.AccountID, stack, &warnings)
	if err != nil {
		return domain.Catalog{}, err
	}
	catalog := buildCatalog(raw, warnings)
	catalog.FetchedAt = time.Now()
	return catalog, nil
}

func (c *Client) readChildren(ctx context.Context, parent string, depth int, uid string, stack map[string]bool, warnings *[]string) ([]rawNode, error) {
	if depth > maxTreeDepth {
		*warnings = append(*warnings, fmt.Sprintf("目录层级超过 %d 层，已停止展开 %s", maxTreeDepth, parent))
		return nil, nil
	}
	if stack[parent] {
		*warnings = append(*warnings, fmt.Sprintf("目录出现循环引用：%s", parent))
		return nil, nil
	}
	stack[parent] = true
	defer delete(stack, parent)

	path := "/section/ajax_list.json?uid=" + url.QueryEscape(uid) +
		"&root=" + url.QueryEscape(parent) + "&_uid=" + url.QueryEscape(uid)
	body, _, err := c.do(ctx, "GET", path, nil, map[string]string{
		"X-Requested-With": "XMLHttpRequest",
		"Accept":           "application/json, text/javascript, */*; q=0.01",
		"Referer":          c.cfg.BaseURL + "/default",
	}, false, true)
	if err != nil {
		return nil, err
	}
	var entries []ajaxEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("解析分区列表 %s 失败: %w", parent, err)
	}

	result := make([]rawNode, 0, len(entries))
	for _, entry := range entries {
		entryID := entryIDString(entry.ID)
		name := entryName(entry.Text)
		if name == "" {
			continue
		}
		href := entryHref(entry.Text)
		switch {
		case strings.Contains(href, "/board/"):
			boardID := boardFromHref(href)
			if boardID == "" {
				*warnings = append(*warnings, fmt.Sprintf("版块 %s 缺少英文标识，已跳过", name))
				continue
			}
			result = append(result, rawNode{ID: entryID, Name: name, Href: href, BoardID: boardID})
		case strings.Contains(href, "/section/"):
			if entryID == "" {
				*warnings = append(*warnings, fmt.Sprintf("分区 %s 缺少节点 ID，已跳过", name))
				continue
			}
			children, err := c.readChildren(ctx, entryID, depth+1, uid, stack, warnings)
			if err != nil {
				*warnings = append(*warnings, fmt.Sprintf("分区 %s 读取失败：%v", name, err))
				children = nil
			}
			result = append(result, rawNode{
				ID: entryID, Name: name, Href: href, Section: true, Children: children,
			})
		}
	}
	return result, nil
}

func entryIDString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func entryName(text string) string {
	match := namePattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return cleanText(match[1])
}

func entryHref(text string) string {
	match := hrefPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func boardFromHref(href string) string {
	index := strings.Index(href, "/board/")
	if index < 0 {
		return ""
	}
	value := href[index+len("/board/"):]
	if cut := strings.IndexAny(value, "?#"); cut >= 0 {
		value = value[:cut]
	}
	value = strings.Trim(value, "/")
	if decoded, err := url.PathUnescape(value); err == nil {
		value = decoded
	}
	return strings.TrimSpace(value)
}

func buildCatalog(raw []rawNode, warnings []string) domain.Catalog {
	used := make(map[string]bool)
	unique := func(base string) string {
		base = strings.TrimSpace(base)
		if base == "" {
			base = "node"
		}
		if !used[base] {
			used[base] = true
			return base
		}
		for i := 2; ; i++ {
			candidate := fmt.Sprintf("%s#%d", base, i)
			if !used[candidate] {
				used[candidate] = true
				return candidate
			}
		}
	}

	var nodes []forum.Node
	var convert func(entries []rawNode, depth int) []forum.NodeID
	convert = func(entries []rawNode, depth int) []forum.NodeID {
		childIDs := make([]forum.NodeID, 0, len(entries))
		for _, entry := range entries {
			if entry.Section {
				id := forum.NodeID(unique("sec:" + entry.ID))
				kind := forum.KindSubForum
				if depth == 0 {
					kind = forum.KindForum
				}
				children := convert(entry.Children, depth+1)
				nodes = append(nodes, forum.Node{ID: id, Kind: kind, Name: entry.Name, Children: children})
				childIDs = append(childIDs, id)
				continue
			}

			boardNodeID := forum.NodeID(unique("board:" + entry.BoardID))
			nodes = append(nodes, forum.Node{
				ID: boardNodeID, Kind: forum.KindBoard, Name: entry.Name, BoardID: forum.BoardID(entry.BoardID),
			})
			if depth == 0 {
				// A board directly under the site root still needs a forum parent
				// for Navigation, while the directory keeps the same board identity.
				forumID := forum.NodeID(unique("forum-board:" + entry.BoardID))
				nodes = append(nodes, forum.Node{
					ID: forumID, Kind: forum.KindForum, Name: entry.Name,
					Children: []forum.NodeID{boardNodeID},
				})
				childIDs = append(childIDs, forumID)
				continue
			}
			childIDs = append(childIDs, boardNodeID)
		}
		return childIDs
	}

	roots := convert(raw, 0)
	return domain.Catalog{Roots: roots, Nodes: nodes, Warnings: warnings}
}
