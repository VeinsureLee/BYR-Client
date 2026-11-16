# Android–Desktop Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the six reported Android defects and make every desktop top-level workflow available and operable from the Gio mobile client with mobile-appropriate navigation and the same shared-core semantics.

**Architecture:** Keep protocol and business rules in `core/forum` and `core/application`; Android owns routes, lifecycle, Gio widgets, image presentation, and asynchronous UI updates. First repair the shared clickable-control primitive and session state machine, then build avatar, navigation, interaction, compose, and account/message pages on those foundations. Tests use local stores plus `httptest.Server`; no test writes to the real forum.

**Tech Stack:** Go 1.25.6 for Android/core, Gio v0.10.3, shared `core/application`, `core/forum`, `core/local`, `go test`, `go vet`, `gogio` ARM64 packaging.

## Global Constraints

- Preserve `apps/android/app` as an independent module with `GOWORK=off`; do not add Gio dependencies to `core/`.
- Keep Android package `cn.byr.bbs.byrclient`, minSdk 24, targetSdk 36, ARM64, and debug signing until release signing is configured.
- Use Material seed `#0A5CBF`, desktop-equivalent selected states, cards, pills, and one-way like/downvote semantics.
- Reactions cannot be cancelled; switching like ↔ downvote must use `application.App.React` and server-confirmed counts.
- Article collection must use `application.App.SetCollected`; board collection must use forum favorite endpoints.
- Search remains two explicit non-operational entries: `全站搜索 · 敬请期待` and `AI 搜索 · 敬请期待`; it must not issue search requests.
- Do not retry any server write automatically (publish, reaction, favorite, mail, vote, bet).
- Credentials remain in the app-private `session.json` (0600); ordinary local state must never contain a token.
- A failed network page must show a retryable error state, never an empty/white page and never a false “暂无内容”.
- Tests must use local HTTP fixtures only; never like, collect, post, mail, vote, or bet against the live forum.

---

### Task 1: Baseline checkpoint and physical-tap regression harness

**Files:**
- Modify: `apps/android/app/internal/ui/click_test.go`
- Modify: `apps/android/app/internal/ui/style.go:57-130`

**Interfaces:**
- Consumes: `App.click(key string) *widget.Clickable`, Gio `input.Router` and `pointer.Event`.
- Produces: correctly registered `pill(...) (layout.Dimensions, bool)` and `filledPill(...) (layout.Dimensions, bool)` hit regions.

- [ ] **Step 1: Verify the requested checkpoint**

Run:

```sh
git status --short
git log -1 --oneline
```

Expected: no uncommitted source changes; the prior Android UI work is already represented by commit `76f8adf`. Do not create an empty commit.

- [ ] **Step 2: Add a real pointer-event test that fails when a pill is only painted**

Add a helper that lays out one control, frames its operations, sends press/release events inside the returned dimensions, lays out again, and checks the callback state:

```go
func tap(t *testing.T, router *input.Router, frame func(layout.Context) layout.Dimensions, at f32.Point) {
    t.Helper()
    var ops op.Ops
    gtx := layout.Context{Ops: &ops, Source: router.Source(), Constraints: layout.Exact(image.Pt(420, 120))}
    frame(gtx)
    router.Frame(&ops)
    router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: at})
    ops.Reset()
    frame(gtx)
    router.Frame(&ops)
    router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 1, Position: at})
    ops.Reset()
    frame(gtx)
}
```

Create `TestPillRegistersPhysicalTap` and `TestFilledPillRegistersPhysicalTap` and assert each callback fires exactly once.

