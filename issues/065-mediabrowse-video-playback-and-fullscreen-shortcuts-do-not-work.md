# 065 — Mediabrowse video playback and fullscreen shortcuts do not work

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: `examples/mediabrowse/app.go`, `examples/mediabrowse/preview_pane.go`, Loom v0.2.11 update (`50263a3`)

---

## 1. Problem & Motivation
In `mediabrowse`, pressing `p` to play a selected video can make the video disappear instead of starting playback. The `f` fullscreen shortcut also appears to do nothing. This makes two advertised preview controls unreliable.

## 2. Technical Specification / Findings
The app handles fullscreen using `KeyEvent.Key` and handles play using `KeyEvent.Text` only while the file pane is focused. `TogglePlay` changes the playing state and reloads the preview widget. Reproduce the symptoms with Loom v0.2.11 and check actual key events, pane focus, async load completion, and widget replacement before choosing a fix.

## 3. Implementation & Verification Plan
**Goal**: `/goal Make video play/pause and fullscreen shortcuts work reliably without losing the selected preview; verify keyboard handling and playback in mediabrowse, or stop and report if a required Loom API fix is unavailable.`

Reproduce the reported symptoms, fix the event/focus or preview lifecycle issue, and add regression coverage for play/pause and fullscreen. Verify with the mediabrowse tests and a manual video playback check.
