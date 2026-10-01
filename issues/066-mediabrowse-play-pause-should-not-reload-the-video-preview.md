# 066 — Mediabrowse play/pause should not reload the video preview

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [065 — Mediabrowse video playback and fullscreen shortcuts do not work](065-mediabrowse-video-playback-and-fullscreen-shortcuts-do-not-work.md), `examples/mediabrowse/preview_pane.go`

---

## 1. Problem & Motivation
After selecting a video, mediabrowse loads and displays its preview. Pressing play makes the preview disappear while playback initializes. Pressing play again reloads the still preview and resets the interaction; another press is needed to start playback again. Play/pause should operate on the loaded video without reloading the selected file or losing playback position. Restarting from the beginning on resume is acceptable if Loom cannot resume a paused stream.

## 2. Technical Specification / Findings
`TogglePlay()` changes `playing` and calls `loadWidgetLocked()`, which closes the current widget and recreates either the video or preview widget. This ties playback state changes to file loading and causes the visible reload/reset behavior.

## 3. Implementation & Verification Plan
**Goal**: `/goal Make repeated play/pause actions control the selected video without reloading its preview; verify playback and pause/resume behavior, or stop and report if a required Loom capability is unavailable.`

Separate playback control from file/preview loading, retaining the loaded preview and selected file across toggles. Add regression coverage for repeated play/pause and verify with a manual mediabrowse video playback check.

## 4. Resolution
Separated still preview thumbnail decoding (`p.preview`) from active video stream playback (`p.video`) in `examples/mediabrowse/preview_pane.go`. Repeated play/pause toggles now control video stream advancement and frame drawing directly without destroying or reloading the preview or resetting playback state. Added unit regression coverage and updated integration test assertions for repeated play/pause.