- [ ] **Step 3: Run the focused tests and verify failure**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui -run 'Test(Pill|FilledPill)RegistersPhysicalTap' -count=1
```

Expected: FAIL because current `pill` and `filledPill` never call `btn.Layout` and therefore register no pointer target.

- [ ] **Step 4: Wrap the recorded visual in the clickable layout**

Implement both primitives with this shape (retaining their existing visual drawing):

```go
btn := a.click(key)
clicked := btn.Clicked(gtx)
dims := btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
    // record content, paint background, replay content
    return layout.Dimensions{Size: size}
})
return dims, clicked
```

The clickable area must exactly equal the painted pill dimensions.

- [ ] **Step 5: Run Android tests and vet**

Run:

```sh
go run ./tools/dev check --scope android
```

Expected: all tests and vet pass.

- [ ] **Step 6: Commit the interaction foundation**

```sh
git add apps/android/app/internal/ui/style.go apps/android/app/internal/ui/click_test.go
git commit -m "fix(android): register pill controls for touch

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 2: Deterministic logout and process-restart recovery

**Files:**
- Modify: `apps/android/app/internal/ui/app.go:58-105,189-328,337-385`
- Modify: `apps/android/app/internal/ui/pages.go:1288-1312`
- Create: `apps/android/app/internal/ui/session_test.go`
- Test: `apps/android/app/internal/platform/storage_test.go`

**Interfaces:**
- Produces: `func (a *App) resetSessionUI()` and a route-safe `func (a *App) current() Route` that never indexes an empty stack.
- Consumes: `application.App.Logout`, `platform.FileCredentials.Clear`, `platform.FileCredentials.Load`.

- [ ] **Step 1: Write restart-oriented failing tests**

Add tests for these exact transitions:

```go
func TestLogoutThenNewAppStartsAtLogin(t *testing.T) { /* shared temp BYR_DATA_DIR, persisted session, logout, construct second App, start, drain, assert PageLogin */ }
func TestCurrentRouteRepairsEmptyStack(t *testing.T) { /* set stack=nil; call current; assert PageLogin and len(stack)==1 */ }
func TestExpiredOrMissingCredentialsStartAtLogin(t *testing.T) { /* empty/expired session file; assert ready login rather than init error */ }
```

Also test `FileCredentials.Clear` twice and then `Load`, expecting an empty credential and no error.

- [ ] **Step 2: Run focused tests and verify at least one failure**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui ./internal/platform -run 'Test(LogoutThenNewApp|CurrentRoute|ExpiredOrMissingCredentials|FileCredentials)' -count=1
```

- [ ] **Step 3: Centralize all post-logout state clearing**

Implement:

```go
func (a *App) resetSessionUI() {
    a.stack = []Route{{Page: PageLogin}}
    a.sections = nil
    a.section = forum.Section{}
    a.boards = nil
    a.board = forum.Board{}
    a.thread = forum.Thread{}
    a.articles = nil
    a.favorite = forum.Favorite{}
    a.collect = forum.Collection{}
    a.pagination = forum.Pagination{}
    a.user = forum.User{}
    a.history = nil
    a.compose = composeState{}
    a.images = map[string]*mobileImage{}
    a.clicks = map[string]*widget.Clickable{}
    a.username.SetText("")
    a.password.SetText("")
    a.message, a.initErr = "", ""
    a.failed, a.busy = false, false
}
```

Call it only after `core.Logout` succeeds; on failure retain the authenticated route and display the error.

- [ ] **Step 4: Make route access total and startup retryable**

Change `current()` so an empty stack is repaired before returning. On startup with no credential, explicitly call `resetSessionUI()` after assigning `a.core`/`a.dir`, while preserving `ready=true`. Clear stale `initErr` on a successful retry.

- [ ] **Step 5: Run tests and full Android check**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui ./internal/platform -count=1
cd ../../.. && go run ./tools/dev check --scope android
```

Expected: logout, re-entry, missing credentials, and repeated clear all pass with no panic or blank dimensions.

- [ ] **Step 6: Commit lifecycle repair**

