package desktop

import (
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/egoist/mygo/ui"
)

func desktopLongThread(n int) forum.Thread {
	articles := make([]forum.Article, n)
	for i := range articles {
		articles[i] = forum.Article{
			ID:       forum.Scalar(string(rune('a' + i))),
			Position: forum.Number(i + 1),
			Author:   forum.User{ID: "alice", UserName: "Alice"},
			Content:  "这是用于滚动性能对照的长帖正文，包含中文、English words、数字 12345 和 [b]加粗片段[/b]。\n第二自然段保持换行。",
		}
	}
	return forum.Thread{
		ID: "fixture", Board: "Go", BoardDescription: "Go 讨论", Title: "离线长帖滚动夹具",
		Articles: articles, ReplyCount: forum.Number(n - 1),
		Pagination: forum.Pagination{CurrentPage: 1, TotalPages: 3},
	}
}

func TestThreadScrollParseBaseline(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "thread", Name: "Go", ID: "fixture", Number: 1}
	h.app.thread = desktopLongThread(30)
	h.app.prepareThreadModels(h.app.thread)
	h.view.SetSize(1200, 700)
	h.view.Frame()
	state := h.app.desktopListState(h.app.threadListKey())
	before := content.ParseCalls.Load()
	state.ScrollTo(8, ui.Start)
	h.view.Frame()
	delta := content.ParseCalls.Load() - before
	t.Logf("scroll parse delta=%d", delta)
	if delta != 0 {
		t.Fatalf("unchanged floors parsed again while scrolling: %d", delta)
	}
}
