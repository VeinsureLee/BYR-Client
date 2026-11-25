package desktop

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/VeinsureLee/BYR-Client/core/forum"
	"github.com/egoist/mygo/ui"
)

func desktopSearchPageBody(prefix string, startID, count int) string {
	articles := make([]forum.Article, count)
	for i := range articles {
		id := startID + i
		articles[i] = forum.Article{
			ID: forum.Scalar(fmt.Sprint(id)), GroupID: forum.Scalar(fmt.Sprint(id)),
			Board: "Go", Title: fmt.Sprintf("%s %02d", prefix, i+1),
			PostTime: forum.Scalar(fmt.Sprint(1700000000 + id)),
		}
	}
	body, err := json.Marshal(forum.SearchResults{
		Articles:   forum.List[forum.Article](articles),
		Pagination: forum.Pagination{CurrentPage: 1, TotalPages: 2, TotalItems: 60},
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func TestDesktopSearchNextPageFailureKeepsPageOneAndRetriesTarget(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.view.SetSize(1200, 900)
	h.app.route = Route{Page: "search", Number: 1}
	h.app.searchQuery = "needle"
	h.app.searchSubmittedQuery = "needle"
	h.app.searchHasRun = true
	h.app.searchResults = make([]forum.Article, 30)
	for i := range h.app.searchResults {
		h.app.searchResults[i] = forum.Article{
			ID: forum.Scalar(fmt.Sprint(i + 1)), GroupID: forum.Scalar(fmt.Sprint(i + 1)),
			Board: "Go", Title: fmt.Sprintf("第一页结果 %02d", i+1),
		}
	}
	h.app.searchPagination = forum.Pagination{CurrentPage: 1, TotalPages: 2, TotalItems: 60}
	h.view.Frame()
	state := h.app.desktopListState(h.app.articleListKey())
	state.ScrollTo(29, ui.Start)
	h.view.Frame()
	anchor := "第一页结果 30"
	before, found := h.view.Find(anchor)
	if !found {
		t.Fatalf("scrolled search list did not show %q; visible=%q", anchor, h.view.Texts())
	}
	if !h.view.HasText("第 1 / 2 页") {
		t.Fatalf("page one pager missing: %q", h.view.Texts())
	}

	gate := make(chan struct{})
	released := false
	release := func() {
		if !released {
			released = true
			close(gate)
		}
	}
	t.Cleanup(release)
	beforeCount := h.count("GET /open/search/article.json")
	h.mu.Lock()
	h.gates = map[string]chan struct{}{"/open/search/article.json": gate}
	h.overrides = map[string]string{"/open/search/article.json": `{"invalid":`}
	h.mu.Unlock()

	if err := h.view.Click("下一页"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.count("GET /open/search/article.json") >= beforeCount+2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := h.count("GET /open/search/article.json"); got < beforeCount+2 {
		t.Fatalf("next-page requests = %d, want at least %d", got, beforeCount+2)
	}
	h.view.Frame()
	if !h.app.busy || h.app.route.Number != 1 || len(h.app.searchResults) != 30 || h.app.searchRetryPage != 2 {
		t.Fatalf("waiting page = busy:%t page:%d results:%d target:%d", h.app.busy, h.app.route.Number, len(h.app.searchResults), h.app.searchRetryPage)
	}
	if cur, ok := h.view.Find(anchor); !ok || cur.Y != before.Y {
		t.Fatalf("waiting page moved the visible row: before=%v after=%v found=%t", before, cur, ok)
	}
	if !h.view.HasText("第 1 / 2 页") {
		t.Fatalf("waiting page changed the pager: %q", h.view.Texts())
	}

	release()
	h.idle()
	h.view.Frame()
	if !h.app.failed || h.app.route.Number != 1 || len(h.app.searchResults) != 30 || h.app.searchRetryPage != 2 {
		t.Fatalf("failed page = failed:%t page:%d results:%d target:%d", h.app.failed, h.app.route.Number, len(h.app.searchResults), h.app.searchRetryPage)
	}
	if cur, ok := h.view.Find(anchor); !ok || cur.Y != before.Y {
		t.Fatalf("failed page moved the visible row: before=%v after=%v found=%t", before, cur, ok)
	}
	if !h.view.HasText("第 1 / 2 页") {
		t.Fatalf("failed page changed the pager: %q", h.view.Texts())
	}
	if !h.view.HasText("重试") {
		t.Fatalf("failed page has no retry entry: %q", h.view.Texts())
	}
	h.click("关闭提示")
	h.view.Frame()
	if h.app.route.Number != 1 || len(h.app.searchResults) != 30 || !h.view.HasText("第 1 / 2 页") {
		t.Fatalf("dismissing the error changed page one: page=%d results=%d", h.app.route.Number, len(h.app.searchResults))
	}

	h.mu.Lock()
	h.gates = nil
	h.overrides = map[string]string{"/open/search/article.json": desktopSearchPageBody("第二页结果", 61, 60)}
	h.searchBodies = map[string]string{"CS": `{"article":[],"pagination":{}}`}
	h.mu.Unlock()
	h.click("下一页")
	h.view.Frame()
	if h.app.failed || h.app.route.Number != 2 || len(h.app.searchResults) != 30 ||
		h.app.searchPagination.CurrentPage != 2 || h.app.searchRetryPage != 0 {
		t.Fatalf("successful retry = failed:%t page:%d results:%d pagination:%+v target:%d",
			h.app.failed, h.app.route.Number, len(h.app.searchResults), h.app.searchPagination, h.app.searchRetryPage)
	}
	h.has("第 2 / 2 页")
	if !h.view.HasText("第二页结果 30") {
		t.Fatalf("second page results missing: %q", h.view.Texts())
	}
}

func TestDesktopSearchPartialFailureStates(t *testing.T) {
	const hit = `{"article":[{"id":1,"group_id":1,"board_name":"Go","title":"部分命中"}],"pagination":{"page_all_count":1,"item_all_count":1}}`
	const empty = `{"article":[],"pagination":{}}`
	const broken = `{"invalid":`
	for _, tc := range []struct {
		name, goBody, csBody string
		wantTexts            []string
		absentTexts          []string
		wantIncomplete       bool
		wantFailed           bool
	}{
		{
			name: "partial with hits", goBody: hit, csBody: broken,
			wantTexts: []string{"部分命中", "结果可能不完整", "重试"}, wantIncomplete: true,
		},
		{
			name: "partial with no hits", goBody: empty, csBody: broken,
			wantTexts: []string{"搜索未完整", "重试搜索"}, absentTexts: []string{"没有找到主题"}, wantIncomplete: true,
		},
		{
			name: "complete empty", goBody: empty, csBody: empty,
			wantTexts: []string{"没有找到主题"}, absentTexts: []string{"搜索未完整", "结果可能不完整"},
		},
		{
			name: "all failed", goBody: broken, csBody: broken,
			wantTexts: []string{"内容加载失败", "重新加载"}, wantFailed: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.login()
			h.goTo(Route{Page: "search"})
			h.mu.Lock()
			h.searchBodies = map[string]string{"Go": tc.goBody, "CS": tc.csBody}
			h.mu.Unlock()
			h.typeIn("搜索关键词", "needle")
			h.click("开始搜索")
			h.view.Frame()
			if h.app.searchIncomplete != tc.wantIncomplete || h.app.failed != tc.wantFailed {
				t.Fatalf("state = incomplete:%t failed:%t message:%q results:%d",
					h.app.searchIncomplete, h.app.failed, h.app.message, len(h.app.searchResults))
			}
			for _, text := range tc.wantTexts {
				h.has(text)
			}
			for _, text := range tc.absentTexts {
				if h.view.HasText(text) {
					t.Fatalf("unexpected %q in %q", text, h.view.Texts())
				}
			}
			if tc.wantIncomplete && !tc.wantFailed {
				if h.app.searchFailedBoards != 1 {
					t.Fatalf("failed boards = %d, want 1", h.app.searchFailedBoards)
				}
				before := h.count("GET /open/search/article.json")
				h.mu.Lock()
				start := len(h.requests)
				h.mu.Unlock()
				h.app.searchQuery = "draft"
				h.view.Frame()
				retry := "重试"
				if len(h.app.searchResults) == 0 {
					retry = "重试搜索"
				}
				h.click(retry)
				if h.count("GET /open/search/article.json") <= before {
					t.Fatalf("incomplete retry %q did not repeat the search", retry)
				}
				if h.app.searchQuery != "draft" {
					t.Fatalf("incomplete retry changed the draft to %q", h.app.searchQuery)
				}
				h.mu.Lock()
				retryRequests := append([]string(nil), h.requests[start:]...)
				retryQueries := append([]string(nil), h.queries[start:]...)
				h.mu.Unlock()
				for i, request := range retryRequests {
					if request != "GET /open/search/article.json" {
						continue
					}
					values, err := url.ParseQuery(retryQueries[i])
					if err != nil {
						t.Fatal(err)
					}
					if values.Get("title1") != "needle" {
						t.Fatalf("incomplete retry query = %q, want title1=needle", retryQueries[i])
					}
				}
			}
		})
	}
}

func TestDesktopSearchPageRetryUsesSubmittedQueryAfterEditorEdit(t *testing.T) {
	h := newHarness(t)
	h.login()
	h.view.SetSize(1200, 900)
	h.app.route = Route{Page: "search", Number: 1}
	h.app.searchQuery = "needle"
	h.app.searchSubmittedQuery = "needle"
	h.app.searchHasRun = true
	h.app.searchResults = make([]forum.Article, 30)
	for i := range h.app.searchResults {
		h.app.searchResults[i] = forum.Article{
			ID: forum.Scalar(fmt.Sprint(i + 1)), GroupID: forum.Scalar(fmt.Sprint(i + 1)),
			Board: "Go", Title: fmt.Sprintf("第一页结果 %02d", i+1),
		}
	}
	h.app.searchPagination = forum.Pagination{CurrentPage: 1, TotalPages: 2, TotalItems: 60}
	h.view.Frame()
	state := h.app.desktopListState(h.app.articleListKey())
	state.ScrollTo(29, ui.Start)
	h.view.Frame()
	before, found := h.view.Find("第一页结果 30")
	if !found {
		t.Fatalf("scrolled search list did not show the anchor: %q", h.view.Texts())
	}

	gate := make(chan struct{})
	released := false
	release := func() {
		if !released {
			released = true
			close(gate)
		}
	}
	t.Cleanup(release)
	beforeCount := h.count("GET /open/search/article.json")
	h.mu.Lock()
	h.gates = map[string]chan struct{}{"/open/search/article.json": gate}
	h.overrides = map[string]string{"/open/search/article.json": `{"invalid":`}
	h.mu.Unlock()
	if err := h.view.Click("下一页"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.count("GET /open/search/article.json") >= beforeCount+2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	release()
	h.idle()
	h.view.Frame()
	if !h.app.failed || h.app.route.Number != 1 {
		t.Fatalf("failed page state = failed:%t page:%d", h.app.failed, h.app.route.Number)
	}
	if cur, ok := h.view.Find("第一页结果 30"); !ok || cur.Y != before.Y {
		t.Fatalf("failed page moved page one: before=%v after=%v found=%t", before, cur, ok)
	}

	failedCount := h.count("GET /open/search/article.json")
	h.mu.Lock()
	h.gates = nil
	h.overrides = map[string]string{"/open/search/article.json": desktopSearchPageBody("第二页结果", 61, 60)}
	h.searchBodies = map[string]string{"CS": `{"article":[],"pagination":{}}`}
	h.mu.Unlock()
	h.app.searchQuery = "draft"
	h.app.searchSubmittedQuery = "needle"
	h.view.Frame()
	if !h.view.HasText("重试") {
		t.Fatalf("failed page has no retry entry: %q", h.view.Texts())
	}
	h.click("重试")
	h.view.Frame()
	if h.app.failed || h.app.route.Number != 2 || len(h.app.searchResults) != 30 || h.app.searchQuery != "draft" {
		t.Fatalf("retry result = failed:%t page:%d results:%d query:%q", h.app.failed, h.app.route.Number, len(h.app.searchResults), h.app.searchQuery)
	}
	if !h.view.HasText("第 2 / 2 页") || !h.view.HasText("第二页结果 30") {
		t.Fatalf("second page missing: %q", h.view.Texts())
	}
	h.mu.Lock()
	requests := append([]string(nil), h.requests...)
	queries := append([]string(nil), h.queries...)
	h.mu.Unlock()
	checked := 0
	for i := failedCount; i < len(requests); i++ {
		if requests[i] != "GET /open/search/article.json" {
			continue
		}
		values, err := url.ParseQuery(queries[i])
		if err != nil {
			t.Fatal(err)
		}
		if values.Get("title1") != "needle" || values.Get("count") != "60" {
			t.Fatalf("retry query = %q, want title1=needle count=60", queries[i])
		}
		checked++
	}
	if checked < 2 {
		t.Fatalf("retry search requests = %d, want both board requests after the failure", checked)
	}
}

func TestDesktopUnauthenticatedSearchFailureShowsLoginExpired(t *testing.T) {
	h := newHarness(t)
	h.app.fail(forum.ErrUnauthenticated)
	if !h.app.failed || h.app.message != "登录已失效，请退出后重新登录" {
		t.Fatalf("authentication failure state = failed:%t message:%q", h.app.failed, h.app.message)
	}
}

func TestDesktopSameQueryResubmitFailureRetriesSubmittedQueryAfterDraftEdit(t *testing.T) {
	h := newHarness(t)
	h.login()
	const endpoint = "/open/search/article.json"
	h.view.SetSize(1100, 760)
	h.app.route = Route{Page: "search", Number: 1}
	h.app.searchMode = "all"
	h.app.searchQuery = "needle"
	h.app.searchSubmittedQuery = "needle"
	h.app.searchHasRun = true
	h.app.searchResults = make([]forum.Article, 30)
	for i := range h.app.searchResults {
		h.app.searchResults[i] = forum.Article{ID: forum.Scalar(fmt.Sprint(i + 1)), Board: "Go", Title: fmt.Sprintf("旧结果 %02d", i+1)}
	}
	h.app.searchPagination = forum.Pagination{CurrentPage: 1, TotalPages: 2, TotalItems: 60}
	h.view.Frame()

	gate := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gate) }) }
	t.Cleanup(release)
	before := h.count("GET " + endpoint)
	h.mu.Lock()
	h.overrides = map[string]string{endpoint: "{invalid"}
	h.gates = map[string]chan struct{}{endpoint: gate}
	h.mu.Unlock()
	if err := h.view.Click("开始搜索"); err != nil {
		t.Fatal(err)
	}
	h.waitFor(func() bool { return h.count("GET "+endpoint) >= before+2 })
	release()
	h.idle()
	h.view.Frame()
	if !h.app.failed || len(h.app.searchResults) != 30 || h.app.route.Number != 1 {
		t.Fatalf("failed resubmit = failed:%t results:%d page:%d", h.app.failed, len(h.app.searchResults), h.app.route.Number)
	}

	h.app.searchQuery = "draft"
	h.view.Frame()
	h.mu.Lock()
	h.gates = nil
	h.overrides = map[string]string{endpoint: desktopSearchPageBody("新结果", 1, 30)}
	h.searchBodies = map[string]string{"CS": `{"article":[],"pagination":{}}`}
	start := len(h.requests)
	h.mu.Unlock()
	h.click("重试")
	h.view.Frame()
	if h.app.failed || h.app.route.Number != 1 || len(h.app.searchResults) != 30 || h.app.searchQuery != "draft" {
		t.Fatalf("retry = failed:%t page:%d results:%d draft:%q", h.app.failed, h.app.route.Number, len(h.app.searchResults), h.app.searchQuery)
	}
	if !h.view.HasText("新结果 30") {
		t.Fatalf("retry did not commit new page one: %q", h.view.Texts())
	}
	h.mu.Lock()
	requests := append([]string(nil), h.requests[start:]...)
	queries := append([]string(nil), h.queries[start:]...)
	h.mu.Unlock()
	checked := 0
	for i, request := range requests {
		if request != "GET "+endpoint {
			continue
		}
		values, err := url.ParseQuery(queries[i])
		if err != nil {
			t.Fatal(err)
		}
		if values.Get("title1") != "needle" || values.Get("count") != "30" || values.Get("page") != "1" {
			t.Fatalf("retry query = %q, want needle/page 1/count 30", queries[i])
		}
		checked++
	}
	if checked < 2 {
		t.Fatalf("retry search requests = %d, want both boards", checked)
	}
}
