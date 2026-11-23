package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/VeinsureLee/BYR-Client/core/local"
	"github.com/egoist/mygo/ui"
)

func TestDesktopThreadReadRecordsHistoryAfterBody(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "thread", Name: "Go", ID: "42", Number: 1})
	h.has("你好 北邮")
	deadline := time.Now().Add(2 * time.Second)
	for {
		entries := h.app.Core.Local.History("alice")
		if len(entries) == 1 {
			if entries[0].Board != "Go" || entries[0].ID != "42" || entries[0].Title == "" {
				t.Fatalf("history entry = %+v", entries[0])
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("history was not recorded: %+v", entries)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDesktopQuickThreadSwitchKeepsNewestRoute(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.mu.Lock()
	h.overrides = map[string]string{
		"/open/threads/Go/42.json": `{"id":42,"group_id":42,"title":"主题 A","board_name":"Go","article":[{"id":42,"position":1,"content":"A 正文","user":{"id":"alice"}}]}`,
		"/open/threads/Go/43.json": `{"id":43,"group_id":43,"title":"主题 B","board_name":"Go","article":[{"id":43,"position":1,"content":"B 正文","user":{"id":"bob"}}]}`,
	}
	gate := make(chan struct{})
	h.gates = map[string]chan struct{}{"/open/threads/Go/42.json": gate}
	h.mu.Unlock()

	h.app.navigate(Route{Page: "thread", Name: "Go", ID: "42", Number: 1}, true)
	deadline := time.Now().Add(2 * time.Second)
	for h.count("GET /open/threads/Go/42.json") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("A request did not start")
		}
		time.Sleep(2 * time.Millisecond)
	}
	// A page read is interruptible: opening B cancels A and must not be
	// blocked by the global busy flag.
	h.app.navigate(Route{Page: "thread", Name: "Go", ID: "43", Number: 1}, true)
	h.idle()
	h.view.Frame()
	if h.app.route.ID != "43" || h.app.thread.ID != "43" {
		t.Fatalf("route/thread after B = %+v/%q, want 43", h.app.route, h.app.thread.ID)
	}
	close(gate)
	h.idle()
	h.view.Frame()
	if h.app.route.ID != "43" || h.app.thread.ID != "43" {
		t.Fatalf("stale A replaced B: route=%+v thread=%q", h.app.route, h.app.thread.ID)
	}
	h.has("B 正文")
	if h.view.HasText("A 正文") {
		t.Fatal("stale A body was shown")
	}
}

func TestDesktopHistoryFailureKeepsThreadReadable(t *testing.T) {
	h := newHarness(t)
	h.login()
	store, err := local.Open(context.Background(), failingDisk{})
	if err != nil {
		t.Fatal(err)
	}
	h.app.Core.Local = store
	h.app.navigate(Route{Page: "thread", Name: "Go", ID: "42", Number: 1}, true)
	h.idle()
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(h.app.historyNotice, "阅读历史保存失败") && time.Now().Before(deadline) {
		h.idle()
		h.view.Frame()
		time.Sleep(2 * time.Millisecond)
	}
	if h.app.failed {
		t.Fatalf("history failure hid the thread: %s", h.app.message)
	}
	if !strings.Contains(h.app.historyNotice, "阅读历史保存失败") {
		t.Fatalf("history failure notice = %q", h.app.historyNotice)
	}
	h.has("阅读历史保存失败")
	h.has("你好 北邮")
}

func TestDesktopThreadKeepsFloorAcrossWidthChange(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "thread", Name: "Go", ID: "fixture", Number: 1}
	h.app.thread = desktopLongThread(24)
	h.app.prepareThreadModels(h.app.thread)
	h.view.SetSize(1100, 300)
	h.view.Frame()
	state := h.app.desktopListState(h.app.threadListKey())
	state.ScrollTo(8, ui.Start)
	h.view.Frame()
	first, _ := state.Visible()
	if first != 8 {
		t.Fatalf("first visible floor = %d, want 8", first)
	}
	h.view.SetSize(720, 300)
	h.view.Frame()
	got, _ := state.Visible()
	if got != first {
		t.Fatalf("floor after width change = %d, want %d", got, first)
	}
}

func TestDesktopThreadTitleAppearsOnce(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "thread", Name: "Go", ID: "42", Number: 1}
	h.app.thread = forumThreadFixture("测试主题")
	h.view.SetSize(1100, 760)
	h.view.Frame()
	count := 0
	for _, text := range h.view.Texts() {
		if text == "测试主题" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("complete title appears %d times: %q", count, h.view.Texts())
	}
}

