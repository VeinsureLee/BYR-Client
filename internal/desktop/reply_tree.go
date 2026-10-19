package desktop

import (
	"strings"
	"unicode"

	"BYR-Client/internal/domain"
)

type replyNode struct {
	Reply       domain.Reply
	Children    []*replyNode
	ParentFloor int
}

// buildReplyTree links replies that quote an earlier reply. BYR writes quoted
// replies as "【 在 author 的大作中提到: 】" followed by the quoted text; the
// adapter records that author and text, and this matcher finds the closest
// earlier reply with the same author and matching content.
func buildReplyTree(replies []domain.Reply) []*replyNode {
	nodes := make([]*replyNode, len(replies))
	for index := range replies {
		nodes[index] = &replyNode{Reply: replies[index]}
	}
	roots := make([]*replyNode, 0)
	for index, node := range nodes {
		if node.Reply.QuoteAuthor == "" {
			roots = append(roots, node)
			continue
		}
		parentIndex := -1
		for candidate := index - 1; candidate >= 0; candidate-- {
			if sameAuthor(nodes[candidate].Reply.Author, node.Reply.QuoteAuthor) &&
				quoteMatches(nodes[candidate].Reply.Content, node.Reply.QuoteContent) {
				parentIndex = candidate
				break
			}
		}
		if parentIndex >= 0 {
			parent := nodes[parentIndex]
			parent.Children = append(parent.Children, node)
			node.ParentFloor = parent.Reply.Floor
		} else {
			roots = append(roots, node)
		}
	}
	return roots
}

func expandReplyPath(nodes []*replyNode, floor int, open map[string]bool) bool {
	for _, node := range nodes {
		if node.Reply.Floor == floor {
			return true
		}
		if expandReplyPath(node.Children, floor, open) {
			open[string(node.Reply.Key.ReplyID)] = true
			return true
		}
	}
	return false
}

func sameAuthor(a, b string) bool {
	return normalizeText(a) == normalizeText(b)
}

func quoteMatches(content, quote string) bool {
	content = normalizeText(content)
	quote = normalizeText(quote)
	if content == "" || quote == "" {
		return false
	}
	if strings.Contains(content, quote) {
		return true
	}
	runes := []rune(quote)
	for _, length := range []int{60, 40, 24} {
		if len(runes) < length {
			continue
		}
		if strings.Contains(content, string(runes[:length])) {
			return true
		}
	}
	contentRunes := []rune(content)
	for _, length := range []int{40, 24} {
		if len(contentRunes) < length {
			continue
		}
		if strings.Contains(quote, string(contentRunes[:length])) {
			return true
		}
	}
	return false
}

func normalizeText(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsSpace(r) {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}
