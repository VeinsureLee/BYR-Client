package ui

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
)

func waitForThreadExtras(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		a.drain()
		if !a.threadExtrasBusy {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("thread extras did not finish: busy=%t done=%t message=%q", a.threadExtrasBusy, a.threadExtrasDone, a.message)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func renderAppAt(t *testing.T, a *App, width, height int, pxPerSp float32) {
	t.Helper()
	var router input.Router
	var ops op.Ops
	a.Layout(layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(width, height)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: pxPerSp},
	})
}

func TestAndroidQuickThreadSwitchKeepsNewestRoute(t *testing.T) {
	var aRequests, bRequests atomic.Int64
	gate := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/threads/Go/42.json":
			aRequests.Add(1)
			<-gate
			_, _ = w.Write([]byte(`{"id":42,"group_id":42,"title":"主题 A","board_name":"Go","article":[{"id":42,"position":1,"content":"A 正文","user":{"id":"alice"}}]}`))
		case "/open/threads/Go/43.json":
			bRequests.Add(1)
			_, _ = w.Write([]byte(`{"id":43,"group_id":43,"title":"主题 B","board_name":"Go","article":[{"id":43,"position":1,"content":"B 正文","user":{"id":"bob"}}]}`))
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	defer server.Close()
	defer once.Do(func() { close(gate) })
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}

	a.openThread("Go", "42")
	deadline := time.Now().Add(2 * time.Second)
	for aRequests.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("A request did not start")
		}
		time.Sleep(2 * time.Millisecond)
	}
	// A read is interruptible, so opening B does not wait for A.
	a.openThread("Go", "43")
	waitForIdle(t, a)
	if bRequests.Load() == 0 || a.thread.ID != "43" {
		t.Fatalf("B was not applied: requests=%d thread=%q", bRequests.Load(), a.thread.ID)
	}
	once.Do(func() { close(gate) })
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		a.drain()
		time.Sleep(2 * time.Millisecond)
	}
	if a.thread.ID != "43" || a.current().ID != "43" {
		t.Fatalf("stale A replaced B: thread=%q route=%+v", a.thread.ID, a.current())
	}
}

func TestAndroidHistoryFailureKeepsThreadReadable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/open/threads/Go/42.json" {
			_, _ = w.Write([]byte(`{"id":42,"group_id":42,"title":"主题","board_name":"Go","article":[{"id":42,"position":1,"content":"正文仍可阅读","user":{"id":"alice"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":"0000"}`))
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	store, err := local.Open(context.Background(), failingMobileDisk{})
	if err != nil {
		t.Fatal(err)
	}
	a.core.Local = store
	a.stack = []Route{{Page: PageHome}}
	a.openThread("Go", "42")
	waitForIdle(t, a)
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(a.message, "阅读历史保存失败") && time.Now().Before(deadline) {
		a.drain()
		time.Sleep(2 * time.Millisecond)
	}
	if a.failed {
		t.Fatalf("history failure hid the thread: %s", a.message)
	}
	if !strings.Contains(a.message, "阅读历史保存失败") {
		t.Fatalf("history failure message = %q", a.message)
	}
	if a.thread.ID != "42" || len(a.thread.Articles) != 1 {
		t.Fatalf("thread was cleared: %+v", a.thread)
	}
	if !strings.Contains(a.thread.Articles[0].Content, "正文仍可阅读") {
		t.Fatal("body content was lost")
	}
}

func TestAndroidInlineBoldStaysInParagraph(t *testing.T) {
	a := previewApp(t)
	pc := buildThreadContent("你好 [b]北邮[/b]！", nil)
	if len(pc.blocks) != 1 || pc.blocks[0].kind != threadParagraphBlock {
		t.Fatalf("inline paragraph blocks = %+v", pc.blocks)
	}
	if _, simple := simpleRunStyle(pc.blocks[0]); simple {
		t.Fatal("mixed bold paragraph was treated as a single style")
	}
	var router input.Router
	var ops op.Ops
	dims := a.threadParagraph(layout.Context{
		Ops: &ops, Source: router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(320, 200)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}, pc, 0)
	if dims.Size.Y > 40 {
		t.Fatalf("mixed inline paragraph height = %d, want one line", dims.Size.Y)
	}
}