func TestDesktopThreadContentParsingIsColdOnScroll(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "thread", Name: "Go", ID: "fixture", Number: 1}
	h.app.thread = desktopLongThread(30)
	h.app.prepareThreadModels(h.app.thread)
	h.view.SetSize(1100, 320)
	h.view.Frame()
	state := h.app.desktopListState(h.app.threadListKey())
	before := content.ParseCalls.Load()
	state.ScrollTo(12, ui.Start)
	h.view.Frame()
	if delta := content.ParseCalls.Load() - before; delta != 0 {
		t.Fatalf("scroll parsed %d unchanged floors", delta)
	}
}

func forumThreadFixture(title string) forum.Thread {
	thread := desktopLongThread(2)
	thread.Title = title
	thread.Articles[0].Title = title
	return thread
}

type failingDisk struct{}

func (failingDisk) Load(context.Context) ([]byte, error) { return nil, nil }
func (failingDisk) Save(context.Context, []byte) error   { return errors.New("disk full") }

func TestDesktopThreadFitsWindowMatrix(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.app.route = Route{Page: "thread", Name: "Go", ID: "fixture", Number: 1}
	h.app.back = []Route{{Page: "board", Name: "Go", Number: 1}}
	h.app.thread = desktopLongThread(20)
	h.app.prepareThreadModels(h.app.thread)
	for _, size := range [][2]int{{720, 480}, {1100, 760}, {1440, 900}} {
		h.view.SetSize(size[0], size[1])
		h.view.Frame()
		for _, label := range []string{"返回", "收藏文章", "复制链接", "下一页"} {
			rect, ok := h.view.Find(label)
			if !ok {
				t.Fatalf("%dx%d: missing %q", size[0], size[1], label)
			}
			if rect.X < 0 || rect.Y < 0 || rect.W <= 0 || rect.H <= 0 ||
				rect.X+rect.W > float32(size[0])+1 || rect.Y+rect.H > float32(size[1])+1 {
				t.Fatalf("%dx%d: %q outside window at %+v", size[0], size[1], label, rect)
			}
		}
	}
}

func boardFixture(n int) string {
	var articles []string
	for i := 0; i < n; i++ {
		articles = append(articles, fmt.Sprintf(`{"id":%d,"group_id":%d,"board_name":"Go","title":"主题 %d","user":{"id":"alice"}}`, 42+i, 42+i, i))
	}
	return fmt.Sprintf(`{"name":"Go","description":"Go 讨论","article":[%s],"pagination":{"page_all_count":1,"page_current_count":1,"item_all_count":%d}}`, strings.Join(articles, ","), n)
}

func TestDesktopBackButtonLeavesBlockedThreadRead(t *testing.T) {
	h := newHarness(t)
	h.login()
	gate := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(gate) })
	h.mu.Lock()
	h.overrides = map[string]string{"/open/board/Go.json": boardFixture(20)}
	h.gates = map[string]chan struct{}{"/open/threads/Go/47.json": gate}
	h.mu.Unlock()
	h.goTo(Route{Page: "board", Name: "Go", Number: 1})
	h.view.SetSize(1100, 600)
	h.view.Frame()
	state := h.app.desktopListState(h.app.articleListKey())
	state.ScrollTo(5, ui.Start)
	h.view.Frame()
	first, _ := state.Visible()
	if first != 5 {
		t.Fatalf("board first row = %d, want 5", first)
	}
	if err := h.view.Click("主题 5"); err != nil {
		t.Fatal(err)
	}
	h.waitFor(func() bool { return h.count("GET /open/threads/Go/47.json") == 1 })
	h.view.Frame()
	if !h.app.busy || !h.app.pageLoading {
		t.Fatalf("thread read is not pending: busy=%t pageLoading=%t", h.app.busy, h.app.pageLoading)
	}
	if err := h.view.Click("返回"); err != nil {
		t.Fatalf("visible return button inaccessible during read: %v", err)
	}
	h.idle()
	if h.app.route.Page != "board" {
		t.Fatalf("return left route on %q", h.app.route.Page)
	}
	if got, _ := state.Visible(); got != 5 {
		t.Fatalf("board position after return = %d, want 5", got)
	}
	once.Do(func() { close(gate) })
	h.idle()
	if h.app.route.Page != "board" || h.app.thread.ID != "" {
		t.Fatalf("late thread result was applied: route=%+v thread=%q", h.app.route, h.app.thread.ID)
	}
}