```sh
git add apps/android/app/internal/ui/app.go apps/android/app/internal/ui/pages.go apps/android/app/internal/ui/session_test.go apps/android/app/internal/platform/storage_test.go
git commit -m "fix(android): recover cleanly after logout and restart

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 3: Real forum avatars and user navigation

**Files:**
- Modify: `apps/android/app/internal/ui/images.go`
- Modify: `apps/android/app/internal/ui/style.go:147-172,211-279`
- Modify: `apps/android/app/internal/ui/pages.go` (post, collection, self avatar call sites)
- Create: `apps/android/app/internal/ui/avatar_test.go`

**Interfaces:**
- Produces: `func (a *App) avatar(gtx layout.Context, user forum.User, size unit.Dp) layout.Dimensions`.
- Reuses: `imageEntry(raw string) *mobileImage`, `forum.User.FaceURL`, `forum.Client.FetchAsset`.

- [ ] **Step 1: Add avatar pipeline tests**

Use `httptest.Server` to serve a 2×2 PNG from `/face/alice.png`; create a user with `FaceURL` set to that URL. Assert `imageEntry` reaches a decoded image, and assert an empty/failed URL renders the initial fallback without panicking.

- [ ] **Step 2: Run the avatar tests and verify failure**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui -run 'TestAvatar' -count=1
```

Expected: FAIL because Android currently accepts only a display-name string and always draws an initial.

- [ ] **Step 3: Implement clipped image avatars with fallback**

Implement `avatar` so it:

1. Fixes constraints to a square.
2. If `FaceURL` is empty/loading/failed, draws `avatarCircle` using `DisplayName()`.
3. When decoded, applies `clip.Ellipse`, lays out `widget.Image{Src: paint.NewImageOp(e.img), Fit: widget.Cover}`, and restores the clip.
4. Uses the same cache and authenticated `FetchAsset` restrictions as inline post images.

- [ ] **Step 4: Pass complete `forum.User` values at every call site**

Replace calls such as:

```go
a.avatarCircle(gtx, v.Author.DisplayName(), 34, colorAccentSoft, colorAccent)
```

with:

```go
a.avatar(gtx, v.Author, 34)
```

Cover board/timeline article tiles, thread posts, collection rows, and the self profile card.

- [ ] **Step 5: Verify avatar tests and preview rendering**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui -run 'TestAvatar|TestPreviewScreens' -count=1
```

Expected: decoded images render; fallback initials remain for anonymous/no-face fixtures.

- [ ] **Step 6: Commit avatar support**

```sh
git add apps/android/app/internal/ui/images.go apps/android/app/internal/ui/style.go apps/android/app/internal/ui/pages.go apps/android/app/internal/ui/avatar_test.go
git commit -m "feat(android): display forum user avatars

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 4: Mobile navigation aligned with desktop top-level pages

**Files:**
- Modify: `apps/android/app/internal/ui/app.go`
- Create: `apps/android/app/internal/ui/top.go`
- Modify: `apps/android/app/internal/ui/pages.go` (search only)
- Modify: `apps/android/app/internal/ui/preview_test.go`
- Create: `apps/android/app/internal/ui/navigation_test.go`

**Interfaces:**
- Adds routes: `PageTop`, `PageCompose`, `PageSearch`, `PageMessages`, while retaining `PageHome`, `PageTimeline`, `PageFavorites`, `PageSelf`.
- Produces: `loadTop(sub string)`, `topPage(layout.Context)`, and a visible top-level search destination.

- [ ] **Step 1: Add navigation parity tests**

Assert the mobile navigation model contains the same labels as desktop:

```go
want := []string{"首页", "十大", "时间线", "收藏", "发帖", "搜索", "消息", "我"}
```

For each item, activate its `widget.Clickable`, lay out `bottomNav`, and assert `current().Page` changes to the expected route. Add an assertion that `PageSearch` renders both `全站搜索` and `AI 搜索` while the fixture server receives zero search requests.

