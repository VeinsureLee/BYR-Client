# Message and Mobile Navigation Cleanup Implementation Plan

> **For agentic workers:** Use the repository's existing Go/Gio and MyGo patterns, test each behavior before implementation, and keep each task independently reviewable.

**Goal:** Make private conversations the single meaning of “站内信”, separate recommendations from Today's Top, add Android section-page swiping, and correct the requested mobile alignment/empty-state behavior.

**Architecture:** Keep inbox/outbox aggregation in `core/application.Conversation`; remove redundant mailbox navigation from both UI shells without deleting protocol support. Model Android module destinations as route state, keep vertical list scrolling independent from horizontal page swipes, and render reminder empty states in the full content viewport.

**Tech Stack:** Go, Gio (`gioui.org/layout`, `gioui.org/io/pointer`), MyGo UI, existing application/forum APIs and Go tests.

## Global Constraints

- “站内信” means private messages/conversations; do not show redundant inbox/outbox entries in the user-facing messages module.
- Android must retain the existing Gio architecture and use measured pointer gestures; do not add device-specific fixed screen dimensions.
- “今日十大” and “版面推荐” must be distinct destinations with matching titles and independently loaded data.
- Keep `core/application.Conversation` aggregation semantics and server API support unless a UI change requires otherwise.

---

### Task 1: Unify private-message navigation

**Files:**
- Modify: `apps/android/app/internal/ui/mail.go`
- Modify: `apps/desktop/internal/desktop/mail.go`
- Modify: `apps/desktop/internal/desktop/shell.go`
- Modify: account/profile shortcuts that open “我的信箱” in Android and desktop
- Test: `apps/android/app/internal/ui/interactions_test.go`, `apps/android/app/internal/ui/parity_test.go`, `apps/desktop/internal/desktop/app_test.go`

- [ ] Add tests asserting the messages module has only conversation, reply-reminder, and @-reminder destinations; no inbox/outbox selector is rendered or routable from the user-facing messages UI.
- [ ] Run the focused Android, desktop, and core conversation tests; confirm the new navigation assertions fail while existing `Conversation` aggregation still passes.
- [ ] Route the “站内信”/“我的信箱” destination to the aggregated contacts/conversation list. Remove inbox/outbox tabs and tab-selection branches from both shells; keep legacy mailbox endpoints available only where an existing direct detail flow requires them.
- [ ] Run `go test ./...` from each affected module and verify conversation loading still aggregates incoming and outgoing messages.

### Task 2: Give Recommendations a distinct route

**Files:**
- Modify: `apps/android/app/internal/ui/top.go`, `app.go`, `pages.go`, `route_state.go`
- Test: `apps/android/app/internal/ui/navigation_test.go`, `session_test.go`, `preview_test.go`

- [ ] Add route tests that Today's Top and Recommendations can be opened independently, have distinct titles, and call `Forum.TopTen` vs `Forum.Recommendations` respectively.
- [ ] Run `go test ./...` in `apps/android/app` and confirm the new route assertions fail against the shared `PageTop` tab implementation.
- [ ] Replace the `topTab`-only destination distinction with explicit route state (a dedicated page or `PageTop` sub-route); remove the nested “今日十大 / 版面推荐” tab strip from the Today's Top page. Add an explicit Recommendations entry to the section module strip and update login/resume to enter Today's Top.
- [ ] Preserve route-specific list position and stale-response guards for each destination; run the focused navigation/session tests.

### Task 3: Add horizontal swipe between Android section modules

**Files:**
- Modify: `apps/android/app/internal/ui/pages.go`, `app.go`, `module_tabs.go` or a focused new gesture file
- Test: `apps/android/app/internal/ui/navigation_test.go`, `parity_test.go`

- [ ] Add pure gesture/state tests for left/right direction, minimum horizontal threshold, rejection when vertical movement dominates, boundary behavior, and unchanged selection after a tap.
- [ ] Run the new tests and confirm they fail before wiring gestures into the page.
- [ ] Register a bounded Gio pointer target over the section content region; recognize horizontal swipes only when `abs(dx)` exceeds the threshold and dominates `abs(dy)`. Switch among 全部分区、收藏版面、时间线、今日十大、版面推荐 while leaving the horizontal tab strip and vertical list scrolling usable.
- [ ] Verify all five modules remain reachable through taps, swipes do not trigger on vertical list scrolls, and list positions remain route-keyed; run Android UI tests.

### Task 4: Align the full-site search action and center reminder empty states

**Files:**
- Modify: `apps/android/app/internal/ui/search.go`, `mail.go`, `app.go`
- Test: `apps/android/app/internal/ui/navigation_test.go`, `interactions_test.go`

- [ ] Add layout tests at 320dp and 420dp asserting the “开始全站搜索” button shares the input's horizontal bounds and reminder empty text is placed at the center of the body viewport (not merely centered in its natural-height row).
- [ ] Run the focused tests and confirm they fail on the current measurement/centering behavior.
- [ ] Lay out the search field and submit action in one shared inset/width contract; ensure the button's hitbox and painted bounds match. For empty reply/@ results, set the child constraints to the available body viewport before calling `layout.Center`; keep loading/error behavior intact.
- [ ] Run the Android UI test suite and verify no empty state covers the message tabs or pager.
