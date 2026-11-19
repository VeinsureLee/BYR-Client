# Android 主题详情与操作栏重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 参考官方 APK 的“通用 AppBar + 单一滚动内容流”设计，重做 Android 主题详情首屏，并在 PC/Android 两端删除重复主题操作、把收藏和复制链接放入右上角操作区、只保留一个版面跳转入口。

**Architecture:** Android 主题页改成一个包含版面 Chip、主题标题、必要摘要和楼层卡片的统一 `material.List`；主题页隐藏全局底部导航，顶部 AppBar 只显示通用标题和图标操作。主题级收藏固定在右上角，复制链接/刷新/管理/投票/竞猜进入可展开的更多菜单；楼层卡片继续负责楼层点赞、踩、回复和复制正文。PC 采用同一操作归属，保留现有 MyGo 控件而不引入新 UI 框架。

**Tech Stack:** Go 1.27.4、Gio v0.10.3、MyGo v0.9.1、共享 `core/forum` 模型和端点、Gio `widget.Clickable`/`material.List`、MyGo `ui.List`、本地 `httptest` 与预览截图。

## Global Constraints

- 保留当前 `v0.6.0` 已发布行为：搜索、匿名评价 0500 处理、canonical board、收藏和帖子内容解析不能回退。
- Android 主题详情不得继续显示全局底部导航；加载失败时不能渲染可操作的主题按钮。
- Android 主题标题只在滚动内容中显示一次；顶部 AppBar 使用固定通用标题“帖子”或“主题阅读”。
- 主题详情的第一楼仍然是可滚动楼层卡片；非第一楼保留点赞、踩、回复、复制正文。
- 主题级版面入口只保留可点击的版面信息 Chip；删除 `进入版面` 重复按钮。
- 收藏图标固定在 Android 主题页右上角；复制链接固定在右上角更多菜单中，不与楼层“复制正文”混淆。
- 投票/竞猜只有在明确确认对应资源存在且属于当前主题时才显示；不能对所有主题无条件显示入口。
- 测试使用本地 fixture；不对线上主题执行写操作；截图验证使用 `BYR_ANDROID_PREVIEW_DIR`。

## File Structure

- Create: `apps/android/app/internal/ui/thread_actions.go` — Android 主题更多菜单、操作可用性、主题级动作。
- Modify: `apps/android/app/internal/ui/app.go:691-710,905-961,963-1315` — 隐藏底部导航、通用标题、右上角收藏/更多图标和菜单状态。
- Modify: `apps/android/app/internal/ui/pages.go:921-1257` — 单一滚动主题列表、紧凑主题头部、根楼层重复操作清理、唯一版面入口。
- Modify: `apps/android/app/internal/ui/style.go` — 仅为主题详情增加紧凑触摸控件，不改变全局 pill 尺寸。
- Modify: `apps/desktop/internal/desktop/thread.go:54-146,217-244` — 删除重复版面/根楼层主题级入口，保留统一操作归属。
- Modify: `apps/desktop/internal/desktop/app.go` / `browse.go` — 统一 canonical board 的主题链接和版面入口。
- Test: `apps/android/app/internal/ui/interactions_test.go`、`navigation_test.go`、`preview_test.go`。
- Test: `apps/desktop/internal/desktop/app_test.go`。
- Optional Modify: `core/forum/models.go` and endpoint load helpers only if vote/bet availability metadata is added; do not invent fields without fixture evidence.

---

### Task 1: Establish the theme action policy and failing regression tests

**Files:**
- Test: `apps/android/app/internal/ui/interactions_test.go`
- Test: `apps/android/app/internal/ui/navigation_test.go`
- Test: `apps/desktop/internal/desktop/app_test.go`
- Modify: `apps/android/app/internal/ui/app.go` only for test-visible `threadMoreOpen` state if needed

**Interfaces:**
- Produces the behavior contract for `threadBoard` as the sole board navigation entry and for `threadMoreOpen` as the Android menu state.
- Consumes existing `thread`, `Route`, `sampleThread`, `newInteractionServer`, and desktop `h.view` helpers.

- [ ] **Step 1: Add Android duplicate-action tests**