- [ ] **Step 2: Run focused tests and verify failure**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui -run 'TestNavigation|TestSearch' -count=1
```

Expected: FAIL because the current bar exposes only 首页/时间线/收藏/我 and has no Top or Messages route.

- [ ] **Step 3: Define one source of truth for top-level destinations**

Add a package-level route descriptor slice used by both rendering and tests. On narrow phones, keep five primary destinations in the bottom bar and expose all eight in a mobile navigation sheet/page reached through “更多”; Search and Compose must remain one-tap entries from the primary shell. Avoid squeezing eight labels into 45dp cells.

- [ ] **Step 4: Implement Top Ten and recommendations**

`loadTop("")` calls `Forum.TopTen`; `loadTop("recommend")` calls `Forum.Recommendations`. Render with the shared `articleTile`, and provide pills for `今日十大` and `版面推荐`.

- [ ] **Step 5: Wire search as a stable top-level route**

`setTab(PageSearch)` must reset the stack to `[]Route{{Page: PageSearch}}`. Keep both cards explicitly disabled/coming-soon and do not invoke forum search APIs.

- [ ] **Step 6: Run tests and regenerate previews**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui -count=1
BYR_ANDROID_PREVIEW_DIR=../../../build/android-previews GOWORK=off go test ./internal/ui -run TestPreviewScreens -count=1
```

- [ ] **Step 7: Commit navigation parity**

```sh
git add apps/android/app/internal/ui/app.go apps/android/app/internal/ui/top.go apps/android/app/internal/ui/pages.go apps/android/app/internal/ui/navigation_test.go apps/android/app/internal/ui/preview_test.go
git commit -m "feat(android): align top-level navigation with desktop

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 5: Reactions, replies, collections, and full-card tap targets

**Files:**
- Modify: `apps/android/app/internal/ui/pages.go:625-936,994-1112`
- Modify: `apps/android/app/internal/ui/style.go:211-279`
- Create: `apps/android/app/internal/ui/interactions_test.go`

**Interfaces:**
- Consumes: `application.App.React`, `application.App.SetCollected`, `App.openCompose`, `App.openThread`.
- Produces: working physical taps for like, downvote, reply, collect, collection tab, collection article, and uncollect.

- [ ] **Step 1: Write endpoint-backed interaction tests**

Create a local fixture server that records paths/forms and returns server-shaped JSON. Test:

- tapping `thread-like` sends one like request and updates `LikeSum`/`IsLiked`;
- tapping a post downvote sends one downvote request and updates `VotedownSum`/`IsVotedown`;
- tapping reply pushes `PageCompose` with the selected article ID;
- tapping `thread-collect` toggles `thread.Collect` only after success;
- tapping `fav-tab:collection` changes the tab and loads `/collection.json`;
- tapping anywhere on a collection article card opens that thread;
- tapping remove issues one uncollect request and reloads the collection.

- [ ] **Step 2: Run tests and verify failures**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui -run 'Test(ThreadReaction|PostReaction|Reply|Collection)' -count=1
```

- [ ] **Step 3: Make collection cards fully clickable without nesting targets**

Use an outer `btn.Layout` for the article body/card and a sibling remove action; do not nest one `Clickable` inside another. Preserve `RememberCollection` before `openThread`.

- [ ] **Step 4: Keep reaction state server-authoritative**

Update counts and selected flags only from the `forum.Article` returned by `application.React`. On error, leave the previous article untouched and show the server message. Prevent a second request when the same one-way reaction is already selected.

- [ ] **Step 5: Run focused and full tests**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui -run 'Test(ThreadReaction|PostReaction|Reply|Collection)' -count=1
cd ../../.. && go run ./tools/dev check --scope android
```

- [ ] **Step 6: Commit forum interactions**

```sh
git add apps/android/app/internal/ui/pages.go apps/android/app/internal/ui/style.go apps/android/app/internal/ui/interactions_test.go
git commit -m "fix(android): connect thread and collection actions

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 6: Non-blank compose flow and desktop-equivalent post semantics

**Files:**
- Modify: `apps/android/app/internal/ui/app.go` (`composeState`)
- Modify: `apps/android/app/internal/ui/compose.go`
- Create: `apps/android/app/internal/ui/compose_test.go`

**Interfaces:**
- Adds `composeState.boardEditor widget.Editor` and an immediately renderable compose state.
- Consumes: `application.App.Publish`, `local.DraftKey`, `local.Store.SaveDraft`.

- [ ] **Step 1: Add failing tests for every compose entry path**

