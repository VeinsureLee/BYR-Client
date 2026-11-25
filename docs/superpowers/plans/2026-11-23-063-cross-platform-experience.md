# BYR-Client 0.6.3 Cross-Platform Experience Implementation Plan

> **For agentic workers:** This request explicitly asks for implementation in the current workspace, so execute this plan inline and complete each phase's regression gate before beginning the next. Do not commit unless asked.

**Goal:** Implement the `0.6.3` input, empty-state, navigation restore, storage/cache, palette, and API/UI boundary revisions for the MyGo desktop and Gio Android clients while preserving existing module entry points and `state.json v1` compatibility.

**Architecture:** Keep each client responsible for widgets, focus, navigation, scroll anchors, rendering, and native interaction. Move request/session/persistence rules behind shared `core/application` use cases, preserve protocol and local-state compatibility, and use explicit Android bootstrap/platform adapters. Implement the user-specified phases in order; run each phase's package tests before moving to the next.

**Tech Stack:** Go workspaces and independent Android module (`GOWORK=off`), MyGo desktop, Gio Android, `httptest`, shared `core/application`, `core/forum`, and `core/local`.

## Global Constraints

- The compatibility baseline is `cc7fe4d` (0.6.3); preserve existing module and packaging entry points and `state.json v1`.
- Use only offline fixtures and test credentials for diagnostics; never issue real forum writes from tests.
- Preserve exact case, punctuation, Unicode, long-text suffixes, selections, and allowed newlines; input policy is field-specific.
- Native Android input acceptance requires a real device and must be marked pending if unavailable; automated Gio events are not a substitute for IME verification.
- History clear is per current account; cache clear affects only rebuildable media, cancels old work, and stops animation.
- Use shared semantic color values for `byr-blue`, `pine`, and `plum`, each in light and dark mode; unknown or absent legacy preference values safely fall back to `byr-blue`.
- Results from old sessions, routes, requests, history writes, and media generations must not overwrite newer or cleared state.

---

### Phase 1: Reproduce private-message input, then repair all editable fields

**Files to inspect/modify:**
- Desktop: `apps/desktop/internal/desktop/mail.go`, `search.go`, `compose.go`, `accounts.go`, `browse.go`, `thread.go`, `app_test.go`.
- Android: `apps/android/app/internal/ui/mail.go`, `search.go`, `pages.go`, `compose.go`, `interactions_test.go`, `navigation_test.go`.
- Protocol/API tests: `core/forum/endpoints.go` and existing core/application tests.

- [ ] First reproduce from the write-mail route with the exact sequence `aBcD-URL_9中文😀`, append a non-prefilled English suffix, replace a middle selection, paste two lines, confirm IME candidates, delete, undo, and submit. Capture input-event text, widget value, send snapshot, and decoded `title`/`content` from an `httptest` server. Test the Android title separately from fields using `Editor.Submit`.
- [ ] Before changing code, run the focused desktop and Android mail/input tests to record whether the offline reproducer fails and the earliest layer where any value differs.
- [ ] Centralize Gio editor event consumption so each event batch is applied once before a submit snapshot is read. Normalize line separators only for single-line fields; keep body newlines and provide visible validation for rejected password separators.
- [ ] Apply explicit field policies to login, search, mail recipient/title/body/forward, compose/reply/edit, board names, link URL/display text, blacklist names, lookup-user, and bet points. Keep password masked, text areas multiline, and numeric validation explicit.
- [ ] Add offline table-driven input tests for mixed case, Unicode, suffix preservation, cursor selection replacement, multiline paste policy, undo/delete, same-frame edit plus submit, candidate confirmation without implicit send, exactly-one submit, and failed-send content retention.
- [ ] Run `go test ./apps/desktop/internal/desktop`, `cd apps/android/app && GOWORK=off go test ./internal/ui`, and the relevant core/forum tests. Do not begin Phase 2 until these pass or are recorded as blocked by a confirmed environment constraint.