func TestDesktopBackButtonLeavesBlockedVotePreflight(t *testing.T) {
	h := newHarness(t)
	h.login()
	gate := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(gate) })
	h.mu.Lock()
	h.gates = map[string]chan struct{}{"/open/vote/42.json": gate}
	h.mu.Unlock()
	h.app.navigate(Route{Page: "board", Name: "Go", Number: 1}, true)
	h.idle()
	h.app.navigate(Route{Page: "thread", Name: "Go", ID: "42", Number: 1}, true)
	h.waitFor(func() bool { return h.app.thread.ID == "42" && h.count("GET /open/vote/42.json") == 1 })
	if h.app.busy {
		t.Fatal("vote preflight still owns the global busy flag")
	}
	h.view.Frame()
	if err := h.view.Click("返回"); err != nil {
		t.Fatalf("return button inaccessible during vote preflight: %v", err)
	}
	h.idle()
	if h.app.route.Page != "board" {
		t.Fatalf("return during preflight left route on %q", h.app.route.Page)
	}
	once.Do(func() { close(gate) })
	h.idle()
	if h.app.threadVoteAvailable || h.app.route.Page != "board" {
		t.Fatalf("late vote result leaked: available=%t route=%q", h.app.threadVoteAvailable, h.app.route.Page)
	}
}

func TestDesktopFailedPageKeepsDisplayedPageAndRetriesRequestedPage(t *testing.T) {
	h := newHarness(t)
	h.login()
	page1 := `{"id":42,"group_id":42,"title":"主题","board_name":"Go","article":[{"id":42,"position":1,"content":"第一页正文","user":{"id":"alice"}}],"pagination":{"page_all_count":2,"page_current_count":1}}`
	page2 := `{"id":42,"group_id":42,"title":"主题","board_name":"Go","article":[{"id":44,"position":31,"content":"第二页正文","user":{"id":"bob"}}],"pagination":{"page_all_count":2,"page_current_count":2}}`
	h.mu.Lock()
	h.overrides = map[string]string{"/open/threads/Go/42.json": page1}
	h.mu.Unlock()
	h.goTo(Route{Page: "thread", Name: "Go", ID: "42", Number: 1})
	if len(h.app.thread.Articles) != 1 || h.app.thread.Articles[0].ID != "42" {
		t.Fatalf("page one fixture = %+v", h.app.thread.Articles)
	}
	h.mu.Lock()
	h.overrides["/open/threads/Go/42.json"] = "{invalid"
	h.mu.Unlock()
	h.app.pageNumber(2)
	h.idle()
	h.view.Frame()
	if h.app.route.Number != 1 || h.app.threadRequestPage != 2 {
		t.Fatalf("displayed/requested page = %d/%d, want 1/2", h.app.route.Number, h.app.threadRequestPage)
	}
	if h.app.threadDisplayPage() != 1 || h.app.threadLoadedRoute.Number != 1 {
		t.Fatalf("displayed page = %d loaded=%d, want old page 1", h.app.threadDisplayPage(), h.app.threadLoadedRoute.Number)
	}
	if len(h.app.thread.Articles) != 1 || h.app.thread.Articles[0].ID != "42" {
		t.Fatalf("old page floors were relabelled: %+v", h.app.thread.Articles)
	}
	if !h.view.HasText("第 1 / 2 页") {
		t.Fatalf("pager does not show displayed page: %q", h.view.Texts())
	}
	h.mu.Lock()
	h.overrides["/open/threads/Go/42.json"] = page2
	h.mu.Unlock()
	h.click("重试")
	if h.app.threadLoadedRoute.Number != 2 || h.app.threadDisplayPage() != 2 {
		t.Fatalf("retry loaded page = %d displayed=%d, want 2", h.app.threadLoadedRoute.Number, h.app.threadDisplayPage())
	}
	if len(h.app.thread.Articles) != 1 || h.app.thread.Articles[0].ID != "44" {
		t.Fatalf("retry page floors = %+v, want floor 44", h.app.thread.Articles)
	}
	if !h.view.HasText("第 2 / 2 页") {
		t.Fatalf("pager after retry = %q", h.view.Texts())
	}
}

