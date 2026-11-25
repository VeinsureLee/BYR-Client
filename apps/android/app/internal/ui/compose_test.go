package ui

import (
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func loadComposeDraft(t *testing.T, a *App, key string) (local.Draft, bool) {
	t.Helper()
	draft, found, err := a.core.LoadDraft(context.Background(), a.compose.session, key)
	if err != nil {
		t.Fatal(err)
	}
	return draft, found
}

func TestComposeOpensReadyAndRendersWithoutNetwork(t *testing.T) {
	a := composeTestApp(t, "")
	defer a.Close()
	a.stack = []Route{{Page: PageBoard, Name: "Go", Number: 1}}
	a.thread.Title = "已经是 Re: 标题"

	a.openCompose("Go", "", "new")
	if got := a.current().Page; got != PageCompose {
		t.Fatalf("page = %v, want compose", got)
	}
	if !a.compose.ready || !a.compose.boardLocked || a.composeBoard() != "Go" {
		t.Fatalf("compose was not initialized: %+v", a.compose)
	}
	assertComposeRenders(t, a)
}

func TestGlobalComposeRestoresBoardlessDraft(t *testing.T) {
	a := composeTestApp(t, "")
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openCompose("", "", "new")
	a.compose.titleEditor.SetText("未选版面的草稿")
	a.compose.contentEditor.SetText("正文")
	a.back()
	a.openCompose("", "", "new")
	if got := a.compose.titleEditor.Text(); got != "未选版面的草稿" {
		t.Fatalf("restored title = %q", got)
	}
	if got := a.compose.contentEditor.Text(); got != "正文" {
		t.Fatalf("restored content = %q", got)
	}
}

func TestGlobalComposeSelectsBoardFromPicker(t *testing.T) {
	a := composeTestApp(t, "")
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openCompose("", "", "new")
	if got := a.current().Page; got != PageCompose {
		t.Fatalf("page = %v, want compose", got)
	}
	if a.compose.boardLocked || a.composeBoard() != "" {
		t.Fatalf("global composer should start without a board: %+v", a.compose)
	}
	assertComposeRenders(t, a)

	a.openSection("", true)
	if got := a.current().Page; got != PageSection || a.current().Sub != "pick" {
		t.Fatalf("board picker route = %+v", a.current())
	}
	a.selectComposeBoard("Go")
	if got := a.current().Page; got != PageCompose {
		t.Fatalf("picker did not return to compose: %v", got)
	}
	if got := a.composeBoard(); got != "Go" {
		t.Fatalf("board = %q, want Go", got)
	}
	assertComposeRenders(t, a)
}

func TestReplyTitleDoesNotDuplicatePrefix(t *testing.T) {
	a := composeTestApp(t, "")
	defer a.Close()
	a.stack = []Route{{Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	a.thread.Title = "Re: 已有前缀"
	a.openCompose("Go", "44", "reply")
	if got := a.compose.titleEditor.Text(); got != "Re: 已有前缀" {
		t.Fatalf("reply title = %q", got)
	}
}

func TestBackFromComposeSavesDraft(t *testing.T) {
	a := composeTestApp(t, "")
	defer a.Close()
	a.stack = []Route{{Page: PageBoard, Name: "Go", Number: 1}}
	a.openCompose("Go", "", "new")
	a.compose.titleEditor.SetText("草稿标题")
	a.compose.contentEditor.SetText("草稿正文")
	a.back()
	if got := a.current().Page; got != PageBoard {
		t.Fatalf("page after back = %v, want board", got)
	}
	draft, ok := loadComposeDraft(t, a, local.DraftKey("Go", "", ""))
	if !ok || draft.Title != "草稿标题" || draft.Content != "草稿正文" {
		t.Fatalf("draft was not saved: ok=%v draft=%+v", ok, draft)
	}
}

func TestClearingComposeRemovesSavedDrafts(t *testing.T) {
	a := composeTestApp(t, "")
	defer a.Close()
	a.stack = []Route{{Page: PageBoard, Name: "Go", Number: 1}}
	a.openCompose("Go", "", "new")
	a.compose.titleEditor.SetText("草稿标题")
	a.compose.contentEditor.SetText("草稿正文")
	if err := a.saveComposeDraft(); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadComposeDraft(t, a, local.DraftKey("Go", "", "")); !ok {
		t.Fatal("draft was not stored before clearing")
	}

	a.compose.titleEditor.SetText("")
	a.compose.contentEditor.SetText("")
	if err := a.saveComposeDraft(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{local.DraftKey("Go", "", ""), local.DraftKey("", "", "")} {
		if _, ok := loadComposeDraft(t, a, key); ok {
			t.Fatalf("clearing composer retained draft %q", key)
		}
	}
}

func TestComposeInputAutosavesChangedText(t *testing.T) {
	a := composeTestApp(t, "")
	defer a.Close()
	a.stack = []Route{{Page: PageBoard, Name: "Go", Number: 1}}
	a.openCompose("Go", "", "new")

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 680)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	a.composePage(gtx)
	router.Frame(&ops)
	gtx.Execute(key.FocusCmd{Tag: &a.compose.titleEditor})
	router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: "自动保存标题"})
	ops.Reset()
	a.composePage(gtx)
	draft, ok := loadComposeDraft(t, a, local.DraftKey("Go", "", ""))
	if !ok || draft.Title != "自动保存标题" {
		t.Fatalf("changed editor text was not autosaved: ok=%v draft=%+v", ok, draft)
	}
}

func TestPublishFailureUnlocksComposer(t *testing.T) {
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/board/Go.json":
			_, _ = w.Write([]byte(`{"name":"Go","allow_post":true}`))
		case "/open/article/Go/post.json":
			posts.Add(1)
			http.Error(w, `{"code":"500","msg":"发布失败"}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	a := composeTestApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageBoard, Name: "Go", Number: 1}}
	a.openCompose("Go", "", "new")
	a.compose.titleEditor.SetText("标题")
	a.compose.contentEditor.SetText("正文")
	a.publish()
	waitForIdle(t, a)
	if a.compose.publishing {
		t.Fatal("composer stayed locked after publish failure")
	}
	if !a.failed || a.message == "" {
		t.Fatalf("publish failure was not shown: failed=%v message=%q", a.failed, a.message)
	}
	if posts.Load() != 1 {
		t.Fatalf("post requests = %d, want 1", posts.Load())
	}
	if got := a.current().Page; got != PageCompose {
		t.Fatalf("failed publish navigated to %v", got)
	}
}

func composeTestApp(t *testing.T, baseURL string) *App {
	t.Helper()
	credentials := &testCredentials{session: application.Credential{Username: "alice", Token: "token", ExpiresAt: time.Now().Add(time.Hour).Unix()}}
	core := newUIFixtureCore(t, baseURL, http.DefaultClient, nil, credentials)
	a, err := New(noopInvalidator{})
	if err != nil {
		t.Fatal(err)
	}
	a.started = true
	a.ready = true
	a.core = core
	return a
}

func assertComposeRenders(t *testing.T, a *App) {
	t.Helper()
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(420, 680)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	dims := a.composePage(gtx)
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("compose rendered blank dimensions: %+v", dims.Size)
	}
}
