package byr

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"

	"BYR-Client/internal/domain"
)

var (
	articleHrefPattern = regexp.MustCompile(`/article/([^/]+)/([^/?#]+)`)
	floorPosPattern    = regexp.MustCompile(`第(\d+)楼`)
	floorAnchorPattern = regexp.MustCompile(`^a(\d+)$`)
	postTimePattern    = regexp.MustCompile(`\(([A-Z][a-z]{2})\s+([A-Z][a-z]{2})\s+(\d{1,2})\s+(\d{1,2}):(\d{2}):(\d{2})\s+(\d{4})\)`)
	quoteHeaderPattern = regexp.MustCompile(`(?m)^\s*【\s*在\s*([^\s（(]+).*?的大作中提到\s*[:：]\s*】\s*$`)
)

var cst = time.FixedZone("CST", 8*3600)

func ajaxHeaders(referer string) map[string]string {
	return map[string]string{
		"X-Requested-With": "XMLHttpRequest",
		"Accept":           "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Referer":          referer,
	}
}

func (c *Client) ReadBoardPage(ctx context.Context, _ domain.Scope, board domain.BoardID, cursor domain.Cursor, _ int) (domain.BoardPage, error) {
	if err := c.requireLogin(); err != nil {
		return domain.BoardPage{}, err
	}
	if strings.TrimSpace(string(board)) == "" {
		return domain.BoardPage{}, fmt.Errorf("版块标识为空")
	}
	page := pageFromCursor(cursor)
	path := "/board/" + escapePath(string(board))
	if page > 1 {
		path += "?p=" + strconv.Itoa(page)
	}
	path = withUID(path, c.Session().AccountID)
	body, _, err := c.do(ctx, "GET", path, nil, ajaxHeaders(c.cfg.BaseURL+"/default"), false, true)
	if err != nil {
		return domain.BoardPage{}, err
	}
	doc, err := parseHTML(body)
	if err != nil {
		return domain.BoardPage{}, fmt.Errorf("解析版面 %s 失败: %w", board, err)
	}
	articles := parseArticleList(doc, board)
	next, end := nextPageCursor(doc, page)
	return domain.BoardPage{
		BoardID:    board,
		Articles:   articles,
		Next:       next,
		End:        end,
		ObservedAt: time.Now(),
	}, nil
}

func (c *Client) SearchBoardPage(ctx context.Context, _ domain.Scope, board domain.BoardID, query domain.BoardQuery, cursor domain.Cursor, _ int) (domain.BoardPage, error) {
	if err := c.requireLogin(); err != nil {
		return domain.BoardPage{}, err
	}
	page := pageFromCursor(cursor)
	session := c.Session()
	params := []string{"b=" + encodeQueryPart(string(board))}
	keywords := strings.TrimSpace(strings.Join(query.Keywords, " "))
	if keywords != "" {
		params = append(params, "t1="+encodeQueryPart(keywords))
	}
	// au must exist even when empty; BYR returns no results without it.
	params = append(params, "au="+encodeQueryPart(strings.TrimSpace(query.Author)))
	if page > 1 {
		params = append(params, "p="+strconv.Itoa(page))
	}
	params = append(params, "_uid="+encodeQueryPart(session.AccountID))
	path := "/s/article?" + strings.Join(params, "&")
	body, _, err := c.do(ctx, "GET", path, nil, ajaxHeaders(c.cfg.BaseURL+"/default"), false, true)
	if err != nil {
		return domain.BoardPage{}, err
	}
	doc, err := parseHTML(body)
	if err != nil {
		return domain.BoardPage{}, fmt.Errorf("解析版面 %s 搜索结果失败: %w", board, err)
	}
	articles := parseArticleList(doc, board)
	next, end := nextPageCursor(doc, page)
	return domain.BoardPage{
		BoardID:    board,
		Articles:   articles,
		Next:       next,
		End:        end,
		ObservedAt: time.Now(),
	}, nil
}

func (c *Client) ReadThreadPage(ctx context.Context, _ domain.Scope, key domain.ArticleKey, cursor domain.Cursor, _ int) (domain.ThreadPage, error) {
	if err := c.requireLogin(); err != nil {
		return domain.ThreadPage{}, err
	}
	page := pageFromCursor(cursor)
	path := "/article/" + escapePath(string(key.BoardID)) + "/" + escapePath(string(key.ArticleID))
	if page > 1 {
		path += "?p=" + strconv.Itoa(page)
	}
	path = withUID(path, c.Session().AccountID)
	body, _, err := c.do(ctx, "GET", path, nil, ajaxHeaders(c.cfg.BaseURL+"/default"), false, true)
	if err != nil {
		return domain.ThreadPage{}, err
	}
	doc, err := parseHTML(body)
	if err != nil {
		return domain.ThreadPage{}, fmt.Errorf("解析主题 %s 失败: %w", key.ArticleID, err)
	}
	article, replies, err := parseThread(doc, key, c.cfg.BaseURL)
	if err != nil {
		return domain.ThreadPage{}, err
	}
	next, end := nextPageCursor(doc, page)
	return domain.ThreadPage{
		Article:    article,
		Replies:    replies,
		Next:       next,
		End:        end,
		PagesRead:  1,
		ObservedAt: time.Now(),
	}, nil
}

