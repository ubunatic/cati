# 087 — Fix halfblock aspect formula distortion and move formula params to spec

**Status**: Closed — Fixed halfblock aspect formula and moved formula parameters to spec/aspect.yaml
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: [#078](078-pixel-and-aligned-aspect-edge-cases.md)

---

## 1. Problem & Motivation

When rendering in half-block mode with `--aspect pixel` or `--aspect aligned` (e.g. `cati testdata/demo_checker_20x20/source.png --aspect pixel -m half -W 5`), the rendered output is distorted vertically. A 20×20 checkerboard with 5×5 checks renders as 5×7 pixels instead of 5×5, causing nearest-neighbor row repetition (solid blocks and broken checkerboard pattern).

The root cause is that `internal/viewgeom/planner.go` hardcodes `* 2 / 3` in its derived aspect formulas:
`RenderH = baseRenderW * CellH * srcH * 2 / (CellW * srcW * 3)`
For halfblock mode (`CellW=1, CellH=2`), `(CellH * 2) / (CellW * 3) = 4/3`, stretching halfblock renders by 33%.

## 2. Technical Specification / Findings

1. `spec/aspect.yaml` and `spec/schemas/aspect.schema.json` must own the formula parameters for derived dimensions:
   - `default`: `{ num: 2, den: 3 }` (for composite/sextant 2:3 lattice)
   - `halfblock`: `{ num: 1, den: 2 }` (for halfblock 1:2 cell, preserving 1:1 square subcells)
2. `spec/aspect.go` loads the formula parameters into `PixelAspectPolicy.Formula`.
3. `cmd/root.go` passes the loaded policy into `constraints.PixelPolicy`.
4. `internal/viewgeom/planner.go` uses the formula parameters from `c.PixelPolicy` according to mode subcell geometry.

## 3. Implementation & Verification Plan

1. Update `spec/schemas/aspect.schema.json` to define `formula` with `default` and `halfblock`.
2. Update `spec/aspect.yaml` with the formula params.
3. Update `spec/aspect.go` and `spec/aspect_test.go` to load and validate `Formula`.
4. Update `internal/viewgeom/planner.go` to use `policy.Formula` parameters instead of hardcoded 2 and 3.
5. Update `cmd/root.go` to supply `PixelPolicy` to `constraints` for both pixel and aligned aspect modes.
6. Add unit tests in `planner_test.go` and `root_test.go` verifying halfblock square renders (such as 20x20 checkerboard at `-W 5` producing 3 canvas rows / 5 content rows).
7. Run `make preflight` and verify demo checker rendering.

