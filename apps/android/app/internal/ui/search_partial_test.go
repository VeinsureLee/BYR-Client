package ui

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/widget"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func newSearchBoardServer(t *testing.T, search http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/section.json":
			_, _ = io.WriteString(w, `{"section":[{"name":"0"}]}`)
		case "/open/section/0.json":
			_, _ = io.WriteString(w, `{"name":"0","board":[{"name":"Go"},{"name":"Advice"}]}`)
		case "/open/search/article.json":
			search(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestAndroidSearchNextPageFailureKeepsPageOneAndRetriesTarget(t *testing.T) {
	var mu sync.Mutex
	counts := []string{}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	fail := true
	server := newSearchBoardServer(t, func(w http.ResponseWriter, r *http.Request) {
		count := r.URL.Query().Get("count")
		mu.Lock()
		counts = append(counts, count)
		mu.Unlock()
		if count == "60" {
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
			if fail {
				http.Error(w, "page unavailable", http.StatusBadGateway)
				return
			}
			if r.URL.Query().Get("board") == "Advice" {
				_, _ = io.WriteString(w, `{"article":[],"pagination":{}}`)
				return
			}
			var body strings.Builder
			body.WriteString(`{"article":[`)
			for i := 1; i <= 60; i++ {
				if i > 1 {
					body.WriteByte(',')
				}
				fmt.Fprintf(&body, `{"id":%d,"group_id":%d,"board_name":"Go","title":"第二页 %d","post_time":%d}`, i, i, i, 1700000000+i)
			}
			body.WriteString(`],"pagination":{"page_all_count":2,"page_current_count":2,"item_all_count":60}}`)
			_, _ = io.WriteString(w, body.String())
			return
		}
		_, _ = io.WriteString(w, `{"article":[{"id":1,"group_id":1,"board_name":"Go","title":"第一页结果"}],"pagination":{"page_all_count":2,"page_current_count":1,"item_all_count":31}}`)
	})
	t.Cleanup(unblock)
	a := interactionAppWithBackend(t, server.URL, nil)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"
	a.searchHasRun = true
	a.searchEdit.SetText("needle")
	a.searchListQuery = "needle"
	a.searchResults = make([]forum.Article, 20)
	for i := range a.searchResults {
		a.searchResults[i] = forum.Article{ID: forum.Scalar(fmt.Sprintf("%d", i+1)), Board: "Go", Title: fmt.Sprintf("第一页结果 %d", i+1)}
	}
	a.searchPagination = forum.Pagination{CurrentPage: 1, TotalPages: 2, TotalItems: 31}
	key := searchListKey(a.current(), "needle")
	list := a.mobileList(key)
	list.Position.First = 20
	list.Position.Offset = 9
	renderAppAt(t, a, 420, 700, 1)
	before := list.Position
	if before.First == 0 {
		t.Fatalf("fixture list did not scroll: %+v", before)
	}
	if a.clicks["pager-next:1"] == nil {
		t.Fatalf("pager next control was not rendered; clicks=%d", len(a.clicks))
	}
	a.click("pager-next:1").Click()
	renderAppAt(t, a, 420, 700, 1)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("next-page request did not start")
	}
	if a.current().Number != 1 || len(a.searchResults) != 20 || a.searchResults[0].ID != "1" ||
		a.searchPagination.CurrentPage != 1 || list.Position != before {
		t.Fatalf("waiting page changed displayed state: page=%d results=%d pagination=%+v pos=%+v before=%+v",
			a.current().Number, len(a.searchResults), a.searchPagination, list.Position, before)
	}
	unblock()
	waitForIdle(t, a)
	if !a.failed || a.current().Number != 1 || len(a.searchResults) != 20 || a.searchResults[0].ID != "1" ||
		a.searchPagination.CurrentPage != 1 || list.Position != before {
		t.Fatalf("failed page replaced page one: failed=%t page=%d results=%d pagination=%+v pos=%+v before=%+v",
			a.failed, a.current().Number, len(a.searchResults), a.searchPagination, list.Position, before)
	}

	fail = false
	a.click("retry-status").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	if a.failed || a.current().Number != 2 || len(a.searchResults) != 30 ||
		a.searchPagination.CurrentPage != 2 || a.searchPagination.TotalPages != 2 || a.searchRetryPage != 0 {
		t.Fatalf("successful retry = failed:%t page=%d results=%d pagination=%+v retry=%d",
			a.failed, a.current().Number, len(a.searchResults), a.searchPagination, a.searchRetryPage)
	}
	newKey := searchListKey(a.current(), "needle")
	if got := a.mobileList(newKey); got.Position.First != 0 || got.Position.Offset != 0 {
		t.Fatalf("new page list position = %+v, want a fresh page", got.Position)
	}
	mu.Lock()
	gotCounts := append([]string(nil), counts...)
	mu.Unlock()
	if len(gotCounts) != 4 {
		t.Fatalf("paging requests = %v, want two failed and two retried board reads", gotCounts)
	}
	for _, count := range gotCounts {
		if count != "60" {
			t.Fatalf("paging requests = %v, want page 2 expressed as count=60", gotCounts)
		}
	}
}

