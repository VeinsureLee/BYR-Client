# Desktop Thread Refresh Feedback Layout Implementation Plan

> **For agentic workers:** This plan is being executed inline because the request already authorizes implementation and verification. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep desktop thread refresh feedback in a stable reserved area so it cannot cover floor actions or pagination.

**Architecture:** Replace the thread page's `Bottom(56)` overlay in `apps/desktop/internal/desktop/shell.go` with a fixed-height feedback slot allocated after the thread page. Keep the existing thread list, floor action controls, pager, refresh state, retry callback, and dismiss behavior; make the status content fit the slot without changing its height. Add offline desktop interaction tests with isolated fixtures at both requested window sizes; then run the specified cross-platform refresh, alias, race, screenshot, and offline checks.

**Tech Stack:** Go, MyGo UI, `httptest` offline fixtures, desktop and Android Go test modules.

## Global Constraints

- Validate desktop window sizes **720×480** and **1100×760** with independent fixtures for each scenario.
- The feedback slot is allocated before refresh and keeps the same height while waiting, failed, and dismissed.
- Refresh failure retains the last successful thread data and current page/reading anchor; existing account-generation, route, and request-sequence checks remain unchanged.
- The error's **「重试」** and **「关闭提示」** controls stay clickable; the last-floor like action and pager stay reachable while the error is visible.
- Do not commit changes unless explicitly requested.

---

### Task 1: Reproduce and fix the desktop feedback overlap

**Files:**
- Modify: `apps/desktop/internal/desktop/shell.go`
- Reference: `apps/desktop/internal/desktop/thread.go:325-395`, `apps/desktop/internal/desktop/thread.go:587-613`
- Reference: `build/ui-review-round8/probes/desktop_test.go`

**Interfaces:**
- Consumes: `App.hasLoadedThreadContent()`, `App.threadLoadingStrip(*ui.Context)`, `App.statusView(*ui.Context)`.
- Produces: `App.threadFeedbackSlot(*ui.Context)`, `App.threadLoadingSlot(*ui.Context)`, and `App.threadStatusView(*ui.Context)` with a fixed 72 DIP thread-only slot.

- [x] Confirmed the current overlap with the offline long-thread fixture at both sizes. Before the fix, clicks on `「赞 3141」` returned “not the input target” and sent zero like requests; dismissing the error restored the action.
- [x] Replaced thread refresh feedback overlays with the fixed-height slot after `a.page(c)` whenever `a.hasLoadedThreadContent()` is true. Kept the existing absolute feedback behavior for non-thread pages.
- [x] Rendered loading/status feedback in the reserved slot, left the same 72 DIP allocation when dismissed, and constrained thread feedback message/diagnostic text to one line while retaining retry and dismiss controls.
- [x] Ran `gofmt` and focused desktop tests; the last-floor action, paging, and anchor stability regressions pass at both requested sizes.

### Task 2: Add desktop interaction regressions at both sizes

**Files:**
- Create: `apps/desktop/internal/desktop/thread_feedback_test.go`
- Modify: `apps/desktop/internal/desktop/thread_lifecycle_test.go`
- Modify: `apps/desktop/internal/desktop/app_test.go`

**Interfaces:**
- Consumes: the existing fixture server, `desktopLongThread(30)`, test harness click/scroll/find/idle helpers, and `desktopListState`.
- Produces: `TestDesktopThreadRefreshErrorDoesNotCoverLastFloorLike`; strengthens `TestDesktopCanPageWhileRefreshFailureIsShown` to parse `page=2`; strengthens `TestDesktopPendingRefreshKeepsFloorAndInRowOffset` to hold the scroll anchor static and assert the same first row plus a 17 DIP offset across all four states. Updates the reminder target assertion to account for the existing header row at list index 0.

- [x] For each size `{720,480}` and `{1100,760}`, created a fresh 30-floor fixture with floor 30 ID `last-floor` and `LikeSum` 3141, failed refresh, kept the error visible, scrolled to the end, clicked `「赞 3141」`, and asserted exactly one additional `GET /open/article/Go/like/last-floor.json` request.
- [x] For each size, used a fresh fixture with distinct page-one/page-two content, failed page-one refresh, kept the error visible, clicked `「下一页」`, asserted exactly one additional thread request and parsed query parameter `page=2`, then verified page-two fixture content.
- [x] For each size, scrolled floor 8 to a 17 DIP in-row offset, recorded its pre-refresh Y coordinate and first visible row, blocked the refresh response, and recorded waiting, failed, and dismissed states without further scrolling. All four Y coordinates stayed within 0.5 DIP, and the first visible row and offset remained unchanged. Registered an idempotent release callback immediately after creating the blocking channel.
- [x] Ran the focused regressions and recorded the four coordinates for each size in test output.

### Task 3: Run cross-platform refresh and full offline regression

**Files:**
- Verify existing tests only; do not broaden implementation changes beyond the desktop refresh feedback layout and its regression tests.

- [x] Ran desktop and Android `悄悄话` / `IWhisper` alias refresh tests, including failure retention and retry-to-top behavior; ran desktop page-two failure/retry-to-page-one-top behavior.
- [x] Ran the existing refresh scenarios: desktop refresh failure preserves floor 8 and 17 DIP offset; desktop refresh from a reminder succeeds, clears old `Sub`, and profile return stays at top; Android inserted-floor refresh resets row/offset and failed-refresh retry succeeds at top.
- [x] Ran `go run ./tools/dev check` and `go run ./tools/dev check --scope gio-probe` from the repository root; both passed.
- [x] Ran core and desktop `-race`, Android UI `-race` with `GOWORK=off`, desktop `TestCaptureMigratedScreens`, and Android `TestPreviewScreens` including dark thread views with `GOWORK=off`.
- [x] Recorded changed files, the request counts at both sizes, the four floor Y coordinates per size, and regression command outcomes for delivery.