func TestDesktopLongTitleScrollsWithFloorsAtNarrowWindow(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "thread", Name: "Go", ID: "42", Number: 1})
	prefs := h.app.Core.Local.Preferences()
	prefs.FontScale = 1.4
	if err := h.app.Core.Local.SetPreferences(context.Background(), prefs); err != nil {
		t.Fatal(err)
	}
	h.app.thread.Title = strings.Repeat("测试", 80)
	h.view.SetSize(720, 480)
	h.view.Frame()
	state := h.app.desktopListState(h.app.threadListKey())
	state.ScrollTo(2, ui.Start)
	h.view.Frame()
	if rect, ok := h.view.Find("回复正文"); !ok || rect.Y < 0 || rect.Y >= 480 {
		t.Fatalf("floor body inaccessible under a long title at 720x480: rect=%+v ok=%t", rect, ok)
	}
}

func TestDesktopFailedThreadRefreshDoesNotReportSuccessOnBack(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "board", Name: "Go", Number: 1})
	h.mu.Lock()
	h.overrides = map[string]string{
		"/open/threads/Go/42.json": `{"id":42,"board_name":"Go","article":[{"id":43,"position":51,"content":"page two"}],"pagination":{"page_current_count":2,"page_all_count":2}}`,
	}
	h.mu.Unlock()
	h.app.navigate(Route{Page: "thread", Name: "Go", ID: "42", Number: 2}, true)
	h.idle()
	if h.app.threadDisplayPage() != 2 {
		t.Fatalf("initial displayed page = %d, want 2", h.app.threadDisplayPage())
	}
	h.mu.Lock()
	h.overrides["/open/threads/Go/42.json"] = "{invalid"
	h.mu.Unlock()
	h.view.Frame()
	if err := h.view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	if !h.app.failed {
		t.Fatal("thread refresh failure did not reach the UI")
	}
	h.view.Frame()
	if err := h.view.Click("返回"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	if h.app.route.Page != "board" {
		t.Fatalf("left on %q, want board", h.app.route.Page)
	}
	if h.app.message == "刷新成功" {
		t.Fatal("failed thread refresh was reported successful after board loaded")
	}
}

func TestDesktopThreadRefreshSuccessAndRetrySettleOwnRequest(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "thread", Name: "Go", ID: "42", Number: 1})
	h.view.Frame()
	if err := h.view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	if h.app.message != "刷新成功" {
		t.Fatalf("successful thread refresh message = %q", h.app.message)
	}
	h.mu.Lock()
	h.overrides = map[string]string{"/open/threads/Go/42.json": "{invalid"}
	h.mu.Unlock()
	h.app.route.Number = 2
	h.app.threadRequestPage = 2
	h.view.Frame()
	if err := h.view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	if h.app.route.Number != 2 || h.app.threadRequestPage != 1 {
		t.Fatalf("displayed/request page after failed refresh = %d/%d, want 2/1", h.app.route.Number, h.app.threadRequestPage)
	}
	h.mu.Lock()
	h.overrides["/open/threads/Go/42.json"] = `{"id":42,"board_name":"Go","article":[{"id":44,"position":1,"content":"retried"}],"pagination":{"page_current_count":1,"page_all_count":1}}`
	h.mu.Unlock()
	h.view.Frame()
	if err := h.view.Click("重试"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	if h.app.threadRequestPage != 1 || len(h.app.thread.Articles) != 1 || h.app.thread.Articles[0].ID != "44" {
		t.Fatalf("retry page/floors = %d/%+v", h.app.threadRequestPage, h.app.thread.Articles)
	}
	if h.app.message != "刷新成功" {
		t.Fatalf("refresh retry success message = %q", h.app.message)
	}
}

func TestDesktopLeavingThreadDuringRefreshDoesNotReportSuccess(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.goTo(Route{Page: "thread", Name: "Go", ID: "42", Number: 1})
	gate := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(gate) })
	h.mu.Lock()
	h.gates = map[string]chan struct{}{"/open/threads/Go/42.json": gate}
	h.mu.Unlock()
	h.view.Frame()
	if err := h.view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for h.count("GET /open/threads/Go/42.json") < 2 {
		if time.Now().After(deadline) {
			t.Fatal("refresh request did not start")
		}
		h.view.Frame()
		time.Sleep(2 * time.Millisecond)
	}
	if err := h.view.Click("返回"); err != nil {
		t.Fatalf("back inaccessible during refresh: %v", err)
	}
	h.idle()
	if h.app.route.Page == "thread" {
		t.Fatalf("still on thread %+v", h.app.route)
	}
	once.Do(func() { close(gate) })
	h.idle()
	if h.app.message == "刷新成功" {
		t.Fatal("cancelled refresh reported success")
	}
}