func pageFromCursor(cursor domain.Cursor) int {
	page, err := strconv.Atoi(string(cursor))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func nextPageCursor(doc *xhtml.Node, current int) (domain.Cursor, bool) {
	pagination := findClass(doc, "pagination")
	if pagination == nil {
		return "", true
	}
	next := findHTML(pagination, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && node.Data == "a" && attr(node, "title") == "下一页"
	})
	if next == nil {
		return "", true
	}
	return domain.Cursor(strconv.Itoa(current + 1)), false
}

func parseArticleList(doc *xhtml.Node, board domain.BoardID) []domain.Article {
	table := findHTML(doc, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && node.Data == "table" && hasClass(node, "board-list")
	})
	if table == nil {
		return nil
	}
	rows := findAllTags(table, "tr")
	articles := make([]domain.Article, 0, len(rows))
	for _, row := range rows {
		titleCell := findClass(row, "title_9")
		if titleCell == nil {
			continue
		}
		link := findTag(titleCell, "a")
		if link == nil {
			continue
		}
		title := cleanText(textContent(link))
		href := attr(link, "href")
		boardFromLink, articleID := articleRef(href)
		if articleID == "" || title == "" {
			continue
		}
		boardID := board
		if boardFromLink != "" {
			boardID = domain.BoardID(boardFromLink)
		}

		author := ""
		authorCells := findAllClass(row, "title_12")
		if len(authorCells) > 0 {
			link := findHTML(authorCells[0], func(node *xhtml.Node) bool {
				return node.Type == xhtml.ElementNode && node.Data == "a" && strings.Contains(attr(node, "href"), "/user/query/")
			})
			if link != nil {
				author = cleanText(textContent(link))
			} else {
				author = strings.Trim(cleanText(textContent(authorCells[0])), "| \t\r\n")
			}
		}

		dates := findAllClass(row, "title_10")
		created := time.Time{}
		lastReply := time.Time{}
		if len(dates) > 0 {
			created = parseForumDate(cleanText(textContent(dates[0])))
		}
		if len(dates) > 1 {
			lastReply = parseForumDate(cleanText(textContent(dates[1])))
		}
		if lastReply.IsZero() {
			lastReply = created
		}

		replyCount := 0
		if cell := findClass(row, "title_11"); cell != nil {
			replyCount = leadingInt(cleanText(textContent(cell)))
		}
		pinned := hasClass(row, "top") || findClass(row, "ico-pos-article-top") != nil
		articles = append(articles, domain.Article{
			Key:         domain.ArticleKey{SiteID: SiteID, BoardID: boardID, ArticleID: domain.ArticleID(articleID)},
			Title:       title,
			Author:      author,
			Pinned:      pinned,
			ReplyCount:  replyCount,
			CreatedAt:   created,
			LastReplyAt: lastReply,
		})
	}
	return articles
}