func TestAndroidThreadAnchorSurvivesWidthAndFontChange(t *testing.T) {
	a := previewApp(t)
	a.thread = longThreadFixture(20)
	a.prepareThreadContent(a.thread)
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "1", Number: 1}}
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	state := a.mobileList(key)
	state.Position.First = 8
	renderAppAt(t, a, 420, 700, 1)
	if got := state.Position.First; got != 8 {
		t.Fatalf("initial first row = %d, want 8", got)
	}
	renderAppAt(t, a, 840, 700, 1)
	if got := state.Position.First; got != 8 {
		t.Fatalf("first row after width change = %d, want 8", got)
	}
	renderAppAt(t, a, 840, 700, 1.3)
	if got := state.Position.First; got != 8 {
		t.Fatalf("first row after font change = %d, want 8", got)
	}
}

func TestAndroidThreadTouchTargetsMeet48dp(t *testing.T) {
	a := previewApp(t)
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops: &ops, Source: router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(420, 700)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	icon, _ := a.iconButton(gtx, "touch-icon", "star")
	action, _ := a.compactIconAction(gtx, "touch-action", "赞", "like", "like_on", 2, false, false)
	pill, _ := a.pill(gtx, "touch-pill", "收藏", "star", false, false)
	for name, dims := range map[string]image.Point{"icon": icon.Size, "action": action.Size, "pill": pill.Size} {
		if dims.X < 48 || dims.Y < 48 {
			t.Fatalf("%s touch target = %v, want at least 48x48dp", name, dims)
		}
	}
}

type failingMobileDisk struct{}

func (failingMobileDisk) Load(context.Context) ([]byte, error) { return nil, nil }
func (failingMobileDisk) Save(context.Context, []byte) error   { return errors.New("disk full") }

func TestAndroidGIFSchedulerStopsWhenNothingIsVisible(t *testing.T) {
	a := previewApp(t)
	defer a.stopImageAnimations()
	entry := &mobileImage{frames: []image.Image{image.NewRGBA(image.Rect(0, 0, 2, 2)), image.NewRGBA(image.Rect(0, 0, 2, 2))}}
	entry.img = entry.frames[0]
	a.beginImageLayoutFrame()
	a.markImageVisible(entry)
	a.commitImageAnimations()
	if a.animationTimer == nil || len(a.visibleAnimations) != 1 {
		t.Fatalf("visible GIF scheduler = timer:%v entries:%d", a.animationTimer, len(a.visibleAnimations))
	}
	a.beginImageLayoutFrame()
	a.commitImageAnimations()
	if a.animationTimer != nil || len(a.visibleAnimations) != 0 {
		t.Fatalf("hidden GIF kept redrawing: timer:%v entries:%d", a.animationTimer, len(a.visibleAnimations))
	}
}

func TestAndroidGIFSchedulerCapsVisibleWork(t *testing.T) {
	a := previewApp(t)
	defer a.stopImageAnimations()
	a.beginImageLayoutFrame()
	for i := 0; i < maxVisibleAnimatedGIFs+3; i++ {
		frame := image.NewRGBA(image.Rect(0, 0, 2, 2))
		a.markImageVisible(&mobileImage{img: frame, frames: []image.Image{frame, frame}})
	}
	a.commitImageAnimations()
	if len(a.visibleAnimations) != maxVisibleAnimatedGIFs {
		t.Fatalf("animated GIFs = %d, want cap %d", len(a.visibleAnimations), maxVisibleAnimatedGIFs)
	}
}

func TestAndroidImagePlaceholderReservesLayout(t *testing.T) {
	a := previewApp(t)
	a.images["loading"] = &mobileImage{loading: true}
	var router input.Router
	var ops op.Ops
	dims := a.inlineImage(layout.Context{
		Ops: &ops, Source: router.Source(),
		Constraints: layout.Constraints{Max: image.Pt(300, 500)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}, "loading")
	if dims.Size != image.Pt(300, 225) {
		t.Fatalf("loading placeholder = %v, want 300x225 (4:3 box)", dims.Size)
	}
}

func TestAndroidThreadAnchorSurvivesMediaLoad(t *testing.T) {
	a := previewApp(t)
	a.thread = longThreadFixture(20)
	a.prepareThreadContent(a.thread)
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "1", Number: 1}}
	state := a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1})
	state.Position.First = 6
	renderAppAt(t, a, 420, 700, 1)
	a.mediaGeneration++
	renderAppAt(t, a, 420, 700, 1)
	if got := state.Position.First; got != 6 {
		t.Fatalf("first row after image load = %d, want 6", got)
	}
}