func TestDesktopReturningFromProfileAfterFailedRefreshKeepsDisplayedPage(t *testing.T) {
	h := newHarness(t)
	h.login()
	page2 := `{"id":42,"board_name":"Go","article":[{"id":43,"position":51,"content":"page two"}],"pagination":{"page_current_count":2,"page_all_count":2}}`
	h.mu.Lock()
	h.overrides = map[string]string{
		"/open/threads/Go/42.json":    page2,
		"/open/user/query/alice.json": `{"id":"alice","user_name":"Alice"}`,
	}
	h.mu.Unlock()
	h.app.navigate(Route{Page: "thread", Name: "Go", ID: "42", Number: 2}, true)
	h.idle()
	if h.app.threadDisplayPage() != 2 {
		t.Fatalf("initial page = %d", h.app.threadDisplayPage())
	}
	h.mu.Lock()
	h.overrides["/open/threads/Go/42.json"] = "{invalid"
	h.mu.Unlock()
	h.view.Frame()
	if err := h.view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	if h.app.route.Number != 2 || h.app.threadRequestPage != 1 {
		t.Fatalf("after failed refresh route/request = %d/%d, want 2/1", h.app.route.Number, h.app.threadRequestPage)
	}
	h.app.navigate(Route{Page: "user", Name: "alice"}, true)
	h.idle()
	h.goToThreadFromProfileForTest(t)
	if h.app.route.Number != 2 || h.app.threadRequestPage != 2 || h.app.threadLoadedRoute.Number != 2 {
		t.Fatalf("profile return route/request/loaded = %d/%d/%d, want 2/2/2", h.app.route.Number, h.app.threadRequestPage, h.app.threadLoadedRoute.Number)
	}
	if h.app.message == "刷新成功" {
		t.Fatal("failed refresh was reported successful after profile return")
	}
}

func (h *harness) goToThreadFromProfileForTest(t *testing.T) {
	t.Helper()
	h.view.Frame()
	h.click("返回")
	h.view.Frame()
}

func TestDesktopFailedPageOneRefreshKeepsFloorAndOffset(t *testing.T) {
	h := newHarness(t)
	h.login()
	const endpoint = "/open/threads/Go/fixture.json"
	data, err := json.Marshal(desktopLongThread(30))
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.overrides = map[string]string{endpoint: string(data)}
	h.mu.Unlock()
	h.goTo(Route{Page: "thread", Name: "Go", ID: "fixture", Number: 1})
	h.view.SetSize(1100, 760)
	h.view.Frame()
	state := h.app.desktopListState(h.app.threadListKey())
	state.ScrollTo(8, ui.Start)
	h.view.Frame()
	if first, _ := state.Visible(); first != 8 {
		t.Fatalf("pre-refresh first row = %d, want 8", first)
	}
	h.mu.Lock()
	h.overrides[endpoint] = "{invalid"
	h.mu.Unlock()
	if err := h.view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	h.view.Frame()
	if !h.app.failed || len(h.app.thread.Articles) == 0 {
		t.Fatal("failed refresh did not preserve the loaded thread")
	}
	if first, _ := state.Visible(); first != 8 {
		t.Fatalf("failed refresh first row = %d, want 8", first)
	}
}

func TestDesktopSuccessfulRefreshClearsReminderTargetAndStartsAtTop(t *testing.T) {
	h := newHarness(t)
	h.login()
	const endpoint = "/open/threads/Go/fixture.json"
	data, err := json.Marshal(desktopLongThread(30))
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.overrides = map[string]string{endpoint: string(data)}
	h.mu.Unlock()
	h.goTo(Route{Page: "thread", Name: "Go", ID: "fixture", Sub: "h", Number: 1})
	h.view.SetSize(1100, 760)
	h.view.Frame()
	state := h.app.desktopListState(h.app.threadListKey())
	if first, _ := state.Visible(); first != 8 {
		t.Fatalf("reminder target row = %d, want 8", first)
	}
	if err := h.view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	h.view.Frame()
	if first, _ := state.Visible(); first != 0 {
		t.Fatalf("successful refresh old-key row = %d, want 0", first)
	}
	if h.app.route.Sub != "" {
		t.Fatalf("successful refresh kept reminder target %q", h.app.route.Sub)
	}
	h.app.navigate(Route{Page: "user", Name: "alice"}, true)
	h.idle()
	h.click("返回")
	h.view.Frame()
	state = h.app.desktopListState(h.app.threadListKey())
	if first, _ := state.Visible(); first != 0 {
		t.Fatalf("return after successful refresh row = %d, want 0", first)
	}
	if h.app.route.Sub != "" {
		t.Fatalf("return restored stale reminder target %q", h.app.route.Sub)
	}
}

