# Image Viewer Gestures and Save Implementation Plan

> **For agentic workers:** Use this plan task-by-task; verify desktop and Android behavior separately and do not add broad Android storage permissions.

**Goal:** Open the original image directly when clicked on PC, and provide Android pinch zoom, pan, and a user-accessible save action.

**Architecture:** Reuse the existing parsed `URL`/`FullURL` distinction and image caches. Pass the original URL to the desktop viewer on thumbnail clicks. On Android, keep zoom/pan state in the viewer, consume Gio touch-pointer events in the canvas, and route saving through a narrow platform capability rather than global storage APIs.

**Tech Stack:** Go, Gio pointer/layout operations, MyGo image/modal components, existing `content.Parse` and `Forum.FetchAsset`.

## Global Constraints

- Open the full/original URL on an inline-image click; remove the separate “查看原图” action that duplicates the click.
- Android zoom must be bounded and retain aspect ratio; pan is available only when zoomed beyond fit.
- Save the original encoded bytes (not a screenshot or re-encoded frame); do not request broad filesystem permissions.
- Image fetching must continue through `Forum.FetchAsset` so existing origin validation and credential scoping are preserved.

---

### Task 1: Make PC image click open the original

**Files:**
- Modify: `apps/desktop/internal/desktop/images.go`
- Test: `apps/desktop/internal/desktop/image_test.go` (create)

- [ ] Add tests for thumbnail-only image click, distinct thumbnail/full URL click, and absence of a redundant “查看原图” action. Assert the viewer URL is the full URL when present.
- [ ] Run `go test ./...` in `apps/desktop` and confirm the new original-URL assertion fails before implementation.
- [ ] Extend the image-render helper to accept an optional full URL; on click set `imageView` to full URL with thumbnail fallback. Remove the separate full-image button from `bodyView` while preserving attachments and emoji behavior.
- [ ] Run desktop image and UI tests; verify the existing save-original modal continues fetching `imageView`.

### Task 2: Implement Android pinch zoom and pan

**Files:**
- Modify: `apps/android/app/internal/ui/app.go`, `image_viewer.go`
- Test: `apps/android/app/internal/ui/image_test.go`

- [ ] Add unit tests for a two-pointer pinch-in/out changing scale within `minImageScale`/`maxImageScale`, one-pointer pan after zoom, pointer-up reset, and no pan when at fit scale.
- [ ] Run the tests and confirm they fail before adding pointer state.
- [ ] Add a stable gesture tag and pointer-contact state to the viewer. In `imageViewerCanvas`, handle press/drag/release events, derive pinch scale from the distance ratio, pan from the one-pointer delta, and apply a clamped offset while clipping the image to the canvas.
- [ ] Keep +/- controls functional, reset pan on open/close, and run Android UI tests plus an Android build.

### Task 3: Save the original image on Android

**Files:**
- Modify: `apps/android/app/internal/ui/images.go`, `image_viewer.go`, `app.go`
- Modify: `apps/android/app/internal/platform` and the Android host adapter in `apps/android/app/main.go` only after confirming the supported Android export mechanism
- Test: `apps/android/app/internal/ui/image_test.go`, platform save tests

- [ ] Add tests proving the viewer save action passes the original URL's encoded bytes and a safe file name to the platform capability, and reports success/failure without closing the viewer.
- [ ] Run focused tests and confirm the capability/save-path tests fail before implementation.
- [ ] Retain original bytes on each decoded `mobileImage`; save to `Android/data/cn.byr.bbs.byrclient/files/Pictures/BYR-Client` through the narrow host capability and display the resulting path. This is app-specific external storage, not a MediaStore/gallery insertion; do not request `MANAGE_EXTERNAL_STORAGE` or claim gallery visibility.
- [ ] Run Android unit/UI tests and production build; manually verify the saved file opens as the original format and image viewer remains usable.