func parseThread(doc *xhtml.Node, key domain.ArticleKey, baseURL string) (domain.Article, []domain.Reply, error) {
	title := ""
	if head := findClass(doc, "b-head"); head != nil {
		if label := findClass(head, "n-left"); label != nil {
			title = cleanText(textContent(label))
		} else {
			title = cleanText(textContent(head))
		}
	}
	title = strings.TrimSpace(strings.TrimPrefix(title, "文章主题:"))

	wraps := findAllClass(doc, "a-wrap")
	if len(wraps) == 0 {
		if title == "" {
			return domain.Article{}, nil, fmt.Errorf("主题 %s 页面结构无法识别", key.ArticleID)
		}
		return domain.Article{Key: key, Title: title, BodyLoaded: true}, nil, nil
	}

	article := domain.Article{Key: key, Title: title, BodyLoaded: true}
	replies := make([]domain.Reply, 0, len(wraps))
	seenFloors := make(map[int]bool)
	for index, wrap := range wraps {
		contentNode := findClass(wrap, "a-content-wrap")
		if contentNode == nil {
			contentNode = findClass(wrap, "a-content")
		}
		if contentNode == nil {
			continue
		}
		raw := textContent(contentNode)
		content, postTime := cleanPostContent(raw)
		if content == "" {
			content = strings.TrimSpace(raw)
		}
		content, quoteAuthor, quoteContent := splitQuote(content)
		floor := floorNumber(wrap, index+1)
		if floor <= 0 || seenFloors[floor] {
			floor = index + 1
		}
		seenFloors[floor] = true
		author := "匿名"
		if name := findClass(wrap, "a-u-name"); name != nil {
			author = cleanText(textContent(name))
		}
		if author == "" {
			author = "匿名"
		}
		images := extractImages(contentNode, baseURL)
		if floor == 1 {
			article.Author = author
			article.Content = content
			article.Images = images
			article.CreatedAt = postTime
			continue
		}
		replies = append(replies, domain.Reply{
			Key: domain.ReplyKey{
				ArticleKey: key,
				ReplyID:    domain.ReplyID(fmt.Sprintf("%s-f%04d", key.ArticleID, floor)),
			},
			Floor:        floor,
			Author:       author,
			Content:      content,
			Images:       images,
			QuoteAuthor:  quoteAuthor,
			QuoteContent: quoteContent,
			CreatedAt:    postTime,
		})
	}
	if len(replies) > 0 {
		article.LastReplyAt = replies[len(replies)-1].CreatedAt
	} else {
		article.LastReplyAt = article.CreatedAt
	}
	article.ReplyCount = len(replies)
	return article, replies, nil
}

func floorNumber(wrap *xhtml.Node, fallback int) int {
	if pos := findClass(wrap, "a-pos"); pos != nil {
		text := cleanText(textContent(pos))
		switch text {
		case "楼主":
			return 1
		case "沙发":
			return 2
		case "板凳":
			return 3
		}
		if match := floorPosPattern.FindStringSubmatch(text); len(match) == 2 {
			if value, err := strconv.Atoi(match[1]); err == nil {
				return value
			}
		}
	}
	if anchor := findHTML(wrap, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && node.Data == "a" && floorAnchorPattern.MatchString(attr(node, "name"))
	}); anchor != nil {
		if match := floorAnchorPattern.FindStringSubmatch(attr(anchor, "name")); len(match) == 2 {
			if value, err := strconv.Atoi(match[1]); err == nil {
				return value + 1
			}
		}
	}
	for sibling := wrap.PrevSibling; sibling != nil; sibling = sibling.PrevSibling {
		if sibling.Type != xhtml.ElementNode {
			continue
		}
		if match := floorAnchorPattern.FindStringSubmatch(attr(sibling, "name")); len(match) == 2 {
			if value, err := strconv.Atoi(match[1]); err == nil {
				return value + 1
			}
		}
	}
	return fallback
}

func extractImages(root *xhtml.Node, baseURL string) []domain.Image {
	imgs := findAllTags(root, "img")
	if len(imgs) == 0 {
		return nil
	}
	base, _ := url.Parse(baseURL)
	images := make([]domain.Image, 0, len(imgs))
	seen := make(map[string]bool)
	for _, img := range imgs {
		src := strings.TrimSpace(attr(img, "src"))
		if src == "" || strings.Contains(src, "none.gif") || strings.HasPrefix(src, "data:") {
			src = strings.TrimSpace(attr(img, "file"))
		}
		if src == "" {
			src = strings.TrimSpace(attr(img, "zoomfile"))
		}
		if src == "" || strings.HasPrefix(src, "data:") {
			continue
		}
		resolved := src
		if strings.HasPrefix(src, "//") {
			resolved = "https:" + src
		} else if parsed, err := url.Parse(src); err == nil && !parsed.IsAbs() {
			if base != nil {
				resolved = base.ResolveReference(parsed).String()
			}
		}
		if !strings.HasPrefix(resolved, "http://") && !strings.HasPrefix(resolved, "https://") {
			continue
		}
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		width := leadingInt(attr(img, "width"))
		height := leadingInt(attr(img, "height"))
		images = append(images, domain.Image{
			URL:    resolved,
			Kind:   classifyImage(resolved, attr(img, "class"), attr(img, "alt"), width, height),
			Width:  width,
			Height: height,
		})
	}
	return images
}

func classifyImage(rawURL, class, alt string, width, height int) domain.ImageKind {
	hint := strings.ToLower(class + " " + alt + " " + rawURL)
	for _, marker := range []string{
		"smiley", "smilies", "emoticon", "emotion", "emoji", "biaoqing",
		"sticker", "表情", "doge", "huaji", "qqface", "qq_face",
		"/face/", "face_", "/smile", "/em/", "static/image/smiley", "static/image/face",
		"static/image/common/",
	} {
		if strings.Contains(hint, marker) {
			return domain.ImageEmoji
		}
	}
	if width > 0 && height > 0 && width <= 64 && height <= 64 {
		return domain.ImageEmoji
	}
	return domain.ImagePhoto
}