func TestDesktopReminderRefreshRetryClearsTargetAfterFailure(t *testing.T) {
	h := newHarness(t)
	h.login()
	const endpoint = "/open/threads/Go/fixture.json"
	data, err := json.Marshal(desktopLongThread(30))
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.overrides = map[string]string{endpoint: string(data)}
	h.mu.Unlock()
	h.goTo(Route{Page: "thread", Name: "Go", ID: "fixture", Sub: "h", Number: 1})
	h.view.SetSize(1100, 760)
	h.view.Frame()
	state := h.app.desktopListState(h.app.threadListKey())
	if first, _ := state.Visible(); first != 8 {
		t.Fatalf("reminder target row = %d, want 8", first)
	}
	h.mu.Lock()
	h.overrides[endpoint] = "{invalid"
	h.mu.Unlock()
	if err := h.view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	h.view.Frame()
	if !h.app.failed || h.app.route.Sub != "h" {
		t.Fatalf("failed reminder refresh state: failed=%t sub=%q", h.app.failed, h.app.route.Sub)
	}
	if first, _ := state.Visible(); first != 8 {
		t.Fatalf("failed reminder refresh row = %d, want 8", first)
	}
	h.mu.Lock()
	h.overrides[endpoint] = string(data)
	h.mu.Unlock()
	if err := h.view.Click("重试"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	h.view.Frame()
	if h.app.route.Sub != "" {
		t.Fatalf("refresh retry kept target %q", h.app.route.Sub)
	}
	state = h.app.desktopListState(h.app.threadListKey())
	if first, _ := state.Visible(); first != 0 {
		t.Fatalf("refresh retry row = %d, want 0", first)
	}
}

func TestDesktopAliasRefreshFailureKeepsCanonicalThreadContent(t *testing.T) {
	h := newHarness(t)
	h.login()
	const board = "悄悄话"
	const endpoint = "/open/threads/" + board + "/fixture.json"
	thread := desktopLongThread(30)
	thread.Board = "IWhisper"
	data, err := json.Marshal(thread)
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.overrides = map[string]string{endpoint: string(data)}
	h.mu.Unlock()
	h.goTo(Route{Page: "thread", Name: board, ID: "fixture", Number: 1})
	h.view.SetSize(1100, 760)
	state := h.app.desktopListState(h.app.threadListKey())
	state.ScrollTo(8, ui.Start)
	h.view.Frame()
	if h.app.threadLoadedRoute.Name != "IWhisper" || h.app.threadLoadedRequest.Name != board {
		t.Fatalf("loaded route=%+v request=%+v", h.app.threadLoadedRoute, h.app.threadLoadedRequest)
	}
	h.mu.Lock()
	h.overrides[endpoint] = "{invalid"
	h.mu.Unlock()
	h.click("刷新")
	if !h.app.failed || h.app.pageError != "" || len(h.app.thread.Articles) != 30 {
		t.Fatalf("alias failed refresh: failed=%v pageError=%q articles=%d", h.app.failed, h.app.pageError, len(h.app.thread.Articles))
	}
	if first, _ := state.Visible(); first != 8 || !h.view.HasText("#8") {
		t.Fatalf("alias failed refresh row=%d floor8=%t", first, h.view.HasText("#8"))
	}
	h.mu.Lock()
	h.overrides[endpoint] = string(data)
	h.mu.Unlock()
	h.click("重试")
	if h.app.failed || h.app.pageError != "" {
		t.Fatalf("alias refresh retry failed: %q", h.app.message)
	}
	state = h.app.desktopListState(h.app.threadListKey())
	if first, _ := state.Visible(); first != 0 {
		t.Fatalf("alias refresh retry row=%d, want 0", first)
	}
}

func TestDesktopPendingRefreshKeepsFloorAndInRowOffset(t *testing.T) {
	for _, size := range [][2]int{{1100, 760}, {720, 480}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			h := newHarness(t)
			h.login()
			const endpoint = "/open/threads/Go/fixture.json"
			data, err := json.Marshal(desktopLongThread(30))
			if err != nil {
				t.Fatal(err)
			}
			h.mu.Lock()
			h.overrides = map[string]string{endpoint: string(data)}
			h.mu.Unlock()
			h.goTo(Route{Page: "thread", Name: "Go", ID: "fixture", Number: 1})
			h.view.SetSize(size[0], size[1])
			state := h.app.desktopListState(h.app.threadListKey())
			state.ScrollTo(8, ui.Start)
			h.view.Frame()
			atTop, ok := h.view.Find("#8")
			if !ok {
				t.Fatal("missing floor 8")
			}
			h.view.Scroll(float32(size[0])*3/5, float32(size[1])*2/3, 0, 17)
			h.view.Frame()
			preRefresh, ok := h.view.Find("#8")
			if !ok {
				t.Fatal("missing floor 8 after applying the in-row offset")
			}
			offset := atTop.Y - preRefresh.Y
			firstBefore, _ := state.Visible()
			if firstBefore != 8 || offset <= 1 {
				t.Fatalf("invalid offset fixture: first=%d offset=%.1f", firstBefore, offset)
			}

			gate := make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(gate) }) }
			t.Cleanup(release)
			h.mu.Lock()
			h.overrides[endpoint] = "{invalid"
			h.gates = map[string]chan struct{}{endpoint: gate}
			h.mu.Unlock()
			if err := h.view.Click("刷新"); err != nil {
				t.Fatal(err)
			}
			h.view.Frame()
			waiting, ok := h.view.Find("#8")
			firstWaiting, _ := state.Visible()
			if !ok || !h.app.busy || firstWaiting != firstBefore {
				t.Fatalf("pending refresh changed the anchor: busy=%t visible=%t first=%d want=%d", h.app.busy, ok, firstWaiting, firstBefore)
			}
			release()
			h.idle()
			h.view.Frame()
			failed, ok := h.view.Find("#8")
			firstFailed, _ := state.Visible()
			if !ok || !h.app.failed || firstFailed != firstBefore {
				t.Fatalf("failed refresh changed the anchor: floor8=%t failed=%t first=%d want=%d", ok, h.app.failed, firstFailed, firstBefore)
			}
			if _, ok := h.view.Find("关闭提示"); !ok {
				t.Fatal("refresh failure has no dismiss control")
			}
			h.click("关闭提示")
			dismissed, ok := h.view.Find("#8")
			firstDismissed, _ := state.Visible()
			if !ok || firstDismissed != firstBefore {
				t.Fatalf("dismissing feedback changed the anchor: floor8=%t first=%d want=%d", ok, firstDismissed, firstBefore)
			}

			positions := []struct {
				name string
				y    float32
			}{
				{name: "pre-refresh", y: preRefresh.Y},
				{name: "waiting", y: waiting.Y},
				{name: "failed", y: failed.Y},
				{name: "dismissed", y: dismissed.Y},
			}
			for _, position := range positions {
				if math.Abs(float64(position.y-preRefresh.Y)) > .5 {
					t.Fatalf("%s floor moved: y=%.1f want %.1f", position.name, position.y, preRefresh.Y)
				}
				if math.Abs(float64((atTop.Y-position.y)-offset)) > .5 {
					t.Fatalf("%s in-row offset = %.1f, want %.1f", position.name, atTop.Y-position.y, offset)
				}
			}
			t.Logf("%dx%d floor 8 y: pre-refresh=%.1f waiting=%.1f failed=%.1f dismissed=%.1f; visible row=%d in-row offset=%.1f DIP",
				size[0], size[1], preRefresh.Y, waiting.Y, failed.Y, dismissed.Y, firstBefore, offset)
		})
	}
}