func TestAndroidThreadRetryUsesOriginalPage(t *testing.T) {
	var mu sync.Mutex
	var pages []string
	fail := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/open/threads/Go/42.json" {
			_, _ = w.Write([]byte(`{"code":"0000"}`))
			return
		}
		mu.Lock()
		pages = append(pages, r.URL.Query().Get("page"))
		failing := fail
		fail = false
		mu.Unlock()
		if failing {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"id":42,"group_id":42,"title":"主题","board_name":"Go","page":2,"pagination":{"page_current_count":2,"page_all_count":3},"article":[{"id":44,"position":31,"content":"第二页正文","user":{"id":"alice"}}]}`))
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 2}}
	a.loadThread()
	waitForIdle(t, a)
	if !a.failed {
		t.Fatal("failed read was not reported")
	}
	a.reloadCurrent()
	waitForIdle(t, a)
	mu.Lock()
	defer mu.Unlock()
	if len(pages) != 2 || pages[0] != "2" || pages[1] != "2" {
		t.Fatalf("retry pages = %v, want [2 2]", pages)
	}
	if a.thread.ID != "42" || len(a.thread.Articles) != 1 {
		t.Fatalf("retry did not load the requested page: %+v", a.thread)
	}
}

func TestAndroidAccountChangeDropsOldThreadResult(t *testing.T) {
	gate := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/open/threads/Go/42.json" {
			<-gate
			_, _ = w.Write([]byte(`{"id":42,"group_id":42,"title":"旧会话主题","board_name":"Go","article":[{"id":42,"position":1,"content":"旧会话正文","user":{"id":"alice"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":"0000"}`))
	}))
	defer server.Close()
	defer once.Do(func() { close(gate) })
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openThread("Go", "42")
	deadline := time.Now().Add(2 * time.Second)
	for !a.busy && time.Now().Before(deadline) {
		a.drain()
		time.Sleep(2 * time.Millisecond)
	}
	if err := a.core.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(gate) })
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		a.drain()
		time.Sleep(2 * time.Millisecond)
	}
	if a.thread.ID == "42" {
		t.Fatal("old account result was applied after logout")
	}
}

func TestAndroidExtrasSwitchFromVotedToUnvotedThread(t *testing.T) {
	var vote43 atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/threads/Go/42.json":
			_, _ = w.Write([]byte(`{"id":42,"board_name":"Go","article":[{"id":42,"position":1,"content":"A","user":{"id":"alice"}}],"pagination":{"page_current_count":1}}`))
		case "/open/threads/Go/43.json":
			_, _ = w.Write([]byte(`{"id":43,"board_name":"Go","article":[{"id":43,"position":1,"content":"B","user":{"id":"bob"}}],"pagination":{"page_current_count":1}}`))
		case "/open/vote/42.json":
			_, _ = w.Write([]byte(`{"vid":8,"aid":42,"title":"A 投票","options":[{"viid":1,"label":"选项"}]}`))
		case "/open/vote/43.json":
			vote43.Add(1)
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch}}
	a.openThread("Go", "42")
	waitForIdle(t, a)
	a.ensureThreadExtras()
	waitForThreadExtras(t, a)
	if a.threadVote == nil {
		t.Fatal("A vote state was not loaded")
	}
	a.back()
	a.drain()
	a.openThread("Go", "43")
	waitForIdle(t, a)
	a.ensureThreadExtras()
	waitForThreadExtras(t, a)
	if vote43.Load() != 1 {
		t.Fatalf("B vote lookups = %d, want one", vote43.Load())
	}
	if a.threadVote != nil || a.threadBet != nil {
		t.Fatalf("B reused A extras: vote=%+v bet=%+v", a.threadVote, a.threadBet)
	}
}

func TestAndroidExtrasSwitchFromUnvotedToVotedThread(t *testing.T) {
	var vote42 atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/threads/Go/42.json":
			_, _ = w.Write([]byte(`{"id":42,"board_name":"Go","article":[{"id":42,"position":1,"content":"A","user":{"id":"alice"}}],"pagination":{"page_current_count":1}}`))
		case "/open/threads/Go/43.json":
			_, _ = w.Write([]byte(`{"id":43,"board_name":"Go","article":[{"id":43,"position":1,"content":"B","user":{"id":"bob"}}],"pagination":{"page_current_count":1}}`))
		case "/open/vote/42.json":
			vote42.Add(1)
			_, _ = w.Write([]byte(`{"vid":8,"aid":42,"title":"A 投票","options":[{"viid":1,"label":"选项"}]}`))
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch}}
	a.openThread("Go", "43")
	waitForIdle(t, a)
	a.ensureThreadExtras()
	waitForThreadExtras(t, a)
	if a.threadVote != nil {
		t.Fatal("B unexpectedly had a vote")
	}
	a.back()
	a.drain()
	a.openThread("Go", "42")
	waitForIdle(t, a)
	a.ensureThreadExtras()
	waitForThreadExtras(t, a)
	if vote42.Load() != 1 || a.threadVote == nil {
		t.Fatalf("A vote lookups=%d state=%+v, want one and visible", vote42.Load(), a.threadVote)
	}
}