Add a rendered theme fixture test that lays out a root article and asserts the root theme actions are not duplicated in the separate toolbar:

```go
func TestThreadPageHasSingleBoardEntryAndNoDuplicateThemeToolbar(t *testing.T) {
    a := interactionApp(t, "http://127.0.0.1:1")
    defer a.Close()
    a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
    a.thread = sampleThread()

    var router input.Router
    var ops op.Ops
    gtx := layout.Context{
        Ops: &ops, Source: router.Source(),
        Constraints: layout.Exact(image.Pt(420, 820)),
        Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
    }
    a.Layout(gtx)

    if a.click("thread-open-board").Clicked(gtx) {
        t.Fatal("legacy duplicate thread-open-board control is still active")
    }
    if a.click("thread-toolbar").Clicked(gtx) {
        t.Fatal("legacy fixed thread toolbar must not own duplicate root actions")
    }
    if got := a.current().Page; got != PageThread {
        t.Fatalf("theme page changed while checking controls: %v", got)
    }
}
```

The implementation should replace the click-key assertions with a stronger visible control count once the new renderer exists: exactly one `thread-board` key, no `thread-open-board`, and one root floor action row.

- [ ] **Step 2: Add Android top-right action tests**

```go
func TestThreadTopActionsCollectAndMore(t *testing.T) {
    a := interactionApp(t, "http://127.0.0.1:1")
    defer a.Close()
    a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "Go", ID: "42", Number: 1}}
    a.thread = sampleThread()
    a.thread.Collect = false

    var router input.Router
    var ops op.Ops
    gtx := layout.Context{Ops: &ops, Source: router.Source(), Constraints: layout.Exact(image.Pt(420, 820)), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
    a.Layout(gtx)
    a.click("thread-collect-top").Click()
    a.click("thread-more").Click()
    ops.Reset()
    a.Layout(gtx)

    if !a.threadMoreOpen {
        t.Fatal("thread more menu did not open")
    }
    if a.current().Page != PageThread {
        t.Fatal("more menu changed route")
    }
}
```

- [ ] **Step 3: Add PC action cleanup assertions**

Extend the desktop thread harness test:

```go
func TestDesktopThreadHasOneBoardEntryAndNoDuplicateRootActions(t *testing.T) {
    h := newHarness(t)
    h.login()
    h.app.route = Route{Page: "thread", Name: "Go", ID: "42", Number: 1}
    h.app.thread = forum.Thread{ID: "42", Board: "Go", Title: "主题", Articles: []forum.Article{
        {ID: "42", GroupID: "42", Board: "Go", Title: "主题", Author: forum.User{ID: "alice"}},
        {ID: "44", GroupID: "42", Board: "Go", Author: forum.User{ID: "bob"}},
    }}
    h.view.Frame()
    texts := h.view.Texts()
    if countText(texts, "进入版面") != 0 {
        t.Fatalf("duplicate board action remains: %q", texts)
    }
    if countText(texts, "版面：") != 1 {
        t.Fatalf("board breadcrumb count = %d, texts=%q", countText(texts, "版面："), texts)
    }
}
```

Add a local `countText([]string, string) int` test helper if the package does not already have one.

- [ ] **Step 4: Run focused tests and verify they fail**

```sh
GOWORK=off go -C apps/android/app test ./internal/ui -run 'TestThread(PageHasSingleBoardEntryAndNoDuplicateThemeToolbar|TopActionsCollectAndMore)' -count=1
go -C apps/desktop test ./internal/desktop -run TestDesktopThreadHasOneBoardEntryAndNoDuplicateRootActions -count=1
```

Expected: fail because Android still renders `threadToolbar` with `thread-open-board`, and PC still renders `进入版面` in the toolbar.

---

### Task 2: Move Android theme actions to the top bar and make the page scroll as one content stream

**Files:**
- Create: `apps/android/app/internal/ui/thread_actions.go`
- Modify: `apps/android/app/internal/ui/app.go:691-710,905-961,963-985,1253-1281`
- Modify: `apps/android/app/internal/ui/pages.go:921-1076`
- Modify: `apps/android/app/internal/ui/style.go` if a compact theme-only icon/control helper is needed