func TestDesktopAliasPageTwoFailureThenRetryPageOneTop(t *testing.T) {
	h := newHarness(t)
	h.login()
	const board = "悄悄话"
	const endpoint = "/open/threads/" + board + "/fixture.json"
	thread := desktopLongThread(30)
	thread.Board = "IWhisper"
	thread.Pagination.CurrentPage = 2
	data, err := json.Marshal(thread)
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.overrides = map[string]string{endpoint: string(data)}
	h.mu.Unlock()
	h.goTo(Route{Page: "thread", Name: board, ID: "fixture", Number: 2})
	h.view.SetSize(1100, 760)
	state := h.app.desktopListState(h.app.threadListKey())
	state.ScrollTo(8, ui.Start)
	h.view.Frame()
	h.mu.Lock()
	h.overrides[endpoint] = "{invalid"
	h.mu.Unlock()
	h.click("刷新")
	if first, _ := state.Visible(); !h.app.failed || len(h.app.thread.Articles) != 30 || h.app.route.Number != 2 || h.app.threadRequestPage != 1 || first != 8 {
		t.Fatalf("failed alias page two: failed=%t articles=%d display/request=%d/%d first=%d",
			h.app.failed, len(h.app.thread.Articles), h.app.route.Number, h.app.threadRequestPage, first)
	}
	thread.Pagination.CurrentPage = 1
	data, err = json.Marshal(thread)
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.overrides[endpoint] = string(data)
	h.mu.Unlock()
	h.click("重试")
	top := h.app.desktopListState(h.app.threadListKey())
	if first, _ := top.Visible(); h.app.route.Number != 1 || first != 0 || h.app.failed || h.count("GET "+endpoint) != 3 {
		t.Fatalf("retry route/row/failed/loads = %d/%d/%t/%d", h.app.route.Number, first, h.app.failed, h.count("GET "+endpoint))
	}
}

