package desktop

import (
	"fmt"
	"net/url"
	"sync"
	"testing"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/egoist/mygo/ui"
)

func TestDesktopSameSearchRefreshKeepsOldResultsPageAndPositionUntilFailure(t *testing.T) {
	h := newHarness(t)
	h.login()
	const endpoint = "/open/search/article.json"
	h.app.route = Route{Page: "search", Number: 2}
	h.app.searchMode = "all"
	h.app.searchQuery = "needle"
	h.app.searchSubmittedQuery = "needle"
	h.app.searchHasRun = true
	h.app.searchResults = desktopLongThread(30).Articles
	h.app.searchPagination = forum.Pagination{CurrentPage: 2, TotalPages: 3, TotalItems: 120}
	h.view.SetSize(1100, 760)
	h.view.Frame()
	state := h.app.desktopListState(h.app.articleListKey())
	state.ScrollTo(8, ui.Start)
	h.view.Frame()
	firstBefore, _ := state.Visible()
	if firstBefore != 8 {
		t.Fatalf("search result anchor fixture first row = %d, want 8", firstBefore)
	}

	gate := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gate) }) }
	t.Cleanup(release)
	h.mu.Lock()
	h.overrides = map[string]string{endpoint: "{invalid"}
	h.gates = map[string]chan struct{}{endpoint: gate}
	h.mu.Unlock()
	before := h.count("GET " + endpoint)
	h.app.loadSearch()
	h.waitFor(func() bool { return h.count("GET "+endpoint) > before })
	h.view.Frame()
	if !h.app.busy || len(h.app.searchResults) != 30 || h.app.route.Number != 2 {
		t.Fatalf("pending same-query refresh changed data/page: busy=%t results=%d page=%d",
			h.app.busy, len(h.app.searchResults), h.app.route.Number)
	}
	if first, _ := state.Visible(); first != firstBefore {
		t.Fatalf("pending refresh row = %d, want %d", first, firstBefore)
	}

	release()
	h.idle()
	h.view.Frame()
	if !h.app.failed || h.app.pageError != "" || len(h.app.searchResults) != 30 || h.app.route.Number != 2 {
		t.Fatalf("failed same-query refresh replaced data/page: failed=%t pageError=%q results=%d page=%d",
			h.app.failed, h.app.pageError, len(h.app.searchResults), h.app.route.Number)
	}
	if first, _ := state.Visible(); first != firstBefore {
		t.Fatalf("failed refresh row = %d, want %d", first, firstBefore)
	}
	t.Logf("%dx%d retained page 2 and row %d through failed same-query refresh", 1100, 760, firstBefore)
}

func TestDesktopSearchEmptyStateViewportIntegration(t *testing.T) {
	for _, size := range [][2]int{{720, 480}, {1100, 760}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			h := newHarness(t)
			h.login()
			h.app.route = Route{Page: "search"}
			h.app.searchMode = "all"
			h.app.searchHasRun = false
			h.view.SetSize(size[0], size[1])
			h.view.Frame()
			if _, ok := h.view.Find("输入关键词开始搜索"); !ok {
				t.Fatalf("search initial empty state not visible: %q", h.view.Texts())
			}
		})
	}
}