func articleRef(href string) (string, string) {
	match := articleHrefPattern.FindStringSubmatch(href)
	if len(match) != 3 {
		return "", ""
	}
	board := match[1]
	if decoded, err := url.PathUnescape(board); err == nil {
		board = decoded
	}
	return board, match[2]
}

func leadingInt(value string) int {
	value = strings.TrimSpace(value)
	for i, r := range value {
		if r < '0' || r > '9' {
			value = value[:i]
			break
		}
	}
	result, _ := strconv.Atoi(value)
	return result
}

func parseForumDate(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"01-02 15:04",
		"01-02",
	} {
		if parsed, err := time.ParseInLocation(layout, value, cst); err == nil {
			if parsed.Year() == 0 {
				now := time.Now().In(cst)
				parsed = time.Date(now.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, cst)
			}
			return parsed
		}
	}
	return time.Time{}
}

func cleanPostContent(raw string) (string, time.Time) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	headerEnd := 0
	postTime := time.Time{}
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "发信人:") {
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "发信站:") {
				postTime = parsePostTime(trimmed)
			}
			if trimmed == "" {
				headerEnd = i + 1
				break
			}
			if !strings.HasPrefix(trimmed, "发信人:") && !strings.HasPrefix(trimmed, "标") && !strings.HasPrefix(trimmed, "发信站:") {
				headerEnd = i
				break
			}
		}
	}
	footerStart := len(lines)
	for i := len(lines) - 1; i >= headerEnd; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "※ 来源:") || strings.HasPrefix(trimmed, "※ 修改:") || strings.HasPrefix(trimmed, "※来源:") || strings.HasPrefix(trimmed, "※修改:") {
			footerStart = i
			for j := i - 1; j >= headerEnd; j-- {
				if strings.TrimSpace(lines[j]) == "--" {
					footerStart = j
					break
				}
			}
			break
		}
	}
	if footerStart == len(lines) {
		for i := len(lines) - 1; i >= headerEnd; i-- {
			if strings.TrimSpace(lines[i]) == "--" {
				footerStart = i
				break
			}
		}
	}
	if headerEnd > footerStart {
		return "", postTime
	}
	body := strings.Join(lines[headerEnd:footerStart], "\n")
	return strings.TrimSpace(body), postTime
}

// splitQuote recognizes BYR's reply quote block:
//
//	【 在 EricMoin 的大作中提到: 】
//	: quoted line 1
//	: quoted line 2
//
// It returns the reply body without that block, the quoted author and the
// quoted text. Replies that do not quote anything are returned unchanged.
func splitQuote(content string) (body, author, quote string) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	start := -1
	for index, line := range lines {
		match := quoteHeaderPattern.FindStringSubmatch(line)
		if len(match) == 2 {
			author = strings.TrimSpace(match[1])
			start = index
			break
		}
	}
	if start < 0 {
		return content, "", ""
	}
	quoted := make([]string, 0, 4)
	index := start + 1
	for index < len(lines) {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			index++
			continue
		}
		if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "：") {
			quoted = append(quoted, strings.TrimSpace(strings.TrimLeft(line, ":：")))
			index++
			continue
		}
		break
	}
	remaining := make([]string, 0, len(lines)-(index-start))
	remaining = append(remaining, lines[:start]...)
	remaining = append(remaining, lines[index:]...)
	return strings.TrimSpace(strings.Join(remaining, "\n")), author, strings.TrimSpace(strings.Join(quoted, "\n"))
}

func parsePostTime(value string) time.Time {
	match := postTimePattern.FindStringSubmatch(value)
	if len(match) != 8 {
		return time.Time{}
	}
	months := map[string]time.Month{
		"Jan": time.January, "Feb": time.February, "Mar": time.March,
		"Apr": time.April, "May": time.May, "Jun": time.June,
		"Jul": time.July, "Aug": time.August, "Sep": time.September,
		"Oct": time.October, "Nov": time.November, "Dec": time.December,
	}
	month, ok := months[match[2]]
	if !ok {
		return time.Time{}
	}
	day, _ := strconv.Atoi(match[3])
	hour, _ := strconv.Atoi(match[4])
	minute, _ := strconv.Atoi(match[5])
	second, _ := strconv.Atoi(match[6])
	year, _ := strconv.Atoi(match[7])
	return time.Date(year, month, day, hour, minute, second, 0, cst)
}
