# 072 — Add aspect modes for CLI source mapping

**Status**: Closed — unified in CLI geometry pipeline and pure planner
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [#071](071-respect-explicit-width-and-height-in-static-renders.md), [#075](075-unify-cli-render-geometry-contracts-and-validate-route-specific-options.md)

---

## 1. Problem & Motivation
Add a CLI `--aspect` option to choose how the source image maps into a requested cell grid. Keep `default` as the existing aspect-preserving fit. Add `aligned` for the Doom-style case where source pixels are retained and only transparent padding is added to complete the selected mode's cells.

## 2. Technical Specification / Findings
For a 320×200 source in a 160×67 grid using 2×3 cells, `aligned` produces a 320×201 pixel canvas by appending one transparent row. The custom Doom golden exercises this mapping. The CLI's `six` mode currently has 2×6 geometry, so behavior must derive cell dimensions from the selected mode rather than assume 2×3.

Open question: the known use cases need `aligned` and `default`. No third mode has a concrete use case yet.

## 3. Implementation & Verification Plan
**Goal**: Add `--aspect aligned|default` to control mapping into explicit target grids while preserving current behavior by default; stop and report if blocked on a user decision or denied permission.

Verify both modes with explicit width and height: `default` retains current aspect-preserving fitting, while `aligned` fills the requested cell grid, preserves source pixels when they already fit, and pads incomplete mode cells transparently. Cover at least the 320×200 to 160×67 2×3 case.