func TestDesktopSearchTopBarRefreshKeepsPageTwoUntilPageOneCommits(t *testing.T) {
	h := newHarness(t)
	h.login()
	const endpoint = "/open/search/article.json"
	h.view.SetSize(1100, 760)
	h.app.route = Route{Page: "search", Number: 2}
	h.app.searchMode = "all"
	h.app.searchQuery = "draft"
	h.app.searchSubmittedQuery = "needle"
	h.app.searchHasRun = true
	h.app.searchResults = desktopLongThread(30).Articles
	h.app.searchPagination = forum.Pagination{CurrentPage: 2, TotalPages: 3, TotalItems: 120}
	h.view.Frame()
	state := h.app.desktopListState(h.app.articleListKey())
	state.ScrollTo(8, ui.Start)
	h.view.Frame()
	firstBefore, _ := state.Visible()
	if firstBefore != 8 {
		t.Fatalf("search page two first row = %d, want 8", firstBefore)
	}
	if !h.view.HasText("第 2 / 3 页") {
		t.Fatalf("page two pager missing: %q", h.view.Texts())
	}

	gate := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gate) }) }
	t.Cleanup(release)
	requestsFor := func() int {
		h.mu.Lock()
		defer h.mu.Unlock()
		n := 0
		for _, request := range h.requests {
			if request == "GET "+endpoint {
				n++
			}
		}
		return n
	}
	h.mu.Lock()
	h.overrides = map[string]string{endpoint: "{invalid"}
	h.gates = map[string]chan struct{}{endpoint: gate}
	h.mu.Unlock()
	before := requestsFor()
	if err := h.view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	h.waitFor(func() bool { return requestsFor() >= before+2 })
	h.view.Frame()
	if !h.app.busy || h.app.route.Number != 2 || len(h.app.searchResults) != 30 ||
		h.app.searchPagination.CurrentPage != 2 || h.app.searchRetryPage != 1 || h.app.searchRetryQuery != "needle" {
		t.Fatalf("waiting refresh = busy:%t page:%d results:%d pagination:%+v retry:%q/%d",
			h.app.busy, h.app.route.Number, len(h.app.searchResults), h.app.searchPagination, h.app.searchRetryQuery, h.app.searchRetryPage)
	}
	if first, _ := state.Visible(); first != 8 || !h.view.HasText("第 2 / 3 页") {
		t.Fatalf("waiting refresh moved page two: first=%d texts=%q", first, h.view.Texts())
	}
	release()
	h.idle()
	h.view.Frame()
	if !h.app.failed || h.app.route.Number != 2 || h.app.searchPagination.CurrentPage != 2 {
		t.Fatalf("failed refresh = failed:%t page:%d pagination:%+v", h.app.failed, h.app.route.Number, h.app.searchPagination)
	}
	if first, _ := state.Visible(); first != 8 || !h.view.HasText("第 2 / 3 页") {
		t.Fatalf("failed refresh moved page two: first=%d texts=%q", first, h.view.Texts())
	}
	h.click("关闭提示")
	h.view.Frame()
	if h.app.route.Number != 2 || h.app.searchPagination.CurrentPage != 2 {
		t.Fatalf("dismissing refresh error changed page: %d/%+v", h.app.route.Number, h.app.searchPagination)
	}
	if first, _ := state.Visible(); first != 8 || !h.view.HasText("第 2 / 3 页") {
		t.Fatalf("dismissing refresh error moved page two: first=%d texts=%q", first, h.view.Texts())
	}

	// Fail once more so the error retry can be exercised through the real
	// top-bar refresh entry.
	gate2 := make(chan struct{})
	var releaseOnce2 sync.Once
	release2 := func() { releaseOnce2.Do(func() { close(gate2) }) }
	t.Cleanup(release2)
	h.mu.Lock()
	h.overrides = map[string]string{endpoint: "{invalid"}
	h.gates = map[string]chan struct{}{endpoint: gate2}
	h.mu.Unlock()
	before = requestsFor()
	if err := h.view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	h.waitFor(func() bool { return requestsFor() >= before+2 })
	release2()
	h.idle()
	h.view.Frame()
	if !h.app.failed || h.app.route.Number != 2 {
		t.Fatalf("second refresh failure = failed:%t page:%d", h.app.failed, h.app.route.Number)
	}

	h.mu.Lock()
	h.gates = nil
	h.overrides = map[string]string{endpoint: desktopSearchPageBody("第一页结果", 1, 30)}
	h.searchBodies = map[string]string{"CS": `{"article":[],"pagination":{}}`}
	h.mu.Unlock()
	before = requestsFor()
	h.click("重试")
	h.view.Frame()
	if h.app.failed || h.app.route.Number != 1 || h.app.searchPagination.CurrentPage != 1 || len(h.app.searchResults) != 30 {
		t.Fatalf("refresh retry = failed:%t page:%d pagination:%+v results:%d",
			h.app.failed, h.app.route.Number, h.app.searchPagination, len(h.app.searchResults))
	}
	top := h.app.desktopListState(h.app.articleListKey())
	if first, _ := top.Visible(); first != 0 {
		t.Fatalf("refresh success first row = %d, want top", first)
	}
	h.mu.Lock()
	requests := append([]string(nil), h.requests...)
	queries := append([]string(nil), h.queries...)
	h.mu.Unlock()
	checked := 0
	for i := before; i < len(requests); i++ {
		if requests[i] != "GET "+endpoint {
			continue
		}
		values, err := url.ParseQuery(queries[i])
		if err != nil {
			t.Fatal(err)
		}
		if values.Get("title1") != "needle" || values.Get("count") != "30" || values.Get("page") != "1" {
			t.Fatalf("refresh retry query = %q, want needle/page 1/count 30", queries[i])
		}
		checked++
	}
	if checked < 2 {
		t.Fatalf("refresh retry search requests = %d, want both boards", checked)
	}
	if h.app.searchQuery != "draft" {
		t.Fatalf("refresh retry changed the editor draft to %q", h.app.searchQuery)
	}
}

