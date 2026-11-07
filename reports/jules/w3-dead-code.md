# w3-dead-code

## 1. loadAltButtonActions in cmd/browser_config.go
**Finding:** Unused helper function for loading button actions.
**Evidence:**
`cmd/browser_config.go:249`
```go
func loadAltButtonActions() map[string]string {
```
**Severity/Effort:** Minor / S
**Smallest next step:** Delete `loadAltButtonActions` from `cmd/browser_config.go`.

## 2. RasterizeSVG in v1/halfblock/svg.go
**Finding:** `RasterizeSVG` has no callers, only `RasterizeSVGWithTarget` is used.
**Evidence:**
`v1/halfblock/svg.go:37`
```go
func RasterizeSVG(path string) (image.Image, error) {
```
**Severity/Effort:** Minor / S
**Smallest next step:** Evaluate backward compatibility constraints for `v1`. If none, remove `RasterizeSVG` or relocate it.

## 3. FindOptimalSplit in v1/sparkline/render.go
**Finding:** Unused exported function in sparkline rendering.
**Evidence:**
`v1/sparkline/render.go:923`
```go
func FindOptimalSplit(img image.Image, bounds image.Rectangle, x0, x1, y0, y1 int, mode Mode) (bestK int, barColor, emptyColor color.RGBA, bestErr float64) {
```
**Severity/Effort:** Minor / S
**Smallest next step:** Remove `FindOptimalSplit` from `v1/sparkline/render.go`.

## 4. IsCovered in internal/input/input.go
**Finding:** Exported method on `Spec` that is unused.
**Evidence:**
`internal/input/input.go:579`
```go
func (s *Spec) IsCovered(tok string) bool {
```
**Severity/Effort:** Minor / S
**Smallest next step:** Remove `IsCovered` from `internal/input/input.go` or relocate to test files.

## 5. LoadTheme in spec/load.go
**Finding:** Exported theme loader that is unused or whose results are ignored.
**Evidence:**
`spec/load.go:117`
```go
func LoadTheme() (ThemeSpec, error) {
```
**Severity/Effort:** Minor / M
**Smallest next step:** Follow the resolution of #004 §C and #076 to properly wire theme token application in the CLI.
