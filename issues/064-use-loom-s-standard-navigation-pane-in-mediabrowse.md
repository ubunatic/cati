# 064 — Use Loom's standard navigation pane in mediabrowse

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactoring
**Related**: `examples/mediabrowse/app.go`, Loom `examples/filebrowser/filebrowser.NavigationPane`

---

## 1. Problem & Motivation
`mediabrowse` builds its own file browser around `os.ReadDir`, a path map, and `loom.Choice`. This duplicates filesystem navigation behavior and currently gives the list type-to-filter behavior instead of Loom's standard `/`-gated search. Loom provides a reusable `NavigationPane` with directory navigation, selection callbacks, and gated search by default.

## 2. Goal
`/goal Replace mediabrowse's custom directory-navigation implementation with Loom's NavigationPane while preserving media preview, media filtering, and images-only behavior; stop and report if the pinned Loom API cannot support these requirements.`

## 3. Implementation & Verification Plan
- Integrate `codeberg.org/ubunatic/loom/examples/filebrowser/filebrowser.NavigationPane` as the file pane and use its selection/open callbacks to drive the preview and directory state.
- Preserve the existing media-only toggle and supported-media filtering without maintaining a parallel custom browser.
- Add or update tests for `/`-gated filtering, directory navigation, media selection, and images-only mode; run the mediabrowse tests and required Go checks.