func TestAndroidLateExtrasResponseCannotOverwriteCurrentThread(t *testing.T) {
	var vote42, vote43 atomic.Int64
	gate := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/threads/Go/42.json":
			_, _ = w.Write([]byte(`{"id":42,"board_name":"Go","article":[{"id":42,"position":1,"content":"A","user":{"id":"alice"}}],"pagination":{"page_current_count":1}}`))
		case "/open/threads/Go/43.json":
			_, _ = w.Write([]byte(`{"id":43,"board_name":"Go","article":[{"id":43,"position":1,"content":"B","user":{"id":"bob"}}],"pagination":{"page_current_count":1}}`))
		case "/open/vote/42.json":
			vote42.Add(1)
			<-gate
			_, _ = w.Write([]byte(`{"vid":8,"aid":42,"title":"A 投票","options":[{"viid":1,"label":"选项"}]}`))
		case "/open/vote/43.json":
			vote43.Add(1)
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		default:
			_, _ = w.Write([]byte(`{"code":"0000"}`))
		}
	}))
	defer server.Close()
	defer once.Do(func() { close(gate) })
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch}}
	a.openThread("Go", "42")
	waitForIdle(t, a)
	a.ensureThreadExtras()
	deadline := time.Now().Add(2 * time.Second)
	for vote42.Load() == 0 {
		a.drain()
		if time.Now().After(deadline) {
			t.Fatal("A vote request did not start")
		}
		time.Sleep(2 * time.Millisecond)
	}
	a.back()
	a.drain()
	a.openThread("Go", "43")
	waitForIdle(t, a)
	a.ensureThreadExtras()
	waitForThreadExtras(t, a)
	if vote43.Load() != 1 || a.threadVote != nil {
		t.Fatalf("B state before A returns: lookups=%d vote=%+v", vote43.Load(), a.threadVote)
	}
	once.Do(func() { close(gate) })
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.drain()
		time.Sleep(2 * time.Millisecond)
	}
	if a.current().ID != "43" || a.threadVote != nil {
		t.Fatalf("late A extras overwrote B: route=%+v vote=%+v", a.current(), a.threadVote)
	}
	key := threadExtrasKey{Generation: a.core.SessionInfo().Generation, Board: "Go", ID: "42"}
	if state := a.threadExtras[key]; state == nil || state.Vote == nil {
		t.Fatalf("A extras were not cached under A's own key: %+v", state)
	}
}

func TestAndroidNarrowMoreMenuExposesEveryAction(t *testing.T) {
	a := previewAppWithCore(t)
	defer a.Close()
	a.thread = sampleThread()
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	a.threadVote = &forum.Vote{ID: "1"}
	a.threadBet = &forum.Bet{ID: "2"}
	renderAppAt(t, a, 320, 760, 1.3)
	a.threadMoreOpen = true
	var router input.Router
	var ops op.Ops
	a.Layout(layout.Context{
		Ops: &ops, Source: router.Source(),
		Constraints: layout.Exact(image.Pt(320, 760)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1.3},
	})
	router.Frame(&ops)
	seen := make(map[string]image.Rectangle)
	for _, node := range router.AppendSemantics(nil) {
		if node.Desc.Gestures&input.ClickGesture != 0 {
			seen[node.Desc.Label] = node.Desc.Bounds
		}
	}
	for _, label := range []string{"复制链接", "刷新主题", "投票", "竞猜", "管理"} {
		bounds := seen[label]
		if bounds.Dx() < 48 || bounds.Dy() < 48 || bounds.Min.X < 0 || bounds.Max.X > 320 {
			t.Errorf("menu %q hit box %v is not >=48x48 inside 320dp; interactive=%+v", label, bounds, seen)
		}
	}
}