### Phase 2: Center empty states and align search

**Files to inspect/modify:**
- Desktop: `apps/desktop/internal/desktop/style.go`, `account.go`, `browse.go`, `search.go`, `mail.go`, and screenshot/test fixtures.
- Android: `apps/android/app/internal/ui/pages.go`, `search.go`, `account.go`, `app.go`, `navigation_test.go`, `preview_test.go`.

- [ ] Check recent history, sections, favorites, board, search, blacklist, and messages against the working private-message empty state. Give first-load, failed, successful-empty, and refresh-with-old-data distinct states.
- [ ] Center empty-state blocks in the content viewport after title, filters, navigation, and insets; retain vertical scrolling when height or font scale prevents fitting.
- [ ] Correct Android search tab/button text measurement using actual text dimensions and baseline, not the button cell dimensions. Align both modes with and without the clear control; make clearing text issue no request.
- [ ] Add Android measurement/interaction assertions for `「查找人物」` and `「开始全站搜索」` centers within 2dp in normal space at 320, 360, 420dp and enlarged text; verify keyboard-constrained layouts remain scrollable.
- [ ] Verify mode, submitted query, page, and list location survive opening a result and returning. Run the desktop and Android UI package tests and preview tests before Phase 3.

### Phase 3: Restore Android section browsing and desktop home position

**Files to inspect/modify:**
- Android: `apps/android/app/internal/ui/app.go`, `route_state.go`, navigation/home/section page files and `navigation_test.go`.
- Desktop: `apps/desktop/internal/desktop/app.go`, `browse.go`, `list_state.go`, and `app_test.go`.

- [ ] Capture account-scoped Android section module, route stack, page number, first visible stable item ID, and offset when leaving the Home tab. Do not overwrite the section snapshot when a thread is opened from search or reminders.
- [ ] Restore the existing valid route/data first; if missing, reload that same route and page, then restore the stable item anchor after the matching response. Keep snapshots separate for All, Favorites, Top, Recommended, Timeline, subsection, and board.
- [ ] Test All → subsection → board page 2 → target topic → Search → Messages → Me → Home. Assert selected module, route stack, page, first visible ID and offset. Also test favorites, top, recommendations, timeline, reminder-to-same-topic, account switch, delayed response, and desktop return to selected section/list location.
- [ ] Run the full desktop package and `GOWORK=off` Android UI package before Phase 4.

### Phase 4: Add storage/cache controls and clear-history semantics

**Files to inspect/modify:**
- Shared: `core/application`, `core/local/store.go`, `core/local` tests.
- Desktop: `apps/desktop/internal/desktop/account.go`, settings page, `images.go`, and tests.
- Android: `apps/android/app/internal/ui/account.go`, settings page, `images.go`, preview and lifecycle tests.

- [ ] Add an application use case that clears only the active account's reading history under the existing local-store transaction, invalidates visits queued before clear, and allows visits after clear to be recorded.
- [ ] Add a separately confirmed `「清空最近阅读」` action in settings and history. On cancellation keep history; on success show centered `「还没有阅读记录」`; on failure keep entries visible and provide retry.
- [ ] Add `「设置 → 存储与缓存」` with separate `「清除缓存」` and `「清空最近阅读」` actions. Clear only rebuildable image/decode/animation caches; do not clear accounts, drafts, preferences, saved/downloaded images, or other-account history.
- [ ] On cache clear, cancel old-generation image work, stop animations, release decoded resources, reject late callbacks, then allow visible images to fetch again. Test two accounts, pending image requests, cancellation, failure, and late callback/queued-history interleavings.
- [ ] Verify old `state.json v1` fixtures load unchanged and new optional preference fields do not break restoration. Run core/local, desktop, and Android package tests before Phase 5.

### Phase 5: Add three semantic palettes and apply the shared first-batch UI

