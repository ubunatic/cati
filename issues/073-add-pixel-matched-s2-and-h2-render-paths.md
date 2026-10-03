# 073 — Add pixel-matched s2 and h2 render paths

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Performance
**Related**: [#071](071-respect-explicit-width-and-height-in-static-renders.md), [#072](072-add-aspect-modes-for-cli-source-mapping.md)

---

## 1. Problem & Motivation

After source padding (#071) and aspect mapping (#072) land, add direct pixel-to-subcell render modes. The current render path includes prescaling or similar preparation that is unnecessary when each source pixel maps directly to a renderer subcell. Removing that work should provide a minimal, fast path from pixels to cells and establish a stable baseline for later optimization.

## 2. Technical Specification / Findings

- Add `s2` (six 2), which maps source pixels directly to sixel subcells. With `assets/doom1.png` (320×200), `--pad 0,1`, and a 160×67 cell target, it must map the padded 320×201 pixels to 320×201 subcells without prescaling.
- Add `h2` (half 2), the corresponding pixel-matched direct path for halfblock mode. Each source pixel maps directly to a halfblock subcell; a 320×200 image therefore maps to a 320×100 cell grid.
- Expose both `s2` and `h2` as selectable modes in the `cati` CLI, including mode listing/help and rendering through the CLI pipeline.
- Keep both paths free of prescaling or equivalent image resizing in the render path. Apply requested source padding and target/aspect mapping before rendering, then pass pixels directly into cell/subcell composition.

## 3. Implementation & Verification Plan

**Goal**: Implement stable `s2` and `h2` pixel-matched render paths that avoid render-path prescaling and produce the expected output, or stop and report when blocked on a user decision or denied permission.

Use `assets/doom1.png` and the existing custom Doom goldens as test assets. Verify the `cati` CLI lists and renders both new modes, check `s2` against the 160×67 Doom golden using `--pad 0,1`, and add corresponding golden coverage for `h2`. Assert the expected cell and subcell dimensions, direct pixel mapping, and that these modes do not invoke prescaling. Run the relevant Go tests and ensure existing render modes remain covered. Treat future speed improvements beyond removing unnecessary prescaling as follow-up work after these paths are stable and match the Doom goldens.