func TestAndroidSearchPartialFailureStates(t *testing.T) {
	for _, tc := range []struct {
		name       string
		goStatus   int
		adviceSt   int
		goBody     string
		wantHits   int
		incomplete bool
		failed     bool
		retryEntry bool
	}{
		{
			name: "partial with hits", goStatus: 200, adviceSt: 502,
			goBody:   `{"article":[{"id":1,"group_id":1,"board_name":"Go","title":"命中"}],"pagination":{"page_all_count":1,"item_all_count":1}}`,
			wantHits: 1, incomplete: true, retryEntry: true,
		},
		{
			name: "partial with no hits", goStatus: 200, adviceSt: 502,
			goBody:     `{"article":[],"pagination":{"page_all_count":0,"item_all_count":0}}`,
			incomplete: true, retryEntry: true,
		},
		{
			name: "complete empty", goStatus: 200, adviceSt: 200,
			goBody: `{"article":[],"pagination":{"page_all_count":0,"item_all_count":0}}`,
		},
		{
			name: "all failed", goStatus: 502, adviceSt: 502, failed: true, retryEntry: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var searchCalls atomic.Int64
			var titlesMu sync.Mutex
			titles := []string{}
			server := newSearchBoardServer(t, func(w http.ResponseWriter, r *http.Request) {
				searchCalls.Add(1)
				titlesMu.Lock()
				titles = append(titles, r.URL.Query().Get("title1"))
				titlesMu.Unlock()
				status, body := tc.goStatus, tc.goBody
				if r.URL.Query().Get("board") == "Advice" {
					status, body = tc.adviceSt, `{"article":[],"pagination":{}}`
				}
				if status != 200 {
					http.Error(w, "board unavailable", status)
					return
				}
				_, _ = io.WriteString(w, body)
			})
			a := interactionAppWithBackend(t, server.URL, nil)
			defer a.Close()
			a.stack = []Route{{Page: PageSearch, Number: 1}}
			a.searchMode = "all"
			a.searchEdit.SetText("needle")
			a.loadSearch()
			waitForIdle(t, a)
			a.clicks = map[string]*widget.Clickable{}
			renderAppAt(t, a, 420, 700, 1)

			if len(a.searchResults) != tc.wantHits || a.searchIncomplete != tc.incomplete || a.failed != tc.failed {
				t.Fatalf("state = hits:%d incomplete:%t failed:%t message:%q", len(a.searchResults), a.searchIncomplete, a.failed, a.message)
			}
			if tc.incomplete && !tc.failed {
				if a.searchFailedBoards != 1 {
					t.Fatalf("failed boards = %d, want 1", a.searchFailedBoards)
				}
				if a.clicks["search-incomplete-retry"] == nil {
					t.Fatal("incomplete search did not render a retry entry")
				}
				before := searchCalls.Load()
				a.searchEdit.SetText("draft")
				renderAppAt(t, a, 420, 700, 1)
				a.click("search-incomplete-retry").Click()
				renderAppAt(t, a, 420, 700, 1)
				waitForIdle(t, a)
				if searchCalls.Load() <= before {
					t.Fatal("incomplete retry entry did not repeat the search")
				}
				if got := a.searchEdit.Text(); got != "draft" {
					t.Fatalf("incomplete retry changed the draft to %q", got)
				}
				titlesMu.Lock()
				gotTitles := append([]string(nil), titles...)
				titlesMu.Unlock()
				for i, title := range gotTitles {
					if title != "needle" {
						t.Fatalf("incomplete retry request %d used %q, want needle", i, title)
					}
				}
			} else if a.clicks["search-incomplete-retry"] != nil {
				t.Fatal("complete or failed search rendered the incomplete retry entry")
			}
			if tc.failed && a.clicks["retry-status"] == nil {
				t.Fatal("all-board failure did not render the request retry")
			}
			if !tc.incomplete && !tc.failed && len(a.searchResults) == 0 && a.clicks["search-incomplete-retry"] != nil {
				t.Fatal("complete empty search rendered the incomplete entry")
			}
		})
	}
}

