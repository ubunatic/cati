# w2-quadblock

## Verdict
Both `v1/quadblock` and `v1/sextant` duplicate several common internal functions such as ANSI formatters (`appendFgRGB`, `fgRGB`) and basic color helpers (`toRGBA`, `isTransparent`), which appear identically across multiple `v1/` rendering packages. Additionally, both packages retain deprecated compatibility wrappers like `RenderJ` despite the concurrency settings now belonging in the `Options` structure. In `v1/sextant`, there is dead code (e.g. `avgRGBA`, `displayMask`, `heuristicMasks`) that is fully unused.

## Evidence

`v1/quadblock/render.go:94`
```go
func isTransparent(c color.RGBA) bool { return c.A == 0 }
```

`v1/sextant/render.go:248`
```go
func isTransparent(c color.RGBA) bool { return c.A == 0 }
```

`v1/quadblock/render.go:61`
```go
func fgRGB(c color.RGBA) string {
```

`v1/sextant/render.go:270`
```go
func fgRGB(c color.RGBA) string {
```

`v1/sextant/render.go:222`
```go
func displayMask(mask uint8) (uint8, bool) {
```

`v1/sextant/render.go:321`
```go
func avgRGBA(pixels ...color.RGBA) color.RGBA {
```

`v1/sextant/render.go:582`
```go
func heuristicMasks(pixels [6]color.RGBA) []uint8 {
```

`v1/sextant/render.go:868`
```go
func maskOverlap(a, b uint8) int {
```

`v1/quadblock/render.go:1000`
```go
func RenderJ(w io.Writer, img image.Image, opts Options, jobs int) error {
```

`v1/sextant/render.go:1059`
```go
func RenderJ(w io.Writer, img image.Image, mode Mode, jobs int) error {
```

## Smallest next step
1. **Severity: Minor, Effort: S.** Centralise the duplicated ANSI color string formatters (`fgRGB`, `bgRGB`, `appendFgRGB`, `appendBgRGB`) and basic color helpers (`isTransparent`, `toRGBA`) into `v1/core` and export them. This improves modularity and cleans up internal packages.
2. **Severity: Minor, Effort: S.** Remove the deprecated compatibility wrappers `RenderJ` and `RenderToImageJ` from `v1/quadblock` and `v1/sextant`, routing callers to use `Options{Jobs: ...}` via `Render` or `RenderOpts`. This cleans up redundant API surfaces.
3. **Severity: Minor, Effort: S.** Delete the dead code in `v1/sextant` (e.g. `displayMask`, `avgRGBA`, `heuristicMasks`, `maskOverlap`). This reduces cognitive load and package size.

## Ticket corrections
Epic 120 tracks unused Go declarations across CLI and render packages. The dead code findings in `v1/sextant` should be resolved as part of Epic 120's rendering child work.