func TestAndroidResizeAtThreadHeaderKeepsHeader(t *testing.T) {
	a := previewApp(t)
	a.thread = longThreadFixture(30)
	a.prepareThreadContent(a.thread)
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "1", Number: 1}}
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	state := a.mobileList(key)
	state.Position.First = 8
	renderAppAt(t, a, 420, 700, 1)
	state.Position.First = 0
	state.Position.Offset = 0
	renderAppAt(t, a, 840, 700, 1)
	if state.Position.First != 0 {
		t.Fatalf("resizing at the thread header jumped to row %d", state.Position.First)
	}
}

func TestAndroidAnchorKeepsArticleIDAndOffset(t *testing.T) {
	a := previewApp(t)
	a.thread = longThreadFixture(20)
	a.prepareThreadContent(a.thread)
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "1", Number: 1}}
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	state := a.mobileList(key)
	state.Position.First = 9
	state.Position.Offset = 7
	renderAppAt(t, a, 420, 700, 1)
	wantArticle := string(a.thread.Articles[8].ID)
	anchor := a.threadAnchors[key]
	if anchor.ArticleID != wantArticle || anchor.Offset != 7 {
		t.Fatalf("saved anchor = %+v, want article %s offset 7", anchor, wantArticle)
	}
	renderAppAt(t, a, 840, 700, 1)
	if state.Position.First != 9 || state.Position.Offset != 7 {
		t.Fatalf("restored anchor = row %d offset %d, want row 9 offset 7", state.Position.First, state.Position.Offset)
	}
}

func TestAndroidParagraphPreservesBlankLine(t *testing.T) {
	a := previewApp(t)
	measure := func(body string) int {
		var ops op.Ops
		return a.postBodyContent(layout.Context{
			Ops: &ops, Constraints: layout.Constraints{Max: image.Pt(300, 600)},
			Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		}, buildThreadContent(body, nil)).Size.Y
	}
	adjacent, blank := measure("a\nb"), measure("a\n\nb")
	if blank <= adjacent {
		t.Fatalf("blank line collapsed: adjacent=%d px, with blank=%d px", adjacent, blank)
	}
}

func TestAndroidThemePreferenceAppliesDesignPalette(t *testing.T) {
	a := previewAppWithCore(t)
	defer a.Close()
	defer setPalette(false)
	prefs := a.core.Local.Preferences()
	prefs.Theme = 2
	if err := a.core.Local.SetPreferences(context.Background(), prefs); err != nil {
		t.Fatal(err)
	}
	a.thread = sampleThread()
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	renderAppAt(t, a, 840, 900, 1)
	if !a.dark {
		t.Fatal("theme preference did not select the dark palette")
	}
	wantBg := color.NRGBA{R: 0x11, G: 0x14, B: 0x18, A: 0xFF}
	wantSurface := color.NRGBA{R: 0x19, G: 0x1E, B: 0x25, A: 0xFF}
	wantFg := color.NRGBA{R: 0xEF, G: 0xF2, B: 0xF6, A: 0xFF}
	wantMuted := color.NRGBA{R: 0xBC, G: 0xC7, B: 0xD5, A: 0xFF}
	if colorBg != wantBg || colorSurface != wantSurface || colorFg != wantFg || colorMuted != wantMuted {
		t.Fatalf("dark palette = bg:%v surface:%v fg:%v muted:%v", colorBg, colorSurface, colorFg, colorMuted)
	}
}

func TestAndroidAnchorRestoresAfterRouteReentryAtNewWidth(t *testing.T) {
	a := previewApp(t)
	a.thread = longThreadFixture(20)
	a.prepareThreadContent(a.thread)
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "1", Number: 1}}
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	state := a.mobileList(key)
	state.Position.First = 9
	state.Position.Offset = 7
	renderAppAt(t, a, 420, 700, 1)
	wantArticle := string(a.thread.Articles[8].ID)
	// Change the viewport while another page is showing, then return to the
	// thread. Per-route viewport bookkeeping must re-anchor this route.
	a.stack = append(a.stack, Route{Page: PageLogin})
	renderAppAt(t, a, 840, 700, 1)
	a.stack = a.stack[:len(a.stack)-1]
	renderAppAt(t, a, 840, 700, 1)
	anchor := a.threadAnchors[key]
	if anchor.ArticleID != wantArticle || anchor.Offset != 7 {
		t.Fatalf("re-entry anchor = %+v, want article %s offset 7", anchor, wantArticle)
	}
	if state.Position.First != 9 || state.Position.Offset != 7 {
		t.Fatalf("re-entry restored row %d offset %d, want row 9 offset 7", state.Position.First, state.Position.Offset)
	}
}