func TestDesktopSearchModeSwitchDropsLateRefreshResultAndNotice(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "delayed success"
		if fail {
			name = "delayed failure"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.login()
			const endpoint = "/open/search/article.json"
			h.view.SetSize(1100, 760)
			h.app.route = Route{Page: "search", Number: 1}
			h.app.searchMode = "all"
			h.app.searchQuery = "needle"
			h.app.searchSubmittedQuery = "needle"
			h.app.searchHasRun = true
			h.app.searchResults = desktopLongThread(30).Articles
			h.app.searchPagination = forum.Pagination{CurrentPage: 1, TotalPages: 2, TotalItems: 60}
			h.view.Frame()

			gate := make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(gate) }) }
			t.Cleanup(release)
			before := h.count("GET " + endpoint)
			body := desktopSearchPageBody("延迟结果", 1, 30)
			if fail {
				body = "{invalid"
			}
			h.mu.Lock()
			h.overrides = map[string]string{endpoint: body}
			h.gates = map[string]chan struct{}{endpoint: gate}
			h.mu.Unlock()
			if err := h.view.Click("刷新"); err != nil {
				t.Fatal(err)
			}
			h.waitFor(func() bool { return h.count("GET "+endpoint) >= before+2 })
			if !h.app.busy || !h.app.refreshing {
				t.Fatalf("refresh did not start: busy=%t refreshing=%t", h.app.busy, h.app.refreshing)
			}

			if err := h.view.Click("人物"); err != nil {
				t.Fatal(err)
			}
			if h.app.searchMode != "user" || h.app.busy || h.app.refreshing || h.app.failed ||
				h.app.message != "" || h.app.retryLoad != nil || h.app.searchRetryPage != 0 || len(h.app.searchResults) != 0 {
				t.Fatalf("mode switch left old request state: mode=%q busy=%t refreshing=%t failed=%t message=%q retry=%v page=%d results=%d",
					h.app.searchMode, h.app.busy, h.app.refreshing, h.app.failed, h.app.message, h.app.retryLoad != nil, h.app.searchRetryPage, len(h.app.searchResults))
			}

			release()
			h.idle()
			h.view.Frame()
			if h.app.searchMode != "user" || h.app.failed || h.app.message != "" || h.app.retryLoad != nil {
				t.Fatalf("late refresh applied to user mode: mode=%q failed=%t message=%q retry=%v",
					h.app.searchMode, h.app.failed, h.app.message, h.app.retryLoad != nil)
			}
			if h.view.HasText("刷新成功") || h.view.HasText("重试") {
				t.Fatalf("late refresh notice/retry leaked into user mode: %q", h.view.Texts())
			}
			if len(h.app.searchResults) != 0 || h.app.searchHasRun {
				t.Fatalf("late refresh repopulated user mode: results=%d hasRun=%t", len(h.app.searchResults), h.app.searchHasRun)
			}
		})
	}
}