**Files to inspect/modify:**
- Shared: add `core/appearance` semantic palette values; update `core/local` preference encoding and `core/application` preference use cases.
- Desktop: `apps/desktop/internal/desktop/style.go`, settings, shell, search, mail/message, thread and screenshot tests.
- Android: `apps/android/app/internal/ui/style.go`, settings, shell, search, mail/message, thread and preview tests.

- [ ] Define `byr-blue`, `pine`, and `plum` using the exact light/dark semantic values in `docs/ui-redesign.md#063-体验修订`. Keep success/error/warning roles independent of brand color.
- [ ] Persist the optional palette ID without changing `state.json v1`; default missing/unknown IDs to `byr-blue`, preserve existing display mode, font scale and IPv6 preferences, and retain the last saved selection on write failure.
- [ ] Add `「外观」` palette previews and selection beside existing display-mode and font-size controls. Update navigation, links, primary buttons, focus rings, input borders, selected states, surfaces and text in Search, Settings, Messages and Thread pages.
- [ ] Use accessible named light refresh controls with 36×36 DIP desktop and 48×48dp Android hit areas; preserve stable feedback space. Switching palette must keep route, rendered data, focus/input buffer, scroll anchor and in-flight request identity.
- [ ] Test all six palette/mode combinations, persisted restart, unknown legacy values, contrast/state pairs, focus and large-font layouts. Capture desktop 720×480/1100×760 and Android 320/360/420dp, keyboard and dark-mode previews before Phase 6.

### Phase 6: Decouple search first, then migrate application use cases and assembly

**Files to inspect/modify:**
- Shared: `core/application/search.go`, `app.go`, new use-case files, `core/open.go`, `core/architecture_test.go`, protocol tests.
- Android: add `apps/android/app/internal/bootstrap`; update `main.go`, `internal/ui/app.go` and feature pages.
- Desktop: `apps/desktop/internal/bootstrap`, feature pages and assembly.
- Protocol/local packages: migrate only after production callers use the application boundary; keep packaging/module entry points unchanged.

- [ ] Send both real search forms through shared `application.Search`; add `LookupUser(context.Context, string) (forum.User, error)` and bind both calls to one session generation, route identity, request serial, page and cancellation lifecycle.
- [ ] Test search success, empty results, partial failures, cancellation, expired credentials, account switch during request, and stale success/error rejection through the UI entry points and `httptest`.
- [ ] Migrate messages, sections/favorites, media, preferences and history one feature at a time, using narrow consumer interfaces only where a UI substitute is needed. Keep protocol URL/form/JSON/OAuth behavior and v1 local fixtures under contract tests.
- [ ] Move Android platform directory, credentials, local store, HTTP configuration and initial state assembly from `ui.start` into `internal/bootstrap`, preserving asynchronous initialization and recoverable startup errors. Keep `main.go`, MyGo, `gogio`, binary, APK and module boundaries intact.
- [ ] After all production UI callers have moved, make `App.Forum` and `App.Local` private and expose only application use cases/session info. Then move HTTP implementation/DTOs under a core-private protocol package without creating an import cycle; update the Gio probe to its supported core entry point.
- [ ] Run the affected module tests after each migrated feature. Once package boundaries move, run `go run ./tools/dev check --scope gio-probe` and `go run ./tools/dev check android-core`; finish with `go run ./tools/dev check`, race tests and all requested captures.

### Final acceptance and reporting

- [ ] Report private-message reproduction environment, actual root cause layer if reproduced, and clearly mark native Windows/Android IME checks that could not run.
- [ ] Report each phase's changed paths and regression exit status, plus actual `httptest` `title`/`content` values.
- [ ] Report empty-state/search geometry, restored navigation anchor data, cache/history scope and late-callback results, all six palette captures, and the API/module boundary checks.
- [ ] List screenshot output paths and any remaining environment-limited verification; do not claim simulated events were real IME testing.