func TestAndroidExtrasRetryAfterTransientFailureAndRefresh(t *testing.T) {
	var votes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/threads/Go/42.json":
			_, _ = w.Write([]byte(`{"id":42,"board_name":"Go","article":[{"id":42,"position":1,"content":"post","user":{"id":"alice"}}],"pagination":{"page_current_count":1}}`))
		case "/open/vote/42.json":
			if votes.Add(1) == 1 {
				http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"vid":8,"aid":42,"title":"投票","options":[{"viid":1,"label":"A"}]}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openThread("Go", "42")
	waitForIdle(t, a)
	a.ensureThreadExtras()
	waitForThreadExtras(t, a)
	if a.threadVote != nil || votes.Load() != 1 {
		t.Fatalf("first lookup did not fail transiently: calls=%d vote=%+v", votes.Load(), a.threadVote)
	}
	a.refreshCurrent()
	waitForIdle(t, a)
	a.ensureThreadExtras()
	waitForThreadExtras(t, a)
	if a.threadVote == nil || votes.Load() != 2 {
		t.Fatalf("refresh did not retry failed vote: calls=%d vote=%+v", votes.Load(), a.threadVote)
	}
}

func TestAndroidAnchorRestoresByIDWhenFloorInserted(t *testing.T) {
	a := previewApp(t)
	a.thread = longThreadFixture(30)
	a.prepareThreadContent(a.thread)
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "1", Number: 1}}
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	state := a.mobileList(key)
	state.Position.First = 8
	state.Position.Offset = 7
	renderAppAt(t, a, 420, 700, 1)
	articleID := string(a.thread.Articles[state.Position.First-1].ID)
	a.stack = append(a.stack, Route{Page: PageLogin})
	renderAppAt(t, a, 840, 700, 1)
	a.stack = a.stack[:len(a.stack)-1]
	a.thread.Articles = append([]forum.Article{{ID: "inserted", Position: 1, Content: "new floor"}}, a.thread.Articles...)
	a.prepareThreadContent(a.thread)
	renderAppAt(t, a, 840, 700, 1)
	if got := string(a.thread.Articles[state.Position.First-1].ID); got != articleID {
		t.Fatalf("after insertion row %d shows article %s, want %s", state.Position.First, got, articleID)
	}
	if state.Position.Offset != 7 {
		t.Fatalf("after insertion offset = %d, want 7", state.Position.Offset)
	}
}

func TestAndroidAnchorFallsToAdjacentFloorWhenTargetDeleted(t *testing.T) {
	a := previewApp(t)
	a.thread = longThreadFixture(20)
	a.prepareThreadContent(a.thread)
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "1", Number: 1}}
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	state := a.mobileList(key)
	state.Position.First = 9
	state.Position.Offset = 7
	renderAppAt(t, a, 420, 700, 1)
	deleted := string(a.thread.Articles[8].ID)
	articles := append([]forum.Article(nil), a.thread.Articles[:8]...)
	articles = append(articles, a.thread.Articles[9:]...)
	a.thread.Articles = articles
	a.prepareThreadContent(a.thread)
	renderAppAt(t, a, 420, 700, 1)
	if state.Position.First <= 0 || state.Position.First > len(a.thread.Articles) {
		t.Fatalf("deleted target left invalid row %d for %d floors", state.Position.First, len(a.thread.Articles))
	}
	if got := string(a.thread.Articles[state.Position.First-1].ID); got == deleted {
		t.Fatalf("deleted target %s is still shown", deleted)
	}
	if state.Position.Offset != 7 {
		t.Fatalf("adjacent floor offset = %d, want 7", state.Position.Offset)
	}
}

func TestAndroidMoreMenuScrollsInShortViewportAndManageWorks(t *testing.T) {
	a := previewAppWithCore(t)
	defer a.Close()
	a.thread = sampleThread()
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
	a.threadVote = &forum.Vote{ID: "1"}
	a.threadBet = &forum.Bet{ID: "2"}
	renderAppAt(t, a, 320, 292, 1.3)
	a.threadMoreOpen = true
	a.threadMenuList.Position.First = 4
	var router input.Router
	var ops op.Ops
	a.Layout(layout.Context{
		Ops: &ops, Source: router.Source(),
		Constraints: layout.Exact(image.Pt(320, 292)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1.3},
	})
	router.Frame(&ops)
	found := false
	for _, node := range router.AppendSemantics(nil) {
		if node.Desc.Label != "管理" || node.Desc.Gestures&input.ClickGesture == 0 {
			continue
		}
		bounds := node.Desc.Bounds
		if bounds.Dy() < 48 || bounds.Max.Y > 292 || bounds.Min.Y < 0 {
			t.Fatalf("manage hit box %v is clipped or under 48dp", bounds)
		}
		found = true
	}
	if !found {
		t.Fatal("manage control was not built after scrolling the short menu")
	}
	a.click("thread-manage").Click()
	ops.Reset()
	a.Layout(layout.Context{
		Ops: &ops, Source: router.Source(),
		Constraints: layout.Exact(image.Pt(320, 292)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1.3},
	})
	if got := a.current().Page; got != PageCompose {
		t.Fatalf("manage action page = %v, want compose", got)
	}
}

