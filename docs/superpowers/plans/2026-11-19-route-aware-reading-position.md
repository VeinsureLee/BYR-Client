# 跨页面阅读位置恢复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 PC 与 Android 在版面/搜索结果进入主题后返回时恢复原列表阅读位置，并为不同主题、分页和搜索词隔离滚动状态。

**Architecture:** 阅读位置是 UI 导航状态，不进入 `core/local`，也不写磁盘。两端都使用稳定的 `ListKey` 将列表状态从当前页面树中提升到 `App` 生命周期：PC 保存 MyGo `*ui.ListState`，Android 保存 Gio `*widget.List`；网络数据仍按现有 Route 异步加载，返回时只恢复对应 key 的列表状态。

**Tech Stack:** Go 1.27.4、MyGo v0.9.1 `ui.ListState`、Gio v0.10.3 `widget.List`/`layout.Position`、现有 `Route`/`back` 导航、`go test`、`go vet`。

## Global Constraints

- 保留当前 `f166989` / `v0.6.0` 的搜索、点赞和安装包改动；不要 reset、checkout 或覆盖其他工作。
- `Route.Number` 只表示服务端分页号，不能代替滚动位置。
- 阅读位置只在当前 App 生命周期内保存，不写 `core/local`，不改变阅读历史语义。
- 新搜索词、新分页、新版面、新主题使用新的 `ListKey`，位置从顶部开始；返回旧 Route 时恢复旧 key。
- 新异步数据返回时必须保留现有 stale-route 检查；不能把旧请求结果写入当前页面或错误的列表状态。
- Gio 列表只能在下一帧布局前重置，不能在自身 item callback 内重建。
- 测试只使用本地 fixture；必须同时断言分页号和视觉位置，不能只断言页面/主题 ID。

## File Structure

- Create: `apps/desktop/internal/desktop/list_state.go` — `ListKey`、PC 列表状态缓存和稳定行 key。
- Modify: `apps/desktop/internal/desktop/app.go` — App 状态、初始化、Route/搜索 key 辅助。
- Modify: `apps/desktop/internal/desktop/browse.go` — 版面、时间线、搜索共享文章列表使用持久状态。
- Modify: `apps/desktop/internal/desktop/thread.go` — 主题楼层列表使用持久状态。
- Modify: `apps/desktop/internal/desktop/search.go` — 新搜索/换页生成新 list key，返回旧搜索不清除状态。
- Test: `apps/desktop/internal/desktop/app_test.go` — PC 版面和搜索返回位置。
- Create: `apps/android/app/internal/ui/list_state.go` — Android `ListKey` 与 `widget.List` 缓存。
- Modify: `apps/android/app/internal/ui/app.go` — App 缓存、Route key 和 back 生命周期。
- Modify: `apps/android/app/internal/ui/route_state.go` — 清理网络模型但不销毁 key 对应列表状态。
- Modify: `apps/android/app/internal/ui/pages.go` — 版面/主题列表和分页 key。
- Modify: `apps/android/app/internal/ui/search.go` — 搜索词 key 与新结果重置语义。
- Test: `apps/android/app/internal/ui/route_state_test.go`、`navigation_test.go` — 版面/主题/搜索返回和分页位置。

---

### Task 1: 定义两端稳定的列表身份和状态容器

**Files:**
- Create: `apps/desktop/internal/desktop/list_state.go`
- Modify: `apps/desktop/internal/desktop/app.go:31-93,120-170`
- Create: `apps/android/app/internal/ui/list_state.go`
- Modify: `apps/android/app/internal/ui/app.go:63-153,164-193`

**Interfaces:**
- Produces desktop `type listKey struct { Page Page; Name, ID, Sub, Query string; Number int }` and `func (a *App) desktopListState(key listKey) *ui.ListState`.
- Produces Android `type listKey struct { Page Page; Name, ID, Sub, Query string; Number int }` and `func (a *App) mobileList(key listKey) *widget.List`.
- Consumes existing Route fields; no changes to `core/`.

- [ ] **Step 1: Add failing state-isolation tests**

Add these tests before implementing the caches:

```go
func TestDesktopListStateIsolatedByRouteAndPage(t *testing.T) {
    a := newHarness(t).app
    first := a.desktopListState(listKey{Page: "board", Name: "Go", Number: 1})
    first.Key = func(int) any { return "first" }
    second := a.desktopListState(listKey{Page: "board", Name: "Go", Number: 2})
    if first == second {
        t.Fatal("different pages shared desktop list state")
    }
    if got := a.desktopListState(listKey{Page: "board", Name: "Go", Number: 1}); got != first {
        t.Fatal("returning to page 1 did not reuse desktop list state")
    }
}
```

In Android `route_state_test.go`, add:

```go
func TestMobileListStateIsolatedByRoute(t *testing.T) {
    a := startAppForTest(t)
    first := a.mobileList(listKey{Page: PageBoard, Name: "Go", Number: 1})
    first.Position.First = 7
    first.Position.Offset = 13
    second := a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "42", Number: 1})
    if first == second {
        t.Fatal("board and thread shared mobile list state")
    }
    got := a.mobileList(listKey{Page: PageBoard, Name: "Go", Number: 1})
    if got.Position.First != 7 || got.Position.Offset != 13 {
        t.Fatalf("board position = %+v", got.Position)
    }
}
```

- [ ] **Step 2: Run focused tests and verify they fail**

```sh
go -C apps/desktop test ./internal/desktop -run TestDesktopListStateIsolatedByRoute -count=1
GOWORK=off go -C apps/android/app test ./internal/ui -run TestMobileListStateIsolatedByRoute -count=1
```

Expected: FAIL because neither `desktopListState` nor `mobileList` exists.

- [ ] **Step 3: Implement desktop state cache**

Create `list_state.go` with this concrete shape:

```go
type listKey struct {
    Page Page
    Name, ID, Sub, Query string
    Number int
}

func (a *App) desktopListState(key listKey) *ui.ListState {
    if a.listStates == nil {
        a.listStates = make(map[listKey]*ui.ListState)
    }
    state := a.listStates[key]
    if state == nil {
        state = &ui.ListState{}
        a.listStates[key] = state
    }
    return state
}
```

Add `listStates map[listKey]*ui.ListState` to the desktop `App`, initialize it in `New`, and keep it account/session scoped in memory. `listKey` is package-private so it cannot become a cross-platform business contract.

- [ ] **Step 4: Implement Android state cache**

Create `apps/android/app/internal/ui/list_state.go`:

```go
type listKey struct {
    Page Page
    Name, ID, Sub, Query string
    Number int
}

func (a *App) mobileList(key listKey) *widget.List {
    if a.listStates == nil {
        a.listStates = make(map[listKey]*widget.List)
    }
    if state := a.listStates[key]; state != nil {
        return state
    }
    state := &widget.List{List: layout.List{Axis: layout.Vertical}}
    a.listStates[key] = state
    return state
}
```

Add `listStates map[listKey]*widget.List` to `App`, initialize it in `New`, and clear the map only in `resetSessionUI()` after logout. Do not add it to `route_state.go` clear logic.

- [ ] **Step 5: Run focused state tests**

```sh
go -C apps/desktop test ./internal/desktop -run TestDesktopListStateIsolatedByRoute -count=1
GOWORK=off go -C apps/android/app test ./internal/ui -run TestMobileListStateIsolatedByRoute -count=1
```

Expected: PASS, with distinct state for board page 1, board page 2, and thread page while returning to the same key reuses `First`/`Offset`.

---

### Task 2: Wire PC board/search/thread lists to route-owned state

**Files:**
- Modify: `apps/desktop/internal/desktop/browse.go:288-331,333-349`
- Modify: `apps/desktop/internal/desktop/thread.go:41-52`
- Modify: `apps/desktop/internal/desktop/search.go:13-37,39-81`
- Modify: `apps/desktop/internal/desktop/app.go:440-568,612-665`
- Test: `apps/desktop/internal/desktop/app_test.go`

**Interfaces:**
- Consumes `desktopListState(listKey)` from Task 1.
- Produces stable list keys: `PageBoard + board + page`, `PageSearch + query + page`, `PageThread + board + thread ID + page`.

- [ ] **Step 1: Add failing PC return-position tests**

Add a fixture with three board articles and three search articles. Add tests using the real navigation callbacks:

```go
func TestBoardReturnRestoresListState(t *testing.T) {
    h := newHarness(t)
    h.login()
    h.app.board.Articles = []forum.Article{
        {ID: "1", Board: "Go", Title: "第一主题"},
        {ID: "2", Board: "Go", Title: "第二主题"},
        {ID: "3", Board: "Go", Title: "第三主题"},
    }
    h.app.route = Route{Page: "board", Name: "Go", Number: 1}
    h.view.Frame()
    state := h.app.desktopListState(listKey{Page: "board", Name: "Go", Number: 1})
    state.Handle.ScrollTo(2)
    h.app.openArticle(h.app.board.Articles[2])
    h.app.goBack()
    h.view.Frame()
    if h.app.route.Page != "board" || h.app.route.Name != "Go" {
        t.Fatalf("route after back = %+v", h.app.route)
    }
    if got := h.app.desktopListState(listKey{Page: "board", Name: "Go", Number: 1}); got != state {
        t.Fatal("board list state was replaced")
    }
}
```

Add the equivalent search test, asserting `searchQuery` and `searchResults` remain unchanged and the key includes the query:

```go
func TestSearchReturnKeepsQueryAndListState(t *testing.T) {
    h := newHarness(t)
    h.login()
    h.app.route = Route{Page: "search", Number: 1}
    h.app.searchQuery = "搜索"
    h.app.searchHasRun = true
    h.app.searchResults = []forum.Article{{ID: "42", Board: "Go", Title: "搜索主题"}}
    state := h.app.desktopListState(listKey{Page: "search", Query: "搜索", Number: 1})
    state.Handle.ScrollTo(1)
    h.app.openArticle(h.app.searchResults[0])
    h.app.goBack()
    h.view.Frame()
    if h.app.route.Page != "search" || h.app.searchQuery != "搜索" || len(h.app.searchResults) != 1 {
        t.Fatalf("search state after back: route=%+v query=%q results=%d", h.app.route, h.app.searchQuery, len(h.app.searchResults))
    }
    if h.app.desktopListState(listKey{Page: "search", Query: "搜索", Number: 1}) != state {
        t.Fatal("search list state was replaced")
    }
}
```

- [ ] **Step 2: Run the tests and verify the position/state contract fails**

```sh
go -C apps/desktop test ./internal/desktop -run 'Test(BoardReturnRestoresListState|SearchReturnKeepsQueryAndListState)' -count=1
```

Expected: FAIL because `articleList` and `threadView` pass `nil` to `ui.List`, and search/thread page transitions do not own reusable list states.

- [ ] **Step 3: Pass stable state and row keys into PC lists**

Change `articleList` from:

```go
ui.List(c.Key("articles-"+string(a.route.Page)+a.route.Name+strconv.Itoa(a.route.Number)), nil, len(articles), func(i int) {
```

to the equivalent keyed state call:

```go
key := listKey{Page: a.route.Page, Name: a.route.Name, Sub: a.route.Sub, Number: a.route.Number}
if a.route.Page == "search" {
    key.Query = strings.TrimSpace(a.searchQuery)
}
state := a.desktopListState(key)
state.Key = func(i int) any {
    if i >= 0 && i < len(articles) {
        return string(articles[i].Board) + "\x00" + string(articles[i].ID)
    }
    return i
}
ui.List(c.Key("articles-"+string(a.route.Page)+a.route.Name+strconv.Itoa(a.route.Number)), state, len(articles), func(i int) {
```

Change `threadView` to pass:

```go
key := listKey{Page: a.route.Page, Name: a.route.Name, ID: a.route.ID, Number: a.route.Number}
state := a.desktopListState(key)
state.Key = func(i int) any { return string(a.thread.Articles[i].ID) }
ui.List(c.Key("thread-"+a.route.Name+"-"+a.route.ID+"-"+strconv.Itoa(a.route.Number)), state, len(a.thread.Articles), func(i int) {
```

Use a fresh key on `loadSearch` when the trimmed query changes or the pager changes; do not clear the existing state when returning from a thread. Keep `goBack()`’s existing route reload behavior.

- [ ] **Step 4: Make PC search pagination explicitly create a new page key**

Keep `Route.Number` as the server page. In the search pager callback, set `a.route.Number = page`, clear only the new page’s `searchPagination`/results as current code does, and never reuse the previous page’s list state. The key `{PageSearch, Query, Number}` guarantees page 2 starts at top while returning to page 1 restores page 1.

- [ ] **Step 5: Run PC focused tests and full desktop checks**

```sh
go -C apps/desktop test ./internal/desktop -run 'Test(BoardReturnRestoresListState|SearchReturnKeepsQueryAndListState|TestThreadReactionsCollectionAndQuoteMatchReference)' -count=1
go -C apps/desktop vet ./...
```

Expected: PASS; opening a thread and returning preserves the originating board/search state without suppressing normal network reloads.

---

### Task 3: Wire Android board/thread/search lists and nested route positions

**Files:**
- Modify: `apps/android/app/internal/ui/route_state.go:13-92`
- Modify: `apps/android/app/internal/ui/pages.go:697-763,882-941,1795-1866`
- Modify: `apps/android/app/internal/ui/search.go:18-49,192-194`
- Modify: `apps/android/app/internal/ui/app.go:407-505,635-646`
- Test: `apps/android/app/internal/ui/route_state_test.go`
- Test: `apps/android/app/internal/ui/navigation_test.go`

**Interfaces:**
- Consumes `mobileList(listKey)` from Task 1.
- Produces route-owned `First`/`Offset` restoration for board, thread, and search pages.

- [ ] **Step 1: Add failing Android position tests**

Add a board return test:

```go
func TestBoardBackRestoresScrollPosition(t *testing.T) {
    a := startAppForTest(t)
    a.stack = []Route{{Page: PageHome}, {Page: PageBoard, Name: "Go", Number: 1}}
    key := listKey{Page: PageBoard, Name: "Go", Number: 1}
    list := a.mobileList(key)
    list.Position.First, list.Position.Offset = 7, 13
    a.stack = append(a.stack, Route{Page: PageThread, Name: "Go", ID: "42", Number: 1})
    a.back()
    got := a.mobileList(key).Position
    if got.First != 7 || got.Offset != 13 {
        t.Fatalf("board position after back = %+v", got)
    }
}
```

Add nested isolation:

```go
func TestThreadPositionsAreIsolatedByThreadRoute(t *testing.T) {
    a := startAppForTest(t)
    a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "42", Number: 1}).Position.First = 4
    a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "43", Number: 1}).Position.First = 9
    if got := a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "42", Number: 1}).Position.First; got != 4 {
        t.Fatalf("thread 42 position = %d", got)
    }
    if got := a.mobileList(listKey{Page: PageThread, Name: "Go", ID: "43", Number: 1}).Position.First; got != 9 {
        t.Fatalf("thread 43 position = %d", got)
    }
}
```

Extend the existing search navigation test to set `searchList.Position.First = 5`, open a thread, call `back`, and assert the same `First`, query, and results remain. Preserve `TestSearchListResetWaitsForResultApplication` to assert a genuinely new result resets to zero.

- [ ] **Step 2: Run focused Android tests and verify failure**

```sh
GOWORK=off go -C apps/android/app test ./internal/ui -run 'Test(BoardBackRestoresScrollPosition|ThreadPositionsAreIsolatedByThreadRoute|SearchReturnKeepsListPosition)' -count=1
```

Expected: FAIL because `clearRouteModel` reconstructs `boardList`/`threadList` and there is no route-keyed state.

- [ ] **Step 3: Use route keys for Android list rendering**

In `boardPage`, use:

```go
state := a.mobileList(listKey{Page: PageBoard, Name: a.current().Name, Sub: a.current().Sub, Number: a.current().Number})
return material.List(a.theme, state).Layout(...)
```

In `threadPage`, use `{PageThread, Name: current.Name, ID: current.ID, Number: current.Number}`. In `searchPage`, use `{PageSearch, Query: strings.TrimSpace(a.searchEdit.Text()), Number: current.Number}`.

The actual `widget.List` field passed to `material.List` must be the pointer returned by `mobileList`; do not copy the value into a local variable and then replace it during layout.

- [ ] **Step 4: Stop route cleanup from destroying list state**

Remove the `a.boardList = widget.List{...}` and `a.threadList = widget.List{...}` assignments from `clearRouteModel`. Keep clearing `a.board`, `a.thread`, `a.articles`, and other network models. `resetSessionUI` must clear `listStates` so a different account cannot inherit positions.

`restoreRoute` continues to reload the target route and the stale-route guards remain unchanged.

- [ ] **Step 5: Handle page changes explicitly**

When `changePage` changes `Route.Number`, do not reuse the old page’s state: the list key includes `Number`, so page 2 gets a zero `Position`. Returning to page 1 uses the original key and restores its saved position. Add an assertion for both `Number` and `Position.First`.

- [ ] **Step 6: Run Android focused tests and full checks**

```sh
GOWORK=off go -C apps/android/app test ./internal/ui -run 'Test(BoardBackRestoresScrollPosition|ThreadPositionsAreIsolatedByThreadRoute|SearchReturnKeepsListPosition|SearchListResetWaitsForResultApplication)' -count=1
go run ./tools/dev check --scope all
```

Expected: board/search/thread return positions are preserved, nested thread positions are isolated, new searches/pages start at zero, and all test/vet checks pass.

---

### Task 4: Preview and regression gate

**Files:**
- Modify: `apps/android/app/internal/ui/preview_test.go`
- Create: `build/android-previews/thread-scrolled.png` (generated, ignored)

- [ ] **Step 1: Add a preview fixture with multiple rows and a saved position**

Extend `TestPreviewScreens` with a deterministic 10-post thread and set the route-owned thread list position to `First: 4` before rendering `thread-scrolled`. The fixture must use stable IDs and no live network.

- [ ] **Step 2: Run preview generation**

```sh
cd apps/android/app
BYR_ANDROID_PREVIEW_DIR=../../../build/android-previews GOWORK=off go test ./internal/ui -run TestPreviewScreens -count=1
```

Expected: `thread.png` and `thread-scrolled.png` are non-empty and the scrolled preview shows a later floor rather than the first floor.

- [ ] **Step 3: Run final checks**

```sh
go run ./tools/dev check --scope all
git diff --check
git status --short
```

Expected: all tests/vet pass; only intentional source/test/docs changes remain; generated previews stay ignored.
