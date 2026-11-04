package fyneui

import (
	"testing"

	"BYR-Client/internal/domain"
)

func TestReplyRowsCollapseOnlyQuotedDescendants(t *testing.T) {
	key := domain.ArticleKey{SiteID: "byr", BoardID: "test", ArticleID: "1"}
	article := domain.Article{Key: key, Title: "中文主题"}
	makeReply := func(id, author, content string, floor int) domain.Reply {
		return domain.Reply{Key: domain.ReplyKey{ArticleKey: key, ReplyID: domain.ReplyID(id)},
			Author: author, Content: content, Floor: floor}
	}
	parent := makeReply("parent", "alice", "这是父回复", 2)
	child := makeReply("child", "bob", "这是引用回复", 3)
	child.QuoteAuthor, child.QuoteContent = parent.Author, parent.Content
	unrelated := makeReply("other", "carol", "独立回复", 4)
	replies := []domain.Reply{parent, child, unrelated}
	rows := replyRows(article, replies, nil)
	if len(rows) != 4 || !rows[0].root || rows[2].parentFloor != 2 || rows[2].depth != 1 {
		t.Fatalf("quoted relationship was not preserved: %#v", rows)
	}
	rows = replyRows(article, replies, map[string]bool{"parent": false})
	if len(rows) != 3 || rows[1].reply.Key.ReplyID != "parent" || rows[2].reply.Key.ReplyID != "other" {
		t.Fatalf("collapse hid an unrelated reply or retained a descendant: %#v", rows)
	}
}

func TestReplyRowsPreserveUnknownQuoteAsIndependentReply(t *testing.T) {
	article := domain.Article{Title: "引用的父楼层尚未加载"}
	reply := domain.Reply{Author: "bob", QuoteAuthor: "missing", QuoteContent: "没有加载的原文"}
	rows := replyRows(article, []domain.Reply{reply}, nil)
	if len(rows) != 2 || rows[1].depth != 0 || rows[1].parentFloor != 0 {
		t.Fatalf("unknown source quote must remain visible without an invented parent: %#v", rows)
	}
}
