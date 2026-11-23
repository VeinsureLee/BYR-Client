package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func benchThreadFixture(n int) forum.Thread {
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
	return forum.Thread{ID: "1", Board: "Go", BoardDescription: "Go 讨论", Title: "离线长帖滚动夹具", Articles: articles, ReplyCount: forum.Number(n - 1), Pagination: forum.Pagination{CurrentPage: 1, TotalPages: 3}}
}

func BenchmarkThreadScrollFrame(b *testing.B) {
	a, err := New(noopInvalidator{})
	if err != nil {
		b.Fatal(err)
	}
	a.started = true
	a.ready = true
	a.thread = benchThreadFixture(30)
	a.prepareThreadContent(a.thread)
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "1", Number: 1}}
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Source: router.Source(), Constraints: layout.Exact(image.Pt(420, 700)), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	a.beginImageLayoutFrame()
	a.threadPage(gtx)
	a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}).Position.First = 8
	ops.Reset()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var frame op.Ops
		gtx.Ops = &frame
		a.beginImageLayoutFrame()
		a.threadPage(gtx)
	}
}
