## Verdict
This report outlines three targeted cleanup and allocation-reduction improvements for `v1/sparkline` based on the codebase exploration and strict adherence to the required rules. Ideas already covered by active issues (e.g. 016, 018) have been omitted.

## Evidence

### 1. Remove deprecated `FindOptimalSplit`
**Severity**: Low
**Effort**: S
`v1/sparkline/render.go:923`
`func FindOptimalSplit(img image.Image, bounds image.Rectangle, x0, x1, y0, y1 int, mode Mode) (bestK int, barColor, emptyColor color.RGBA, bestErr float64) {`
**Why it helps**: `FindOptimalSplit` is documented as `Deprecated: use FindBestCell`. It should be removed from the public API or moved purely to test helper scope to clean up legacy unused methods.
**Smallest next step**: Remove `FindOptimalSplit` and update any tests referencing it to use `FindBestCell`.

### 2. Remove `fmt.Sprintf` for foreground ANSI codes
**Severity**: Moderate
**Effort**: S
`v1/sparkline/render.go:1131`
`func fgRGB(c color.RGBA) string {`
**Why it helps**: It returns strings using `fmt.Sprintf`, which causes heap allocations. We can switch to using the existing `appendFgRGB` to append bytes directly without string generation or allocation.
**Smallest next step**: Remove `fgRGB` and replace its calls with the `appendFgRGB` zero-allocation byte-appender.

### 3. Remove `fmt.Sprintf` for background ANSI codes
**Severity**: Moderate
**Effort**: S
`v1/sparkline/render.go:1135`
`func bgRGB(c color.RGBA) string {`
**Why it helps**: Similar to `fgRGB`, it returns strings using `fmt.Sprintf`, causing unnecessary allocations. Using the existing `appendBgRGB` will append bytes directly.
**Smallest next step**: Remove `bgRGB` and replace its calls with `appendBgRGB`.

## Ticket corrections
- None.
