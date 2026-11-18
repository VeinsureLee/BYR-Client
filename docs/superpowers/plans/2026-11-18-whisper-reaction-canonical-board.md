# Whisper Reaction Canonical Board Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure likes and downvotes on the “悄悄话” board always use the canonical API board name `IWhisper`, without changing or resolving the anonymous author ID.

**Architecture:** Keep reaction semantics in `core/application.App.React`; the UI layer must supply a canonical board name obtained from the loaded thread response rather than a localized route label. Desktop receives the missing canonical-board resolver, while Android extends its existing resolver to support responses where only the root article carries `board_name`.

**Tech Stack:** Go 1.27.4, MyGo v0.9.1 desktop UI, Gio v0.10.3 Android UI, shared `core/application` and `core/forum`, `httptest`, `go test`, `go vet`.

## Global Constraints

- Preserve every existing unstaged and untracked change on branch `feat/cross-platform-search`; do not stage or commit unless the user separately asks, because several target files already contain unrelated work.
- Do not modify `forum.User`, anonymous-user parsing, or the `IWhisper#<number>` pseudo-ID; reactions do not send an author ID.
- Reactions remain one-way: no cancellation request and no automatic retry of a server write.
- Tests must use local HTTP fixtures only and must never like or downvote a live forum post.
- Prefer canonical board names in this order: `Thread.Board`, root `Thread.Articles[0].Board`, then the current route name as compatibility fallback.
- Keep `apps/android/app` outside the root workspace and run its commands with `GOWORK=off`.

## File Structure

- `apps/desktop/internal/desktop/thread.go`: own desktop thread-level canonical-board selection and use it for both root and floor reactions.
- `apps/desktop/internal/desktop/app_test.go`: exercise the real desktop reaction path with route label `悄悄话`, canonical board `IWhisper`, and anonymous author `IWhisper#93`.
- `apps/android/app/internal/ui/pages.go`: extend the existing `threadBoard()` fallback chain.
- `apps/android/app/internal/ui/interactions_test.go`: cover both canonical-board response shapes on Android.

---

### Task 1: Route desktop reactions through the canonical board

**Files:**
- Modify: `apps/desktop/internal/desktop/thread.go:148-159,297-343`
- Test: `apps/desktop/internal/desktop/app_test.go:17-21,423-451`

**Interfaces:**
- Consumes: `forum.Thread.Board`, `forum.Thread.Articles`, `Route.Name`, and `application.App.React(ctx, board, article, kind)`.
- Produces: `func (a *App) threadBoard() string`, returning the first non-empty value from thread board, root-article board, and route name.

- [ ] **Step 1: Add a desktop regression test using the real whisper data shape**

Add the `core/forum` import to `app_test.go`, then add this test next to `TestThreadReactionsCollectionAndQuoteMatchReference`:

```go
func TestWhisperReactionUsesCanonicalBoard(t *testing.T) {
    h := newHarness(t)
    h.login()

    h.mu.Lock()
    h.overrides = map[string]string{
        "/open/article/IWhisper/like/42.json": `{"like_sum":3,"votedown_sum":0}`,
    }
    h.mu.Unlock()

    anonymous := forum.User{ID: "IWhisper#93"}
    root := forum.Article{
        ID:      "42",
        Board:   "IWhisper",
        Author:  anonymous,
        LikeSum: 2,
    }
    h.app.route = Route{Page: "thread", Name: "悄悄话", ID: "42", Number: 1}
    h.app.thread = forum.Thread{
        ID:       "42",
        Board:    "IWhisper",
        LikeSum:  2,
        Articles: []forum.Article{root},
    }
    h.view.Frame()

    h.click("赞 2")

    if n := h.count("GET /open/article/IWhisper/like/42.json"); n != 1 {
        t.Fatalf("canonical whisper like requests = %d, want 1", n)
    }
    if n := h.count("GET /open/article/悄悄话/like/42.json"); n != 0 {
        t.Fatalf("localized board reached reaction endpoint %d times", n)
    }
    got := h.app.thread.Articles[0]
    if !bool(got.IsLiked) || got.LikeSum != 3 {
        t.Fatalf("reaction state = %+v", got)
    }
    if got.Author.ID != "IWhisper#93" || got.Author.DisplayName() != "IWhisper#93" {
        t.Fatalf("anonymous author changed during reaction: %+v", got.Author)
    }
}
```

- [ ] **Step 2: Run the focused desktop test and verify the regression**

Run:

```sh
go -C apps/desktop test ./internal/desktop -run TestWhisperReactionUsesCanonicalBoard -count=1
```

Expected: FAIL because the request is recorded at `/open/article/悄悄话/like/42.json`, while the canonical `IWhisper` path is never called.

- [ ] **Step 3: Add one canonical-board resolver and use it from both reaction entry points**

Add this helper next to `threadRoot()` in `thread.go`:

```go
func (a *App) threadBoard() string {
    if board := strings.TrimSpace(a.thread.Board); board != "" {
        return board
    }
    if len(a.thread.Articles) > 0 {
        if board := strings.TrimSpace(a.thread.Articles[0].Board); board != "" {
            return board
        }
    }
    return a.route.Name
}
```

In both `reactThread` and `react`, replace:

```go
board := a.route.Name
```

with:

```go
board := a.threadBoard()
```

Do not change `application.App.React` and do not inspect `article.Author.ID` when choosing the endpoint.

- [ ] **Step 4: Run the desktop reaction tests and vet**

Run:

```sh
go -C apps/desktop test ./internal/desktop -run 'Test(WhisperReactionUsesCanonicalBoard|ThreadReactionsCollectionAndQuoteMatchReference)' -count=1
go -C apps/desktop vet ./...
```

Expected: both tests pass; the whisper test records exactly one canonical reaction request and preserves the anonymous author.

- [ ] **Step 5: Record the desktop checkpoint without staging unrelated work**

Run:

```sh
git diff --check -- apps/desktop/internal/desktop/thread.go apps/desktop/internal/desktop/app_test.go
git diff -- apps/desktop/internal/desktop/thread.go apps/desktop/internal/desktop/app_test.go
```

Expected: only the canonical-board helper, its two callers, and the new regression test belong to this task. Do not run `git add` or `git commit` unless the user separately requests a commit; `app_test.go` already contains pre-existing uncommitted work. If a commit is later requested, stage only these hunks and use `fix(desktop): use canonical board for reactions` with the required Claude Code co-author line.

---

### Task 2: Cover Android responses whose canonical board exists only on the root article

**Files:**
- Modify: `apps/android/app/internal/ui/pages.go:1259-1264`
- Test: `apps/android/app/internal/ui/interactions_test.go:207-220`

**Interfaces:**
- Consumes: the desktop-compatible fallback order established by Task 1.
- Produces: Android `threadBoard()` with the same `Thread.Board → root Article.Board → Route.Name` contract.

- [ ] **Step 1: Replace the narrow Android test with a two-shape table test**

Replace `TestThreadLikeUsesCanonicalBoardName` with:

```go
func TestThreadLikeUsesCanonicalBoardName(t *testing.T) {
    tests := []struct {
        name         string
        threadBoard  string
        articleBoard string
    }{
        {name: "thread field", threadBoard: "IWhisper", articleBoard: "IWhisper"},
        {name: "root article fallback", articleBoard: "IWhisper"},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            s := newInteractionServer(t)
            a := interactionApp(t, s.server.URL)
            defer a.Close()

            thread := sampleThread()
            thread.Board = tc.threadBoard
            thread.Articles[0].Board = tc.articleBoard
            thread.Articles[0].Author = forum.User{ID: "IWhisper#93"}
            a.thread = thread
            a.stack = []Route{{Page: PageHome}, {Page: PageThread, Name: "悄悄话", ID: "42", Number: 1}}

            a.likeThread()
            waitForIdle(t, a)

            if len(s.find("GET", "/open/article/IWhisper/like/42.json")) != 1 {
                t.Fatalf("like did not use canonical board: %+v", s.requests)
            }
            if len(s.find("GET", "/open/article/悄悄话/like/42.json")) != 0 {
                t.Fatalf("localized board reached endpoint: %+v", s.requests)
            }
            got := a.thread.Articles[0]
            if got.LikeSum != 3 || !bool(got.IsLiked) {
                t.Fatalf("thread reaction state = %+v", got)
            }
            if got.Author.ID != "IWhisper#93" {
                t.Fatalf("anonymous author changed: %+v", got.Author)
            }
        })
    }
}
```

- [ ] **Step 2: Run the focused Android test and verify the root-article case fails**

Run:

```sh
GOWORK=off go -C apps/android/app test ./internal/ui -run TestThreadLikeUsesCanonicalBoardName -count=1
```

Expected: the `thread field` subtest passes; `root article fallback` fails because current `threadBoard()` falls back directly to `Route.Name == "悄悄话"`.

- [ ] **Step 3: Complete the Android fallback chain**

Change `threadBoard()` in `pages.go` to:

```go
func (a *App) threadBoard() string {
    if board := strings.TrimSpace(a.thread.Board); board != "" {
        return board
    }
    if len(a.thread.Articles) > 0 {
        if board := strings.TrimSpace(a.thread.Articles[0].Board); board != "" {
            return board
        }
    }
    return a.current().Name
}
```

Keep `likeThread`, `react`, reply, and collection callers on this single helper.

- [ ] **Step 4: Run Android and full repository checks**

Run:

```sh
GOWORK=off go -C apps/android/app test ./internal/ui -run 'Test(ThreadLikeUsesCanonicalBoardName|ThreadReactionUpdatesServerCounts|PostDownvoteUpdatesServerCounts)' -count=1
go run ./tools/dev check --scope all
```

Expected: all reaction regressions, all module tests, and all vet checks pass.

- [ ] **Step 5: Record the Android checkpoint without staging unrelated work**

Run:

```sh
git diff --check -- apps/android/app/internal/ui/pages.go apps/android/app/internal/ui/interactions_test.go
git diff -- apps/android/app/internal/ui/pages.go apps/android/app/internal/ui/interactions_test.go
```

Expected: the task adds only the root-article board fallback and its two-shape regression coverage. Do not stage or commit by default because both files already contain pre-existing uncommitted changes. If a commit is later requested, stage only these hunks and use `fix(android): resolve whisper reaction board` with the required Claude Code co-author line.