**Interfaces:**
- Produces `func (a *App) threadMoreMenu(gtx layout.Context) layout.Dimensions` and `func (a *App) threadTopActions(gtx layout.Context) layout.Dimensions`.
- Produces a thread page whose list rows are: compact header, optional primary context row, and posts; no fixed `threadHeader`/`threadToolbar` above the list.
- Consumes the route-owned `threadList` from the reading-position plan; if executing independently, use the existing `&a.threadList` first and replace it with the keyed state in the reading-position plan before final integration.

- [ ] **Step 1: Make the top bar generic and hide bottom navigation on PageThread**

Change `title()` so `PageThread` returns `"帖子"` rather than `a.thread.Title`. Keep `subtitle()` as `"主题阅读"` or use the canonical board description as the one-line subtitle.

Change the bottom-nav condition in `Layout` from:

```go
if page.Page == PageLogin || page.Page == PageCompose || page.Page == PageSection && page.Sub == "pick" {
    return layout.Dimensions{}
}
```

to:

```go
if page.Page == PageLogin || page.Page == PageCompose ||
    page.Page == PageThread || page.Page == PageSection && page.Sub == "pick" {
    return layout.Dimensions{}
}
```

In `topBar`, for `PageThread`, render a star icon keyed `thread-collect-top` and a more icon keyed `thread-more`; keep refresh only in the more menu. The star calls `collectThread()` and changes selected state from `a.thread.Collect`.

- [ ] **Step 2: Implement the Android more-menu state and actions**

Add `threadMoreOpen bool` to `App` and reset it whenever a non-thread route is entered or the thread changes. Create `thread_actions.go` with a single menu renderer containing:

```text
复制链接
刷新主题
投票（only if a confirmed current-thread vote is available）
竞猜（only if a confirmed current-thread bet is available）
编辑本帖（only when current user/admin can manage）
删除本帖（only when current user/admin can manage）
```

The menu items invoke existing `copyText`, `refreshCurrent`, `openVote`, `openBet`, `openCompose`, and delete flow. Every successful action closes `threadMoreOpen`. Do not include `进入版面`; the header Chip is the only board navigation.

If no existing popup primitive is available, render the menu as a `material.List`-style compact card directly below the AppBar in the outer `body` when `threadMoreOpen`; this keeps the icon in the upper-right and remains testable without introducing a new dependency.

- [ ] **Step 3: Replace fixed header/toolbar with a single scroll list**

Replace `threadPage`’s two outer `layout.Rigid` children with one list. Use this shape:

```go
return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
    layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
        rows := 1 + len(a.thread.Articles)
        return material.List(a.theme, a.threadList).Layout(gtx, rows, func(gtx layout.Context, i int) layout.Dimensions {
            if i == 0 {
                return a.compactThreadHeader(gtx)
            }
            return a.postCard(gtx, i-1, a.thread.Articles[i-1])
        })
    }),
    layout.Rigid(a.pager),
)
```

The first row must contain:

- one clickable `thread-board` Chip using `a.threadBoard()` (canonical API board for navigation);
- reply count and last-reply metadata when present;
- actual thread title at `unit.Sp(18)`/`MaxLines=2` with ellipsis;
- no second “进入版面” control and no full toolbar card.

Keep an empty/error row when `thread.Articles` is empty; do not render action controls when the thread load failed. Preserve `threadList` reset only in `openThread` for a genuinely new thread/page key, not in an item callback.

- [ ] **Step 4: Keep floor actions and remove only root duplicates according to the chosen policy**

Use the following policy consistently on Android and PC:

- Theme-level favorite and link copy live in the top-right star/more controls.
- The board Chip is the only board navigation control.
- The root floor keeps its normal content and reaction/reply row as the single visible root action row.
- Remove the separate fixed `threadToolbar` entirely; this avoids a second root like/reply and dramatically reduces first-screen height.
- Non-root floors keep like, downvote, reply, and copy-body actions.

In `postCard`, identify the root by `index == 0` and keep the existing floor row; do not remove non-root actions. Keep stable click keys (`like:<id>`, `down:<id>`, `reply:<id>`, `copy:<id>`) so existing interaction tests continue to target the same controls.