func TestAndroidBackReloadKeepsFloorAnchorAcrossInsertedRow(t *testing.T) {
	base := longThreadFixture(30)
	var loads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/threads/Go/1.json":
			n := loads.Add(1)
			thread := base
			thread.Articles = append([]forum.Article(nil), base.Articles...)
			if n > 1 {
				thread.Articles = append([]forum.Article{{ID: "inserted", Position: 1, Content: "new floor"}}, thread.Articles...)
			}
			_ = json.NewEncoder(w).Encode(thread)
		case "/open/user/query/alice.json":
			_, _ = w.Write([]byte(`{"id":"alice","user_name":"Alice"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openThread("Go", "1")
	waitForIdle(t, a)
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	state := a.mobileList(key)
	state.Position.First = 8
	state.Position.Offset = 7
	renderAppAt(t, a, 420, 700, 1)
	articleID := string(a.thread.Articles[state.Position.First-1].ID)
	a.openUser("alice")
	waitForIdle(t, a)
	renderAppAt(t, a, 840, 700, 1)
	a.click("back").Click()
	renderAppAt(t, a, 840, 700, 1)
	waitForIdle(t, a)
	renderAppAt(t, a, 840, 700, 1)
	if loads.Load() < 2 {
		t.Fatalf("thread reload did not run: loads=%d", loads.Load())
	}
	if state.Position.First <= 0 || state.Position.First > len(a.thread.Articles) {
		t.Fatalf("back reload row = %d for %d floors", state.Position.First, len(a.thread.Articles))
	}
	if got := string(a.thread.Articles[state.Position.First-1].ID); got != articleID {
		t.Fatalf("back reload row %d shows article %s, want %s", state.Position.First, got, articleID)
	}
	if state.Position.Offset != 7 {
		t.Fatalf("back reload offset = %d, want 7", state.Position.Offset)
	}
}

func TestAndroidExplicitRefreshSuccessStartsAtTopWithInsertedRow(t *testing.T) {
	base := longThreadFixture(30)
	var loads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/open/threads/Go/1.json" {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		n := loads.Add(1)
		thread := base
		thread.Articles = append([]forum.Article(nil), base.Articles...)
		if n > 1 {
			thread.Articles = append([]forum.Article{{ID: "inserted", Position: 1, Content: "new floor"}}, thread.Articles...)
		}
		_ = json.NewEncoder(w).Encode(thread)
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openThread("Go", "1")
	waitForIdle(t, a)
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	state := a.mobileList(key)
	state.Position.First = 8
	state.Position.Offset = 7
	renderAppAt(t, a, 420, 700, 1)
	if seed := a.threadAnchors[key]; seed.ArticleID == "" {
		t.Fatalf("anchor fixture = %+v", seed)
	}
	a.click("thread-more").Click()
	renderAppAt(t, a, 420, 700, 1)
	a.click("thread-refresh").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	if loads.Load() < 2 {
		t.Fatalf("explicit refresh loads = %d, want two", loads.Load())
	}
	if state.Position.First != 0 || state.Position.Offset != 0 {
		t.Fatalf("after explicit refresh row/offset = %d/%d, want 0/0", state.Position.First, state.Position.Offset)
	}
	anchor := a.threadAnchors[key]
	if anchor.Row != 0 || anchor.ArticleID != "" {
		t.Fatalf("after explicit refresh anchor = %+v, want top", anchor)
	}
	if a.current().Sub != "" {
		t.Fatalf("refresh kept reminder target %q", a.current().Sub)
	}
}

func TestAndroidReminderRefreshSuccessClearsTargetAndReturnsToTop(t *testing.T) {
	base := longThreadFixture(30)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/threads/Go/1.json":
			_ = json.NewEncoder(w).Encode(base)
		case "/open/user/query/alice.json":
			_, _ = w.Write([]byte(`{"id":"alice","user_name":"Alice"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "1", Sub: "8", Number: 1}}
	a.loadThread()
	waitForIdle(t, a)
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Sub: "8", Number: 1}
	state := a.mobileList(key)
	if state.Position.First != 8 {
		t.Fatalf("reminder target row = %d, want 8", state.Position.First)
	}
	renderAppAt(t, a, 420, 700, 1)
	a.click("thread-more").Click()
	renderAppAt(t, a, 420, 700, 1)
	a.click("thread-refresh").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	if a.current().Sub != "" {
		t.Fatalf("refresh kept reminder target %q", a.current().Sub)
	}
	topKey := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	topState := a.mobileList(topKey)
	if topState.Position.First != 0 || topState.Position.Offset != 0 {
		t.Fatalf("refresh top row/offset = %d/%d, want 0/0", topState.Position.First, topState.Position.Offset)
	}
	a.openUser("alice")
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	a.click("back").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	topState = a.mobileList(topKey)
	if topState.Position.First != 0 || topState.Position.Offset != 0 {
		t.Fatalf("back from profile row/offset = %d/%d, want 0/0", topState.Position.First, topState.Position.Offset)
	}
}

