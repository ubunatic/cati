# 088 — Fix quad mode aspect formula distortion in spec

**Status**: Closed — Fixed quad mode aspect formula in spec/aspect.yaml and planner
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: [#087](087-fix-halfblock-aspect-formula-distortion-and-move-formula-params-to-spec.md)

---

## 1. Problem & Motivation

Quad mode (`-m quad`, `CellW=2, CellH=2`) with `--aspect pixel` was falling through to the `formula.default` ratio (2:3) instead of using the 1:2 font aspect ratio.
Because `CellW=2, CellH=2`, using 2:3 injected a 33.3% vertical stretch:
$\text{idealH} = \text{RenderW} \times (2/2) \times (\text{srcH}/\text{srcW}) \times (2/3)$.
For a 4×4 checkerboard at `-W 4`, $\text{RenderW} = 8$ yielded $\text{idealH} = 5.33 \to 5$ subpixels (3 character rows) instead of 4 subpixels (2 character rows), breaking the aspect ratio and distorting rendering.

## 2. Technical Specification / Findings

Quad mode subpixels are physically 1:2 rectangles (since a terminal cell is 1:2 and divided into 2×2 subpixels). Like halfblock, quad mode is based on 1:2 terminal character cells.
`spec/aspect.yaml` and its schema should specify `quad: { num: 1, den: 2 }` in `formula` alongside `halfblock` and `default`.

## 3. Implementation & Verification Plan

1. Update `spec/schemas/aspect.schema.json` to require `quad` in `formula`.
2. Update `spec/aspect.yaml` to configure `quad: { num: 1, den: 2 }`.
3. Update `spec/aspect.go` and `spec/aspect_test.go` to load and validate `quad`.
4. Update `internal/viewgeom/planner.go` to use `policy.Formula.Quad` when `spec.CellW == 2 && spec.CellH == 2`.
5. Update `cmd/root.go` and `planner_test.go` to propagate the policy.
6. Add unit test in `planner_test.go` and CLI tests in `root_test.go` verifying quad mode pixel aspect rendering.
7. Run `make preflight` and verify with demo commands.