- [ ] **Step 5: Run Android UI tests and verify the new structure**

```sh
GOWORK=off go -C apps/android/app test ./internal/ui -run 'TestThread(PageHasSingleBoardEntryAndNoDuplicateThemeToolbar|TopActionsCollectAndMore|ReactionUpdatesServerCounts|CollectThreadTogglesServerState)' -count=1
GOWORK=off go -C apps/android/app vet ./...
```

Expected: no bottom navigation on a thread page, one scrollable header, no fixed duplicate toolbar, top-right collect/more controls work, and existing reaction/collection behavior remains green.

---

### Task 3: Make vote/bet visibility and action availability truthful

**Files:**
- Modify: `apps/android/app/internal/ui/thread_actions.go`
- Modify: `apps/android/app/internal/ui/pages.go` / `app.go` thread load state
- Modify: `apps/desktop/internal/desktop/thread.go` and thread load state
- Test: `apps/android/app/internal/ui/navigation_test.go`
- Test: `apps/desktop/internal/desktop/app_test.go`

**Interfaces:**
- Produces `threadHasVote` and `threadHasBet` booleans backed by confirmed current-thread resource metadata, not by the existence of a generic thread ID.
- If the API does not include resource metadata in the thread response, performs at most one read-only preflight per opened thread and caches the result for that route; a 404/API “not found” means hidden, not a visible button.

- [ ] **Step 1: Add failing visibility tests**

Use a local server where `/open/vote/42.json` and `/open/bet/42.json` return not-found for a normal thread. Render the more menu and assert:

```go
if a.click("thread-vote").Clicked(gtx) || a.click("thread-bet").Clicked(gtx) {
    t.Fatal("vote/bet controls appeared for a normal thread")
}
```

Use a second fixture with valid vote/bet JSON whose `aid`/`bid` belongs to `42`; assert both menu entries appear and route to `PageVote`/`PageBet` only after selection.

- [ ] **Step 2: Implement cached read-only availability**

On thread load, query the resource metadata only once for the current `{board, thread ID}`. Store the result as route-scoped `voteAvailable`/`betAvailable` state. Do not show a button while the preflight is pending; after a not-found/error response, keep it hidden and leave the thread usable. A valid response must match the current thread’s article ID before setting availability true.

Do not issue these preflights from every frame or every menu open.

- [ ] **Step 3: Fix existing vote/bet condition tests**

Add Android deleted-vote coverage:

```go
func TestDeletedVoteCannotSubmit(t *testing.T) {
    // load a Vote{IsDeleted:true}, select an option, click vote-submit
    // assert no POST /open/vote/42.json was recorded
}
```

The submit control must be visually disabled and must reject `IsDeleted`/`IsEnd`, not merely return early from the callback. Keep the existing endpoint contract tests.

- [ ] **Step 4: Run action availability checks**

```sh
GOWORK=off go -C apps/android/app test ./internal/ui -run 'Test(DeletedVoteCannotSubmit|Thread.*Vote|Thread.*Bet)' -count=1
go -C apps/desktop test ./internal/desktop -run 'TestDesktopThreadHasOneBoardEntryAndNoDuplicateRootActions' -count=1
```

Expected: normal threads have no vote/bet entries; valid resources are reachable; deleted/ended votes cannot be submitted.

---

### Task 4: Apply the same operation ownership to PC and canonicalize links

**Files:**
- Modify: `apps/desktop/internal/desktop/thread.go:54-146,217-244`
- Modify: `apps/desktop/internal/desktop/app.go:651-665`
- Test: `apps/desktop/internal/desktop/app_test.go`

**Interfaces:**
- Consumes `threadBoard()` and the existing `device.Copy`/`ThreadURL` path.
- Produces one desktop board breadcrumb, no duplicate `进入版面`, no duplicate root-level theme reply/like toolbar, and canonical `/article/IWhisper/<id>` links.

- [ ] **Step 1: Add failing PC canonical-link/action tests**

