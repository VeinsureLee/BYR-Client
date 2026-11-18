# Android Global Search Session Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Android full-site search issue one authenticated server-side search request, remain on the search page for ordinary failures, and avoid mutating the active Gio list during its own layout callback.

**Architecture:** Preserve `application.App.Search` as the cross-platform use case, but pass an empty board directly to the official `/open/search/article.json` contract instead of expanding the complete section tree and querying every board. Keep Android session routing unchanged: only `forum.ErrUnauthenticated` may navigate to login; normal search errors stay retryable on `PageSearch`. Apply search results and reset list state together in the posted completion callback, which runs before the next frame layout.

**Tech Stack:** Go 1.27.4, standard-library HTTP client and `httptest`, shared `core/application` and `core/forum`, Gio v0.10.3, `go test`, `go vet`.

## Global Constraints

- Preserve every existing unstaged and untracked change on branch `feat/cross-platform-search`; do not stage or commit unless the user separately asks, because all search files are current uncommitted work.
- A blank `SearchRequest.Board` means full-site search and must be sent as `board=` to the official search endpoint.
- One search submission must issue exactly one `/open/search/article.json` request and no section-tree requests.
- Keep server-side pagination; do not fetch every board, aggregate all results in memory, or locally synthesize pagination.
- Do not broaden authentication-error classification. HTTP 500, `forum.APIError`, decode errors, and network errors must remain on `PageSearch` and retain the session.
- Tests must use local fixtures only; optional real-account verification is read-only and must not be part of automated tests.
- Keep `apps/android/app` outside the root workspace and run its commands with `GOWORK=off`.

## File Structure

- `core/application/search.go`: validate shared search input and make one remote call for both board-specific and full-site searches.
- `core/application/search_test.go`: prove full-site request count, query, result propagation, and absence of section fan-out.
- `core/forum/endpoints_test.go`: lock the empty-board wire contract, including auth and pagination query fields.
- `apps/android/app/internal/ui/search.go`: defer `searchList` replacement until the asynchronous result is applied.
- `apps/android/app/internal/ui/navigation_test.go`: cover full-site request count, route/session retention, render safety, and ordinary failure behavior.

---

### Task 1: Replace all-board fan-out with the official single-request contract

**Files:**
- Modify: `core/application/search.go:3-10,21-24,43-306`
- Test: `core/application/search_test.go:3-12,71-134`
- Test: `core/forum/endpoints_test.go:52-60`

**Interfaces:**
- Consumes: `forum.Client.SearchArticles(ctx, forum.Search)`; an empty `forum.Search.Board` is the full-site filter.
- Produces: unchanged `Search(ctx, SearchRequest) (SearchResult, error)` and `SearchResult` types, with remote pagination passed through verbatim.

- [ ] **Step 1: Replace the fan-out test with a failing single-request contract test**

Replace `TestSearchExpandsAllBoardsWhenTheAPINeedsABoard` in `core/application/search_test.go` with:

```go
func TestSearchUsesSingleRemoteRequestForAllSite(t *testing.T) {
    var mu sync.Mutex
    requests := make([]string, 0, 1)
    var got url.Values
    a := appFor(t, func(w http.ResponseWriter, r *http.Request) {
        mu.Lock()
        requests = append(requests, r.URL.Path)
        mu.Unlock()
        if r.URL.Path != "/open/search/article.json" {
            http.Error(w, "unexpected endpoint", http.StatusInternalServerError)
            return
        }
        got = r.URL.Query()
        io.WriteString(w, `{"article":[{"id":42,"group_id":42,"board_name":"Go","title":"全站结果","user":{"id":"alice"}}],"pagination":{"page_all_count":4,"page_current_count":2,"item_all_count":91}}`)
    }, nil, &credentials{value: Credential{Username: "alice", Token: "token"}})

    result, err := a.Search(context.Background(), SearchRequest{
        Keyword: "  北邮  ",
        Page:    2,
        Size:    30,
    })
    if err != nil {
        t.Fatal(err)
    }

    mu.Lock()
    gotRequests := append([]string(nil), requests...)
    mu.Unlock()
    if !reflect.DeepEqual(gotRequests, []string{"/open/search/article.json"}) {
        t.Fatalf("requests = %v, want one full-site search", gotRequests)
    }
    wantQuery := url.Values{
        "oauth_token": {"token"}, "board": {""}, "title1": {"北邮"},
        "title2": {""}, "titlen": {""}, "author": {""}, "day": {""},
        "m": {""}, "a": {""}, "o": {"1"}, "page": {"2"}, "count": {"30"},
    }
    if !reflect.DeepEqual(got, wantQuery) {
        t.Fatalf("query = %v, want %v", got, wantQuery)
    }
    if len(result.Articles) != 1 || result.Articles[0].Title != "全站结果" {
        t.Fatalf("result = %+v", result)
    }
    if result.Pagination.CurrentPage != 2 || result.Pagination.TotalPages != 4 || result.Pagination.TotalItems != 91 {
        t.Fatalf("pagination = %+v", result.Pagination)
    }
    if result.Source != SearchSourceRemote || result.Truncated {
        t.Fatalf("source=%q truncated=%t", result.Source, result.Truncated)
    }
}
```

