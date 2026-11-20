# Multi-Account Management Implementation Plan

> **For agentic workers:** Treat credentials as secrets. Use the platform secure-storage boundary, preserve existing single-account installs, and test account switches as session transitions rather than as a UI-only selection.

**Goal:** Allow BYR-Client users to sign in to and switch between multiple forum accounts on Android and PC.

**Architecture:** Extend the shared application boundary with account-keyed credential operations while keeping passwords out of persistence. Store Android credentials in an atomic, owner-only file under the app-private sandbox and keep desktop secrets in `mygo.Secrets`; the active account marker lives beside the credential set, so no local-preferences schema migration is needed. Preserve the existing single-account formats, switch only after validation/storage succeeds, and reset account-bound UI state before reloading.

**Tech Stack:** Go `core/application`, existing `core/local` versioned JSON store, Android app-private credential adapter, desktop `mygo.Secrets`, Gio and MyGo account/settings UI.

## Global Constraints

- Never store passwords or forum tokens in ordinary settings/state JSON or logs.
- Keep Android secrets inside its existing app-private sandbox with atomic owner-only writes; do not claim hardware-backed encryption. Keep desktop secrets in the existing system credential backend and never write credentials to `state.json`.
- Preserve the current local preference, draft, and reading-history schema; those user records already use normalized usernames as account keys.
- Switching retains saved credentials; forgetting an account is a separate, explicit operation.
- A failed login/switch must not replace the current active session or erase another saved account.

---

### Task 1: Add account-keyed core credential operations

**Files:**
- Modify: `core/application/app.go`, `core/application/multi_account.go`
- Test: `core/application/app_test.go`, `core/application/multi_account_test.go`, `core/application/multi_account_login_test.go`

- [ ] Add tests for list/save/load/remove by normalized username, duplicate account update, expiry, storage failure, `remember=false`, and preserving other accounts on logout/removal.
- [ ] Run `go test ./...` in `core/application` and verify the multi-account tests fail against the single-slot `CredentialStore`.
- [ ] Define account operations on the shared application boundary, keep the active token source synchronized, and make login persist/update only the returned account when remember is enabled. Do not clear other saved accounts for a non-remembered login.
- [ ] Run all core tests and the `core/forum` token-separation tests.

### Task 2: Migrate platform credential stores

**Files:**
- Modify: `apps/android/app/internal/platform/storage.go`, `account_storage_test.go`
- Modify: `apps/desktop/internal/platform/storage.go`, `multi_account_storage_test.go`

- [ ] Add adapter tests for per-account CRUD, active selection, empty logout selection, failed writes, and migration from Android `session.json` / desktop `forum-session`.
- [ ] Run the platform storage test packages and confirm migration/CRUD tests fail before the envelope and secret-store changes.
- [ ] Store an account map plus active username in Android's app-private `session.json`, atomically and with owner-only file permissions; on desktop store the account envelope under `mygo.Secrets` and migrate the old `forum-session` key after a successful write.
- [ ] Verify logging out deselects the active account without deleting the remembered list, while forgetting removes only the requested credential; ensure no token enters ordinary desktop `state.json`.

### Task 3: Make account switching an atomic session transition

**Files:**
- Modify: `core/application/app.go`, `apps/android/app/internal/ui/app.go`, `apps/desktop/internal/desktop/app.go`
- Test: `core/application/app_test.go`, Android `session_test.go`, desktop `app_test.go`

- [ ] Add tests for A→B→A restoring each account's token, old requests not applying after switch, expired target handling, failed activation preserving A, and username-partitioned drafts/history.
- [ ] Run the new tests and verify they fail before adding transition fencing.
- [ ] Implement activation and removal APIs that mutate the in-memory session only after the credential store succeeds; reject missing/expired targets without replacing the current session.
- [ ] In both UIs, prevent account actions during an in-flight operation, clear account-bound state after a successful transition, and reload the active screen. The existing single-operation/serial callback guards fence stale work.

### Task 4: Add account management UI on both platforms

**Files:**
- Modify: Android `apps/android/app/internal/ui/accounts.go`, `app.go`, `pages.go`, `route_state.go`, and `main.go`
- Modify: desktop `apps/desktop/internal/desktop/accounts.go`, `account.go`, `shell.go`, and `app.go`
- Test: Android `accounts_test.go`, `session_test.go`; desktop `accounts_test.go`, `app_test.go`

- [ ] Add UI tests for active-account display, adding a second account through the existing login form, switching, forgetting a non-active account, and leaving login visible when the last account is forgotten.
- [ ] Run focused Android/desktop UI tests and confirm they fail before adding account controls.
- [ ] Add a dedicated account-management list reachable from the active username/self/settings surface. Show active state; make switch and forget separate actions; use existing login validation for adding accounts and never display secret tokens.
- [ ] Run `go run ./tools/dev check --scope all`, then build Android and Windows desktop; verify legacy single-account restore and both account-switch paths.