func TestAndroidSearchQueryPreservesCommittedPageAndQueryAfterPartialFailure(t *testing.T) {
	server := newSearchBoardServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.URL.Query().Get("board")) == "Advice" {
			http.Error(w, "board unavailable", http.StatusBadGateway)
			return
		}
		_, _ = io.WriteString(w, `{"article":[{"id":7,"group_id":7,"board_name":"Go","title":"部分结果"}],"pagination":{"page_all_count":1,"item_all_count":1}}`)
	})
	a := interactionAppWithBackend(t, server.URL, nil)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"
	a.searchEdit.SetText(" 北邮 ")
	a.loadSearch()
	waitForIdle(t, a)
	if !a.searchIncomplete || a.searchListQuery != "北邮" || a.current().Number != 1 {
		t.Fatalf("partial search state: incomplete=%t query=%q page=%d", a.searchIncomplete, a.searchListQuery, a.current().Number)
	}
}

func TestIncompleteSearchEmptyRemainsReachableInAShortViewport(t *testing.T) {
	a := previewApp(t)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"
	a.searchHasRun = true
	a.searchEdit.SetText("needle")
	a.searchListQuery = "needle"
	a.searchIncomplete = true
	a.searchFailedBoards = 1
	a.clicks = map[string]*widget.Clickable{}
	renderAppAt(t, a, 320, 420, 1.5)
	if a.clicks["search-incomplete-retry"] == nil {
		t.Fatalf("squeezed incomplete state dropped its retry action; clicks=%d", len(a.clicks))
	}
	if a.emptyList.List.Position.Length <= 0 {
		t.Fatal("squeezed incomplete state is not scrollable")
	}
}