func TestDesktopCanPageWhileRefreshFailureIsShown(t *testing.T) {
	for _, size := range [][2]int{{1100, 760}, {720, 480}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			h := newHarness(t)
			h.login()
			const endpoint = "/open/threads/Go/fixture.json"
			thread := desktopLongThread(30)
			page1, err := json.Marshal(thread)
			if err != nil {
				t.Fatal(err)
			}
			thread.Pagination.CurrentPage = 2
			thread.Articles[0].Content = "page two body"
			page2, err := json.Marshal(thread)
			if err != nil {
				t.Fatal(err)
			}
			h.mu.Lock()
			h.overrides = map[string]string{endpoint: string(page1)}
			h.mu.Unlock()
			h.goTo(Route{Page: "thread", Name: "Go", ID: "fixture", Number: 1})
			h.view.SetSize(size[0], size[1])
			h.view.Frame()
			if _, ok := h.view.Find("下一页"); !ok || h.app.threadDisplayPage() != 1 {
				t.Fatal("initial page has no next-page control")
			}
			h.mu.Lock()
			h.overrides[endpoint] = "{invalid"
			h.mu.Unlock()
			h.click("刷新")
			h.view.Frame()
			if !h.app.failed || h.app.busy || len(h.app.thread.Articles) != 30 {
				t.Fatalf("invalid failure fixture: failed=%t busy=%t articles=%d", h.app.failed, h.app.busy, len(h.app.thread.Articles))
			}
			if _, ok := h.view.Find("下一页"); !ok {
				t.Fatal("next-page control disappeared under the refresh error")
			}
			if _, ok := h.view.Find("关闭提示"); !ok {
				t.Fatal("refresh error is not visible before paging")
			}
			h.mu.Lock()
			h.overrides[endpoint] = string(page2)
			h.mu.Unlock()
			before := h.count("GET " + endpoint)
			if err := h.view.Click("下一页"); err != nil {
				t.Fatalf("next page unavailable with refresh error shown: %v", err)
			}
			h.idle()
			h.view.Frame()
			if h.app.threadDisplayPage() != 2 || h.count("GET "+endpoint) != before+1 {
				t.Fatalf("pagination displayed=%d requests=%d want 2/%d", h.app.threadDisplayPage(), h.count("GET "+endpoint), before+1)
			}
			q := h.query("GET " + endpoint)
			values, err := url.ParseQuery(q)
			if err != nil {
				t.Fatalf("parse page request query %q: %v", q, err)
			}
			if got := values.Get("page"); got != "2" {
				t.Fatalf("page request query = %q, page=%q want 2", q, got)
			}
			if h.count("GET "+endpoint) != before+1 {
				t.Fatalf("page two request count = %d, want %d", h.count("GET "+endpoint), before+1)
			}
			t.Logf("%dx%d page request: query=%q, count %d -> %d, displayed page %d", size[0], size[1], q, before, h.count("GET "+endpoint), h.app.threadDisplayPage())
			h.has("page two body")
		})
	}
}