func TestAndroidExplicitRefreshFailureKeepsPositionAndRetryStartsAtTop(t *testing.T) {
	base := longThreadFixture(30)
	var loads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/open/threads/Go/1.json" {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		n := loads.Add(1)
		if n == 2 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(base)
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openThread("Go", "1")
	waitForIdle(t, a)
	key := listKey{Page: PageThread, Name: "Go", ID: "1", Number: 1}
	state := a.mobileList(key)
	state.Position.First = 8
	state.Position.Offset = 7
	renderAppAt(t, a, 420, 700, 1)
	a.click("thread-more").Click()
	renderAppAt(t, a, 420, 700, 1)
	a.click("thread-refresh").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	if !a.failed {
		t.Fatal("explicit refresh failure was not reported")
	}
	if state.Position.First != 8 || state.Position.Offset != 7 {
		t.Fatalf("failed refresh position = %d/%d, want 8/7", state.Position.First, state.Position.Offset)
	}
	if a.threadRetryPage != 1 || !a.threadRetryRefresh {
		t.Fatalf("refresh retry intent = %d/%t, want page 1 refresh", a.threadRetryPage, a.threadRetryRefresh)
	}
	a.click("retry-status").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	if loads.Load() != 3 {
		t.Fatalf("refresh/retry loads = %d, want 3", loads.Load())
	}
	if state.Position.First != 0 || state.Position.Offset != 0 {
		t.Fatalf("refresh retry position = %d/%d, want 0/0", state.Position.First, state.Position.Offset)
	}
}

func TestAndroidAliasRefreshFailureKeepsCanonicalThreadContent(t *testing.T) {
	const board = "悄悄话"
	const endpoint = "/open/threads/" + board + "/1.json"
	base := longThreadFixture(30)
	base.Board = "IWhisper"
	var loads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != endpoint {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		if loads.Add(1) == 2 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(base)
	}))
	defer server.Close()
	a := interactionApp(t, server.URL)
	defer a.Close()
	a.stack = []Route{{Page: PageHome}}
	a.openThread(board, "1")
	waitForIdle(t, a)
	key := listKey{Page: PageThread, Name: board, ID: "1", Number: 1}
	state := a.mobileList(key)
	state.Position.First = 8
	state.Position.Offset = 7
	renderAppAt(t, a, 420, 700, 1)
	if a.threadLoadedRoute.Name != "IWhisper" || a.threadLoadedRequest.Name != board {
		t.Fatalf("loaded route=%+v request=%+v", a.threadLoadedRoute, a.threadLoadedRequest)
	}
	a.click("thread-more").Click()
	renderAppAt(t, a, 420, 700, 1)
	a.click("thread-refresh").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	if !a.failed || len(a.thread.Articles) != 30 || state.Position.First != 8 || state.Position.Offset != 7 {
		t.Fatalf("alias failed refresh: failed=%t articles=%d row=%d offset=%d", a.failed, len(a.thread.Articles), state.Position.First, state.Position.Offset)
	}
	a.click("retry-status").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	renderAppAt(t, a, 420, 700, 1)
	if state.Position.First != 0 || state.Position.Offset != 0 {
		t.Fatalf("alias refresh retry row/offset = %d/%d, want 0/0", state.Position.First, state.Position.Offset)
	}
}