func TestAndroidSearchPageRetryUsesSubmittedQueryAfterEditorEdit(t *testing.T) {
	var mu sync.Mutex
	counts := []string{}
	titles := []string{}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	phase := "fail"
	server := newSearchBoardServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		mu.Lock()
		counts = append(counts, q.Get("count"))
		titles = append(titles, q.Get("title1"))
		currentPhase := phase
		mu.Unlock()
		if q.Get("count") == "60" {
			select {
			case started <- struct{}{}:
			default:
			}
			if currentPhase == "fail" {
				<-release
				http.Error(w, "page unavailable", http.StatusBadGateway)
				return
			}
			if q.Get("board") == "Advice" {
				_, _ = io.WriteString(w, `{"article":[],"pagination":{}}`)
				return
			}
			var body strings.Builder
			body.WriteString(`{"article":[`)
			for i := 1; i <= 60; i++ {
				if i > 1 {
					body.WriteByte(',')
				}
				fmt.Fprintf(&body, `{"id":%d,"group_id":%d,"board_name":"Go","title":"第二页 %d","post_time":%d}`, i, i, i, 1700000000+i)
			}
			body.WriteString(`],"pagination":{"page_all_count":2,"page_current_count":2,"item_all_count":60}}`)
			_, _ = io.WriteString(w, body.String())
			return
		}
		_, _ = io.WriteString(w, `{"article":[{"id":1,"group_id":1,"board_name":"Go","title":"第一页结果"}],"pagination":{"page_all_count":2,"page_current_count":1,"item_all_count":60}}`)
	})
	t.Cleanup(unblock)
	a := interactionAppWithBackend(t, server.URL, nil)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"
	a.searchHasRun = true
	a.searchEdit.SetText("needle")
	a.searchListQuery = "needle"
	a.searchResults = make([]forum.Article, 20)
	for i := range a.searchResults {
		a.searchResults[i] = forum.Article{ID: forum.Scalar(fmt.Sprintf("%d", i+1)), Board: "Go", Title: fmt.Sprintf("第一页结果 %d", i+1)}
	}
	a.searchPagination = forum.Pagination{CurrentPage: 1, TotalPages: 2, TotalItems: 60}
	key := searchListKey(a.current(), "needle")
	list := a.mobileList(key)
	renderAppAt(t, a, 420, 700, 1)
	list.Position.First = 20
	list.Position.Offset = 9
	renderAppAt(t, a, 420, 700, 1)
	before := list.Position
	if before.First == 0 {
		t.Fatalf("fixture did not scroll: %+v", before)
	}

	a.click("pager-next:1").Click()
	renderAppAt(t, a, 420, 700, 1)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("page two request did not start")
	}
	unblock()
	waitForIdle(t, a)
	if !a.failed || a.current().Number != 1 || list.Position.First != before.First || list.Position.Offset != before.Offset {
		t.Fatalf("failed page state = failed:%t page:%d pos:%+v want %+v", a.failed, a.current().Number, list.Position, before)
	}

	mu.Lock()
	phase = "success"
	mu.Unlock()
	a.searchEdit.SetText("draft")
	renderAppAt(t, a, 420, 700, 1)
	if a.current().Number != 1 || len(a.searchResults) != 20 || list.Position.First != before.First || list.Position.Offset != before.Offset {
		t.Fatalf("editing the draft changed page one: page=%d results=%d pos=%+v", a.current().Number, len(a.searchResults), list.Position)
	}
	a.click("retry-status").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	if a.failed || a.current().Number != 2 || len(a.searchResults) != 30 || a.searchEdit.Text() != "draft" {
		t.Fatalf("retry result = failed:%t page:%d results:%d draft:%q", a.failed, a.current().Number, len(a.searchResults), a.searchEdit.Text())
	}
	mu.Lock()
	gotCounts := append([]string(nil), counts...)
	gotTitles := append([]string(nil), titles...)
	mu.Unlock()
	if len(gotCounts) < 4 {
		t.Fatalf("page two requests = counts %v titles %v", gotCounts, gotTitles)
	}
	for i, count := range gotCounts {
		if count != "60" || gotTitles[i] != "needle" {
			t.Fatalf("page retry request %d = count %q title %q, want 60/needle", i, count, gotTitles[i])
		}
	}
}

func TestAndroidUnauthenticatedSearchFailureShowsLoginExpired(t *testing.T) {
	a := previewApp(t)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.fail(forum.ErrUnauthenticated)
	if !a.failed || a.message != "登录已失效，请重新登录" || a.current().Page != PageLogin {
		t.Fatalf("authentication failure state = failed:%t message:%q page:%v", a.failed, a.message, a.current().Page)
	}
}

