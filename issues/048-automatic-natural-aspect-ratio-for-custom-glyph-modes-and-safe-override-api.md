# 048 — Automatic Natural Aspect Ratio for Custom Glyph Modes and Safe Override API

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: `docs/SparklinePixelArt.md`, `v1/sparkline/render.go`, `examples/imgbrowser/render.go`, `issues/015-spark-bottom-row-halfcell-fit.md`

---

## 1. Problem & Motivation

When calling `sparkline.RenderToGrid` with custom glyph shapes and cell geometry (e.g. `spec.ResolveGlyphSetExpression("all")` which has $6 \times 6$ sub-pixel geometry), omitting or incorrectly setting `Options.AspectX` results in significant visual distortion (e.g. $2\times$ vertical stretching).

In `examples/imgbrowser`, the application code attempted to render `all` mode by passing `AspectX: 1`. Because terminal character cells are physically rectangular with an approximate $1:2$ width-to-height ratio, mapping a square $6 \times 6$ sub-pixel grid into a $1:2$ character cell without horizontal compensation rendered images twice as tall as their natural proportions.

Application developers should not have to manually calculate terminal font metric compensation formulas ($\text{AspectX} = \frac{2 \times \text{CellW}}{\text{CellH}}$). The zero value (`AspectX: 0`) in Go structs should be safe by default.

---

## 2. Technical Findings & Root Cause

1. **In `v1/sparkline/render.go`**:
   `sparkline.Options.cellGeometry()` retrieves default geometry from `defaultCellGeometry(o.Mode)`:
   ```go
   func (o Options) cellGeometry() (cellW, cellH, aspectX int) {
       cellW, cellH, aspectX = defaultCellGeometry(o.Mode)
       if o.CellW > 0 {
           cellW = o.CellW
       }
       if o.CellH > 0 {
           cellH = o.CellH
       }
       if o.AspectX > 0 {
           aspectX = o.AspectX
       }
       return cellW, cellH, aspectX
   }
   ```
   When `o.Shapes` is provided alongside custom `CellW` and `CellH`, `o.Mode` is typically left at `Mode(0)` (which defaults to `aspectX = 1`). Unless the caller explicitly sets `o.AspectX = 2 * CellW / CellH`, `aspectX` remains `1`.

2. **Contrast with `cati` internal pipeline**:
   In `cati`'s internal pipeline (`cmd/interactive.go` and `cmd/render_pipeline.go`), custom glyph modes are wrapped in `v2FitSpec`:
   ```go
   viewgeom.NewV2CellRatio(geom.W, geom.H, 2*geom.W, geom.H)
   ```
   which automatically enforces the $2W : H$ ratio. However, library users calling `sparkline.RenderToGrid` directly do not use `renderCfg` or `viewgeom.V2Spec`.

---

## 3. Proposed Solution

### 3.1 Library Enhancement (`v1/sparkline/render.go`)

Update `sparkline.Options.cellGeometry()` so that when `AspectX == 0`, it automatically derives the natural aspect ratio from `CellW` and `CellH`:

```go
func (o Options) cellGeometry() (cellW, cellH, aspectX int) {
    cellW, cellH, aspectX = defaultCellGeometry(o.Mode)
    if o.CellW > 0 {
        cellW = o.CellW
    }
    if o.CellH > 0 {
        cellH = o.CellH
    }
    if o.AspectX > 0 {
        aspectX = o.AspectX // Deliberate caller override
    } else if cellW > 0 && cellH > 0 {
        aspectX = max(1, 2*cellW/cellH) // Safe natural default for 1:2 terminal cells
    }
    return cellW, cellH, aspectX
}
```

Clarify `Options.AspectX` documentation:
```go
type Options struct {
    // AspectX is an optional horizontal aspect compensation override.
    // By default (0), the natural ratio (2 * CellW / CellH) is computed
    // automatically for standard 1:2 terminal cells.
    // Only set this to a positive value to force deliberate non-standard stretching.
    AspectX int
    ...
}
```

### 3.2 Application Cleanup (`examples/imgbrowser`)

Remove manual aspect calculations in `examples/imgbrowser/render.go`:
```go
grid, err = sparkline.RenderToGrid(img, cols, sparkline.Options{
    CellW:  cw,
    CellH:  ch,
    Shapes: shapes,
    Jobs:   jobs,
})
```

---

## 4. Verification & Resolution

1. **Implemented automatic aspect ratio calculation** in `v1/sparkline/render.go`: `Options.cellGeometry()` automatically computes `max(1, 2*cellW/cellH)` when `AspectX == 0`, while preserving explicit `AspectX > 0` overrides.
2. **Added unit tests** `TestCellGeometryAspectX` in `v1/sparkline/sparkline_test.go` covering 6x6, 2x2, 1x2, 4x8 geometries, explicit overrides, and predefined modes.
3. **Cleaned up `examples/imgbrowser/render.go`** to omit manual `aspectX` calculation and rely on the automatic natural aspect ratio.
4. **Verified test suite & preflight**: `go test ./v1/sparkline/...`, `make test`, and `make preflight` all passed cleanly.