Test direct top-level new post (no preselected board), board new post, thread reply, post reply, and edit. Immediately after navigation, assert the page has non-zero dimensions and visible fields even before network work completes. Add publish success/failure tests and assert a failed write keeps the draft and editor content.

- [ ] **Step 2: Run compose tests and verify failure**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui -run 'TestCompose' -count=1
```

Expected: direct compose cannot open because `openCompose` returns for an empty board, and a skipped/failed `run("准备编辑器", ...)` leaves `ready=false` indefinitely.

- [ ] **Step 3: Make editor construction synchronous and data hydration asynchronous**

Initialize editors and set `ready=true` before starting optional edit/draft loading. Do not block the page behind a `Forum.Board` read. For a top-level new post, show a single-line board-name editor; for board/thread routes, prefill and lock the board.

- [ ] **Step 4: Preserve desktop publishing rules**

Build:

```go
p := forum.Post{
    Board: strings.TrimSpace(c.boardEditor.Text()),
    Title: strings.TrimSpace(c.titleEditor.Text()),
    Content: c.contentEditor.Text(),
    ReplyID: c.replyID,
}
result, err := a.core.Publish(ctx, p, c.editID)
```

Keep the draft until server success. If `Publication.LocalError` is non-nil, state “帖子已发布，请勿重复提交” and navigate to the returned thread without submitting again.

- [ ] **Step 5: Add mobile controls for desktop post flags**

Add `widget.Bool` fields for anonymous, signature, mail reply notification, and outgo; pass them through `forum.Post`. Render each with `material.CheckBox`, respecting board capabilities when known.

- [ ] **Step 6: Run tests, vet, and preview**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui -run 'TestCompose' -count=1
cd ../../.. && go run ./tools/dev check --scope android
```

- [ ] **Step 7: Commit compose repair**

```sh
git add apps/android/app/internal/ui/app.go apps/android/app/internal/ui/compose.go apps/android/app/internal/ui/compose_test.go
git commit -m "fix(android): make post composer always available

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 7: Remaining desktop account and communication routes

**Files:**
- Create: `apps/android/app/internal/ui/account.go`
- Create: `apps/android/app/internal/ui/mail.go`
- Modify: `apps/android/app/internal/ui/app.go`
- Modify: `apps/android/app/internal/ui/pages.go` (remove migrated self code)
- Create: `apps/android/app/internal/ui/parity_test.go`

**Interfaces:**
- Adds routes for `PageUser`, `PageHistory`, `PageBlacklist`, `PageSettings`, `PageMessages`, `PageChat`, `PageMailbox`, `PageMail`, `PageMailCompose`, `PageRefers`, `PageVote`, and `PageBet`.
- Reuses desktop-equivalent core methods: `Forum.User`, `Forum.Blacklist`, `App.Contacts`, `App.Conversation`, `Forum.Mailbox`, `Forum.Mail`, `Forum.Refers`, `Forum.Vote`, and `Forum.Bet`.

- [ ] **Step 1: Add a route/load parity table test**

Assert every desktop page in this set has a non-empty Android renderer and loader mapping:

```go
[]Page{
    PageUser, PageHistory, PageBlacklist, PageSettings,
    PageMessages, PageChat, PageMailbox, PageMail, PageMailCompose, PageRefers,
    PageVote, PageBet,
}
```

Use local endpoint fixtures and assert empty, loading, success, and error states all return non-zero dimensions.

- [ ] **Step 2: Implement account routes**

Port desktop functionality into card/list mobile layouts: full user profile and avatar, send-message entry, reading history (all rows plus clear), blacklist list/remove, theme/font/IPv6 preferences, logout, vote read/submit, and bet read/submit. Mutations require explicit buttons and no automatic retry.

- [ ] **Step 3: Implement communication routes**

Port contacts, conversation, inbox/outbox, mail details, send/reply/forward, delete, and reply/@ reminders. Preserve server content direction and read-state semantics from `core/application`/`core/forum` rather than recreating protocol logic in UI code.

- [ ] **Step 4: Link user avatars and thread content routes**

Tapping a non-anonymous avatar opens `PageUser`. Mail/message actions open the corresponding compose/chat routes. Vote and bet links recognized by existing content metadata open `PageVote`/`PageBet`.

- [ ] **Step 5: Run parity tests and full checks**

Run:

```sh
cd apps/android/app && GOWORK=off go test ./internal/ui -run 'TestParity|TestAccount|TestMail|TestVote|TestBet' -count=1
cd ../../.. && go run ./tools/dev check
```

- [ ] **Step 6: Commit route parity**

```sh
git add apps/android/app/internal/ui/app.go apps/android/app/internal/ui/pages.go apps/android/app/internal/ui/account.go apps/android/app/internal/ui/mail.go apps/android/app/internal/ui/parity_test.go
git commit -m "feat(android): add desktop account and message workflows

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 8: Version, documentation, complete verification, and APK