func TestAndroidDelayedSearchSuccessSurvivesDraftEdit(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	server := newSearchBoardServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		if r.URL.Query().Get("board") == "Advice" {
			_, _ = io.WriteString(w, `{"article":[],"pagination":{}}`)
			return
		}
		_, _ = io.WriteString(w, `{"article":[{"id":1,"group_id":1,"board_name":"Go","title":"needle 命中"}],"pagination":{"page_all_count":1,"item_all_count":1}}`)
	})
	t.Cleanup(unblock)
	a := interactionAppWithBackend(t, server.URL, nil)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"
	renderAppAt(t, a, 420, 700, 1)
	a.searchEdit.SetText("needle")
	a.click("search-submit").Click()
	renderAppAt(t, a, 420, 700, 1)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("initial search did not start")
	}
	a.searchEdit.SetText("draft")
	renderAppAt(t, a, 420, 700, 1)
	unblock()
	waitForIdle(t, a)
	if a.searchListQuery != "needle" || len(a.searchResults) != 1 || a.searchResults[0].Title != "needle 命中" {
		t.Fatalf("delayed result = query:%q results:%+v", a.searchListQuery, a.searchResults)
	}
	if got := a.searchEdit.Text(); got != "draft" {
		t.Fatalf("editor draft = %q, want draft", got)
	}
	if a.current().Number != 1 {
		t.Fatalf("delayed result page = %d, want 1", a.current().Number)
	}
}

func TestAndroidSearchStatusRetryUsesSubmittedQueryAfterFailure(t *testing.T) {
	var mu sync.Mutex
	phase := "fail"
	titles := []string{}
	counts := []string{}
	server := newSearchBoardServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentPhase := phase
		titles = append(titles, r.URL.Query().Get("title1"))
		counts = append(counts, r.URL.Query().Get("count"))
		mu.Unlock()
		if currentPhase == "fail" {
			http.Error(w, "search unavailable", http.StatusBadGateway)
			return
		}
		if r.URL.Query().Get("board") == "Advice" {
			_, _ = io.WriteString(w, `{"article":[],"pagination":{}}`)
			return
		}
		_, _ = io.WriteString(w, `{"article":[{"id":1,"group_id":1,"board_name":"Go","title":"重试命中"}],"pagination":{"page_all_count":1,"item_all_count":1}}`)
	})
	a := interactionAppWithBackend(t, server.URL, nil)
	defer a.Close()
	a.stack = []Route{{Page: PageSearch, Number: 1}}
	a.searchMode = "all"
	renderAppAt(t, a, 420, 700, 1)
	a.searchEdit.SetText("needle")
	a.click("search-submit").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	if !a.failed || a.searchRetryQuery != "needle" || a.searchRetryPage != 1 {
		t.Fatalf("failed search retry binding = failed:%t query:%q page:%d", a.failed, a.searchRetryQuery, a.searchRetryPage)
	}

	mu.Lock()
	phase = "success"
	mu.Unlock()
	a.searchEdit.SetText("draft")
	renderAppAt(t, a, 420, 700, 1)
	a.click("retry-status").Click()
	renderAppAt(t, a, 420, 700, 1)
	waitForIdle(t, a)
	if a.failed || len(a.searchResults) != 1 || a.searchResults[0].Title != "重试命中" || a.searchEdit.Text() != "draft" {
		t.Fatalf("retry result = failed:%t results:%+v draft:%q", a.failed, a.searchResults, a.searchEdit.Text())
	}
	mu.Lock()
	gotTitles := append([]string(nil), titles...)
	gotCounts := append([]string(nil), counts...)
	mu.Unlock()
	if len(gotTitles) < 4 {
		t.Fatalf("search requests = titles %v counts %v", gotTitles, gotCounts)
	}
	for i := range gotTitles {
		if gotTitles[i] != "needle" || gotCounts[i] != "30" {
			t.Fatalf("search request %d = title %q count %q, want needle/30", i, gotTitles[i], gotCounts[i])
		}
	}
}