Keep the existing `sync` import because this test protects request recording if the implementation regresses to concurrent fan-out.

- [ ] **Step 2: Run the focused application test and verify fan-out is exposed**

Run:

```sh
go -C core test ./application -run TestSearchUsesSingleRemoteRequestForAllSite -count=1
```

Expected: FAIL after the first request hits `/open/section.json` rather than `/open/search/article.json`.

- [ ] **Step 3: Send the blank board directly to `SearchArticles` and delete fan-out code**

Update the `SearchRequest` comment to:

```go
// SearchRequest is the cross-platform article-search contract. A blank Board
// asks the remote endpoint to search the full site.
```

Replace the final board branch in `Search` with:

```go
board := strings.TrimSpace(request.Board)
return a.searchBoard(ctx, keyword, author, board, page, size)
```

Delete these now-unused declarations from `search.go`:

```text
boardSearchResult
(*App).searchAllBoards
(*App).searchableBoards
searchBoards
articleTime
```

Remove the now-unused `sort`, `strconv`, and `sync` imports from `core/application/search.go`. Keep `SearchResult.Truncated` for API compatibility; direct server pagination leaves it `false`.

- [ ] **Step 4: Add an explicit empty-board endpoint contract**

Add this case next to `article search` in `TestMigratedEndpointContracts`:

```go
{
    name:   "article full-site search",
    method: "GET",
    path:   "/open/search/article.json",
    urlQuery: url.Values{
        "board": {""}, "title1": {"北邮"}, "title2": {""}, "titlen": {""},
        "author": {""}, "day": {""}, "m": {""}, "a": {""}, "o": {"1"},
        "page": {"2"}, "count": {"30"},
    },
    call: func(c *Client) error {
        _, err := c.SearchArticles(ctx, Search{
            Title: "北邮",
            Page:  Page{Number: 2, Size: 30},
        })
        return err
    },
},
```

This locks in `board=` rather than allowing the UI to rediscover board lists.

- [ ] **Step 5: Run shared-core tests and vet**

Run:

```sh
go -C core test ./application ./forum -run 'Test(Search|MigratedEndpointContracts)' -count=1
go -C core test ./...
go -C core vet ./...
```

Expected: a full-site query makes one authenticated request, preserves remote pagination, and all core checks pass.

- [ ] **Step 6: Record the shared-core checkpoint without staging the surrounding search work**

Run:

```sh
git diff --check -- core/application/search.go core/application/search_test.go core/forum/endpoints_test.go
git diff -- core/application/search.go core/application/search_test.go core/forum/endpoints_test.go
```

Expected: the intended delta removes fan-out, replaces its test with a one-request contract, and adds the empty-board endpoint case. Do not stage or commit by default because these files are part of the existing uncommitted search feature. If a commit is later requested, stage only the reviewed search hunks and use `fix(search): use server-side full-site query` with the required Claude Code co-author line.

---

### Task 2: Apply Android search list resets between frames and retain the authenticated route

**Files:**
- Modify: `apps/android/app/internal/ui/search.go:18-49`
- Test: `apps/android/app/internal/ui/navigation_test.go:16-58`

**Interfaces:**
- Consumes: Task 1's one-request `application.App.Search` behavior and Android `run`/`drain` completion queue.
- Produces: `loadSearch()` that leaves the active `widget.List` untouched during the click callback, then resets it only when accepted results are applied.

- [ ] **Step 1: Add a failing test for synchronous list replacement**

Add this test after `TestSearchPageUsesRemoteSearchForAllMode`:

```go
func TestSearchListResetWaitsForResultApplication(t *testing.T) {
    s := newInteractionServer(t)
    a := interactionApp(t, s.server.URL)
    defer a.Close()
    a.stack = []Route{{Page: PageSearch}}
    a.searchMode = "all"
    a.searchEdit.SetText("搜索")
    a.searchList.Position.First = 4

    a.loadSearch()
    if got := a.searchList.Position.First; got != 4 {
        t.Fatalf("search list reset synchronously to %d", got)
    }

    waitForIdle(t, a)
    if got := a.searchList.Position.First; got != 0 {
        t.Fatalf("search list position after applying result = %d, want 0", got)
    }
}
```

- [ ] **Step 2: Strengthen the existing full-site UI test with route, session, and request-budget assertions**

After `waitForIdle(t, a)` in `TestSearchPageUsesRemoteSearchForAllMode`, add:

```go
if got := a.current().Page; got != PageSearch {
    t.Fatalf("page after search = %v, want search", got)
}
if a.renderPanic != "" {
    t.Fatalf("search render panic = %q", a.renderPanic)
}
if got := a.core.Session().Token; got != "token" {
    t.Fatalf("search discarded session token %q", got)
}
if len(s.find("GET", "/open/section.json")) != 0 || len(s.find("GET", "/open/section/0.json")) != 0 {
    t.Fatalf("search expanded section tree: %+v", s.requests)
}
```

Then render one more frame and assert the route remains stable:

```go
ops.Reset()
a.Layout(gtx)
if got := a.current().Page; got != PageSearch || a.renderPanic != "" {
    t.Fatalf("second search frame page=%v panic=%q", got, a.renderPanic)
}
```

Add this ordinary-error regression test to the same file:

```go
func TestSearchFailureKeepsAuthenticatedSearchRoute(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/open/search/article.json" {
            http.Error(w, "search unavailable", http.StatusInternalServerError)
            return
        }
        http.NotFound(w, r)
    }))
    defer server.Close()

    a := interactionApp(t, server.URL)
    defer a.Close()
    a.stack = []Route{{Page: PageSearch}}
    a.searchMode = "all"
    a.searchEdit.SetText("搜索")

    a.loadSearch()
    waitForIdle(t, a)

    if got := a.current().Page; got != PageSearch {
        t.Fatalf("ordinary search error changed page to %v", got)
    }
    if !a.failed || a.message == "" {
        t.Fatalf("search failure was not exposed: failed=%t message=%q", a.failed, a.message)
    }
    if got := a.core.Session().Token; got != "token" {
        t.Fatalf("ordinary search error discarded token %q", got)
    }
}
```

- [ ] **Step 3: Run the deferred-reset test and verify current code fails**

Run:

```sh
GOWORK=off go -C apps/android/app test ./internal/ui -run TestSearchListResetWaitsForResultApplication -count=1
```

Expected: FAIL because `loadSearch()` currently replaces `a.searchList` before it enters the asynchronous completion queue.

- [ ] **Step 4: Move list replacement into the accepted result callback**

Remove this line from the synchronous portion of `loadSearch`:

```go
a.searchList = widget.List{List: layout.List{Axis: layout.Vertical}}
```

Add the same assignment after the stale-route guard in the returned completion callback:

```go
return func() {
    if a.current() != route {
        return
    }
    a.searchList = widget.List{List: layout.List{Axis: layout.Vertical}}
    a.searchResults = result.Articles
    a.searchPagination = result.Pagination
}, err
```

The callback is executed by `a.drain()` before page layout on a later frame, so the list is never replaced from inside its own item callback. Keep the `layout` and `widget` imports because the callback still constructs the list.

- [ ] **Step 5: Run focused Android search tests and the full project check**

Run:

```sh
GOWORK=off go -C apps/android/app test ./internal/ui -run 'TestSearch(PageUsesRemoteSearchForAllMode|ListResetWaitsForResultApplication|FailureKeepsAuthenticatedSearchRoute)' -count=1
go run ./tools/dev check --scope all
```

Expected: one search request, zero section requests, no render panic, ordinary errors remain on `PageSearch`, the token is retained, and all tests/vet checks pass.

- [ ] **Step 6: Record the Android search checkpoint without staging unrelated work**

Run:

```sh
git diff --check -- apps/android/app/internal/ui/search.go apps/android/app/internal/ui/navigation_test.go
git diff -- apps/android/app/internal/ui/search.go apps/android/app/internal/ui/navigation_test.go
```

Expected: only the deferred list replacement and the search route/session regressions belong to this task. Do not stage or commit by default because both target files are part of the current uncommitted search implementation. If a commit is later requested, stage only these reviewed hunks and use `fix(android): keep global search session stable` with the required Claude Code co-author line.