**Files:**
- Modify: `apps/android/app/main.go`
- Modify: `tools/internal/buildflow/android_production.go`
- Modify: `tools/internal/buildflow/android_production_test.go`
- Modify: `apps/android/README.md`
- Modify: `docs/features.md`
- Modify: `docs/build.md`
- Modify: `docs/android-evaluation.md`

**Interfaces:**
- Produces one stable artifact: `dist/android/BYR-Client-android-arm64.apk` and matching `manifest.json`.

- [ ] **Step 1: Update version metadata consistently**

Bump app display version, gogio semantic version/versionCode, build manifest, and test expectations together. The display version and manifest version must match exactly; the versionCode must increase from 11.

- [ ] **Step 2: Update the feature matrix truthfully**

Document all newly connected Android workflows, real avatars, search location, logout/restart behavior, and interaction tests. Keep unsupported OS integrations (for example release signing or file picker if still absent) explicitly listed rather than claiming parity for them.

- [ ] **Step 3: Run formatting and all module checks**

Run:

```sh
gofmt -w apps/android/app/internal/ui/*.go apps/android/app/internal/platform/*.go
go run ./tools/dev check
```

Expected: core, tools, desktop, and Android tests plus vet all pass.

- [ ] **Step 4: Regenerate all Android previews**

Run:

```sh
cd apps/android/app
BYR_ANDROID_PREVIEW_DIR=../../../build/android-previews GOWORK=off go test ./internal/ui -run TestPreviewScreens -count=1
```

Inspect login, home, board, thread, timeline, favorites, compose, search, self, and new parity pages. No preview may be blank, clipped, or horizontally scrollable at 420×820dp.

- [ ] **Step 5: Build and verify the production APK**

Run:

```sh
cd ../../..
go run ./tools/dev build android
go run ./tools/dev check --scope android
```

Verify the APK is a readable ZIP, contains `AndroidManifest.xml` and `lib/arm64-v8a/libgio.so`, and that `dist/android/manifest.json` SHA-256 matches the artifact.

- [ ] **Step 6: Commit the release candidate**

```sh
git add apps/android/app/main.go tools/internal/buildflow/android_production.go tools/internal/buildflow/android_production_test.go apps/android/README.md docs/features.md docs/build.md docs/android-evaluation.md
git commit -m "release(android): align mobile workflows with desktop

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

- [ ] **Step 7: Verify final repository state**

Run:

```sh
git status --short
git log --oneline -8
```

Expected: clean working tree with the implementation commits above and one stable Android distribution directory.

---

## Self-Review

- **Spec coverage:** Task 2 covers logout/re-entry white screen; Task 3 covers personal and post avatars; Tasks 1 and 5 cover like/downvote/reply and collection; Task 4 exposes Search; Task 6 fixes blank compose; Tasks 4 and 7 cover desktop top-level and secondary workflow parity.
- **Placeholder scan:** Every task names exact files, interfaces, tests, commands, and expected behavior; no TODO/TBD implementation placeholders remain.
- **Type consistency:** All page state stays in `ui.App`; forum writes use existing `application.App` methods; `avatar` accepts `forum.User`; click primitives retain their existing `(layout.Dimensions, bool)` signatures; compose continues through `application.App.Publish`.
