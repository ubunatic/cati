# 065 — Mediabrowse video playback and fullscreen shortcuts do not work

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: `examples/mediabrowse/app.go`, `examples/mediabrowse/preview_pane.go`, Loom v0.2.11 update (`50263a3`)

---

## 1. Problem & Motivation
In `mediabrowse`, pressing `p` to play a selected video can make the video disappear instead of starting playback. The `f` fullscreen shortcut also appears to do nothing. This makes two advertised preview controls unreliable.

## 2. Technical Specification / Findings
1. **Key Matching**: The app handled `f` and `f9` by matching `KeyEvent.Key` (`switch k.Key`). In Loom, printable character key events in terminal have `Key == ""` and `Text == "f"`. Matching on `Key` alone prevented `f` from ever toggling fullscreen.
2. **Video Playback Lifecycle Race**: When a video is selected, `preview_pane` starts an async still-frame extraction using ffmpeg. When the user presses `p` to play the video:
   - `TogglePlay` set `playing = true` and created the streaming video widget via `newVideoWidget`.
   - However, `loadWidgetLocked` only incremented `generation` in the still-frame branch.
   - When the in-flight still-frame extraction completed or was cancelled with an exit code, its completion handler saw `p.generation == gen`, called `closeWidgetLocked()` (destroying the active video stream widget), and set `p.message = "Preview error: ..."`.
3. **Pane Focus Key Routing**: App-level shortcuts (`f`, `p`, `m`, `i`) were restricted to `filesFocused`, failing when the preview pane had focus or when in fullscreen mode.

## 3. Implementation & Verification Plan
**Goal**: `/goal Make video play/pause and fullscreen shortcuts work reliably without losing the selected preview; verify keyboard handling and playback in mediabrowse, or stop and report if a required Loom API fix is unavailable.`

- Migrate key event checks in `app.go` and `preview_pane.go` to `k.Is()`.
- Advance `generation` unconditionally at the top of `loadWidgetLocked()` and check `ctx.Err() != nil` to prevent in-flight goroutines from mutating or closing subsequent widgets.
- Allow shortcuts (`f`, `p`, `m`, `i`) to route globally whenever `/` search is inactive, and pass them to search input when `/` search is active.
- Add regression test coverage in `preview_pane_test.go` and `app_test.go`.

## 4. Outcome & Resolution
- Fixed in `examples/mediabrowse/preview_pane.go`:
  - `loadWidgetLocked()` now unconditionally increments `generation` so in-flight async loaders cannot mutate or close replacement video widgets.
  - `SetPath()` and `SetMessage()` reset `playing = false` cleanly.
  - Context cancellation checks ignore cancelled goroutine results.
- Fixed in `examples/mediabrowse/app.go`:
  - Used `k.Is()` for `f`, `f9`, `i`, `m`, `p` shortcuts.
  - Routed shortcuts across panes while search is inactive; preserved `/` search text routing when search is active.
- Added regression tests in `preview_pane_test.go` (`TestPreviewPaneVideoPlayDuringAsyncStillLoad`) and `app_test.go` (`TestAppKeyRoutingAcrossPanesAndSearch`).
- Verified with `go test -v ./examples/mediabrowse`, `go test ./...`, `go vet ./...`, and `make preflight`.
