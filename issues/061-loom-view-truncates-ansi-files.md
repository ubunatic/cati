# 061 — Loom view truncates ANSI files

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: `examples/mediabrowse/mockup/mediabrowse.ansi`, `loom/issues/138`, `loom/cmd/loom`

---

## 1. Problem & Motivation

Running `loom view examples/mediabrowse/mockup/mediabrowse.ansi` displayed output that appeared capped/truncated when the terminal width or default pane limits (50 columns) clipped the 106-column mockup.

## 2. Technical Findings & Resolution

1. **Loom Issue 138 (Delivered in Loom repo)**:
   - `loom.View` now provides full 2D offset and panning methods (`OffsetX`, `OffsetY`, `Pan`, `SetOffset`, `Offset`) with visual-width and ANSI style-preserving horizontal line clipping.
   - `ansiviewer` and `loom view` now support full-width rendering and 2D panning via arrow keys (`left`/`right`/`up`/`down`), `hjkl`, `pgup`/`pgdn`, and `home`/`end`.
2. Verified in Loom and Cati that wide multi-column mockups (106 cols) can be inspected without data truncation.
