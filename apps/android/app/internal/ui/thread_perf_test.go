package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func longThreadFixture(n int) forum.Thread {
	articles := make([]forum.Article, n)
	for i := range articles {
		articles[i] = forum.Article{
			ID:       forum.Scalar(itoa(i + 1)),
			Position: forum.Number(i + 1),
			ReplyID:  forum.Scalar("1"),
			Author:   forum.User{ID: forum.Scalar("user-" + itoa(i)), UserName: "User" + itoa(i)},
			Content:  "这是用于滚动性能对照的长帖正文，包含中文、English words、数字 12345 和 [b]加粗片段[/b]。\n第二自然段保持换行。",
		}
	}
	return forum.Thread{
		ID: "1", Board: "Go", BoardDescription: "Go 讨论", Title: "离线长帖滚动夹具",
		Articles: articles, ReplyCount: forum.Number(n - 1),
		Pagination: forum.Pagination{CurrentPage: 1, TotalPages: 3},
	}
}

func renderThreadAt(t *testing.T, a *App, width, height int) {
	t.Helper()
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(width, height)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.beginImageLayoutFrame()
	a.threadPage(gtx)
}

// TestThreadScrollParseBaseline documents the parse work a thread scroll pass
// does. The cached implementation must keep the increment at zero.
func TestThreadScrollParseBaseline(t *testing.T) {
	a := previewApp(t)
	a.thread = longThreadFixture(30)
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "1", Number: 1}}
	a.prepareThreadContent(a.thread)
	renderThreadAt(t, a, 420, 700)
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	state := a.mobileList(key)
	before := content.ParseCalls.Load()
	state.Position.First = 8
	renderThreadAt(t, a, 420, 700)
	t.Logf("scroll parse delta=%d", content.ParseCalls.Load()-before)
	if got := content.ParseCalls.Load() - before; got != 0 {
		t.Fatalf("unchanged floors parsed again while scrolling: %d", got)
	}
}
