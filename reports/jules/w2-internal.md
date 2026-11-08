## Verdict
The `internal/` packages contain duplicated standard library functions and a few opportunities to consolidate utility helpers for better modularity.

## Evidence

1. Remove duplicate `min()` function. (Severity: Low, Effort: S)
**Smallest next step:** Delete `min` from `internal/viewgeom/viewgeom.go` as it is now a Go built-in.
`internal/viewgeom/viewgeom.go:446`: `func min(a, b int) int {`

2. Remove duplicate `max()` function. (Severity: Low, Effort: S)
**Smallest next step:** Delete `max` from `internal/viewgeom/viewgeom.go` as it is now a Go built-in.
`internal/viewgeom/viewgeom.go:453`: `func max(a, b int) int {`

3. Modernize generic math clamping. (Severity: Low, Effort: S)
**Smallest next step:** Use generic `min`/`max` in `clamp` instead of `math.Max`/`math.Min`.
`internal/pixelart/pixelart.go:214`: `func clamp(v float64) float64 { return math.Max(0, math.Min(255, v)) }`

## Smallest next step
See above per finding.

## Ticket corrections
No ticket corrections required.