```go
func TestDesktopThreadUsesCanonicalBoardForLinkAndOneBoardEntry(t *testing.T) {
    h := newHarness(t)
    h.login()
    h.app.route = Route{Page: "thread", Name: "悄悄话", ID: "42", Number: 1}
    h.app.thread = forum.Thread{ID: "42", Board: "IWhisper", BoardDescription: "悄悄话", Articles: []forum.Article{
        {ID: "42", Board: "IWhisper", Author: forum.User{ID: "alice"}},
    }}
    h.view.Frame()
    h.click("版面：悄悄话 · IWhisper")
    if h.app.route.Page != "board" || h.app.route.Name != "IWhisper" {
        t.Fatalf("board route = %+v", h.app.route)
    }
    h.app.route = Route{Page: "thread", Name: "悄悄话", ID: "42", Number: 1}
    h.app.thread.Board = "IWhisper"
    h.view.Frame()
    h.click("复制链接")
    if !strings.Contains(h.device.copied, "/article/IWhisper/42") || strings.Contains(h.device.copied, "/article/悄悄话/42") {
        t.Fatalf("copied link = %q", h.device.copied)
    }
}
```

- [ ] **Step 2: Run the focused test and verify old duplicate behavior fails**

```sh
go -C apps/desktop test ./internal/desktop -run TestDesktopThreadUsesCanonicalBoardForLinkAndOneBoardEntry -count=1
```

Expected: current header/toolbar expose duplicate board actions and copy-link uses `a.route.Name`.

- [ ] **Step 3: Simplify desktop thread operations**

Use `a.threadBoard()` for header navigation and `Forum.ThreadURL`. Remove the toolbar’s `进入版面`; keep the header breadcrumb as the only board entry. Remove duplicate root theme actions from the separate toolbar, or remove the toolbar entirely if the root floor action row remains the single root control. Keep non-root floor actions and permission-gated edit/delete.

Put `复制链接`, `刷新`, and conditional vote/bet/manage actions in the desktop’s existing secondary action area/menu rather than adding another board button. Preserve `收藏文章` in the desktop top-level action area if the desktop shell has no Android-style AppBar.

- [ ] **Step 4: Run desktop tests and full checks**

```sh
go -C apps/desktop test ./internal/desktop -run 'Test(DesktopThreadUsesCanonicalBoardForLinkAndOneBoardEntry|ThreadReactionsCollectionAndQuoteMatchReference)' -count=1
go run ./tools/dev check --scope all
```

Expected: canonical links, one board entry, no duplicate root actions, and all existing PC behavior remain green.

---

### Task 5: Screenshot and physical interaction acceptance

**Files:**
- Modify: `apps/android/app/internal/ui/preview_test.go`
- Modify: `apps/android/app/internal/ui/interactions_test.go`
- Generated: `build/android-previews/thread*.png` (ignored)

- [ ] **Step 1: Add long-title/many-floor preview fixtures**

Generate deterministic previews for:

- `thread.png` — normal theme;
- `thread-long-title.png` — three-line title;
- `thread-many-posts.png` — ten floors;
- `thread-scrolled.png` — list position `First=4`;
- `thread-dark.png` — dark palette.

- [ ] **Step 2: Assert the first floor begins in the first third of the viewport**

At `420×820`, record the first floor’s layout bounds with the existing preview/test harness. The first floor header must begin before approximately `y=240px`, no horizontal overflow is allowed, and the page must render no `renderPanic`.

- [ ] **Step 3: Assert the action ownership**

Use physical click/layout tests to verify:

- `thread-collect-top` is clickable in the top bar;
- `thread-more` opens the menu;
- `thread-board` navigates to the canonical board;
- `thread-open-board` is absent;
- `thread-share` is only in the more menu;
- root and non-root floor actions retain stable click keys.

- [ ] **Step 4: Run final Android preview and all-module validation**

```sh
cd apps/android/app
BYR_ANDROID_PREVIEW_DIR=../../../build/android-previews GOWORK=off go test ./internal/ui -run TestPreviewScreens -count=1
cd ../../..
go run ./tools/dev check --scope all
git diff --check
```

Expected: all tests/vet pass; the new UI is compact, scrollable, and has no duplicate board/theme actions.
