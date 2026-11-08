# w3-naming

## 1. Extract common `Options` into `core.RenderOptions`
**Severity**: Low | **Effort**: S | **Target**: `v1/halfblock/render.go`, `v1/quadblock/render.go`, `v1/sextant/render.go`, `v1/sparkline/render.go`

Each rendering mode (`halfblock`, `quadblock`, `sextant`, `sparkline`) declares a very similar `Options` struct containing `Rows`, `Jobs`, `OnProgress`, and `NoLinePrefix`. For example, `v1/halfblock/render.go:330` declares:
```go
type Options struct {
	Rows int
	Jobs int
	// OnProgress is called as rendered rows complete. Parallel rendering may
	// invoke it concurrently; callbacks should be quick and concurrency-safe.
	OnProgress func(core.Progress)

	// NoLinePrefix omits the erase-line and carriage-return prefix emitted
	// before each rendered line. Set this when composing output alongside
	// other content on the same terminal row.
	NoLinePrefix bool
}
```
This repetition scatters common configuration. We should extract these shared fields into a new `core.RenderOptions` struct which package-specific `Options` can embed.

**Smallest next step**: Define `core.RenderOptions` in `internal/core` with `Rows`, `Jobs`, `OnProgress`, and `NoLinePrefix` and update `v1/halfblock/Options` to embed it.

## 2. Consolidate `RenderJ` wrapper into `Render`
**Severity**: Low | **Effort**: S | **Target**: `v1/halfblock/render.go`, `v1/quadblock/render.go`, `v1/sextant/render.go`, `v1/sparkline/render.go`

Several packages export a `RenderJ` function (e.g. `v1/halfblock/render.go:633`) specifically for parallel rendering tests:
```go
func RenderJ(w io.Writer, img image.Image, jobs int) error {
	b := img.Bounds()
	return Render(w, img, b.Dx(), Options{Jobs: jobs})
}
```
Since `Options` already includes `Jobs` and `Render` accepts `Options`, this wrapper is redundant and clutters the API.

**Smallest next step**: Deprecate or remove `RenderJ` from `v1/halfblock/render.go` and replace its usage in tests with `Render` passing `Options{Jobs: jobs}`.

## 3. Consolidate `RenderOpts` wrapper into `Render`
**Severity**: Low | **Effort**: S | **Target**: `v1/quadblock/render.go`, `v1/sparkline/render.go`

`quadblock` and `sparkline` export `RenderOpts` (e.g., `v1/quadblock/render.go:946`):
```go
func RenderOpts(w io.Writer, img image.Image, opts Options) error {
	b := img.Bounds()
	cols := (b.Dx() + 1) / 2
	return Render(w, img, cols, opts)
}
```
This is another thin wrapper around `Render` that calculates default columns. We should unify this into `Render` itself (e.g. if `cols <= 0` calculate from bounds).

**Smallest next step**: Move the default `cols` calculation into `v1/quadblock/Render`, remove `RenderOpts`, and update its callers.

## 4. Unify `RenderToImage` and `RenderToImageJ`
**Severity**: Low | **Effort**: S | **Target**: `v1/halfblock/render.go`, `v1/quadblock/render.go`, `v1/sextant/render.go`, `v1/sparkline/render.go`

Similar to `RenderJ`, packages provide a `RenderToImageJ` (e.g., `v1/quadblock/render.go:1008`) alongside `RenderToImage` to support setting the number of jobs.
```go
func RenderToImageJ(img image.Image, opts Options, jobs int) *image.RGBA {
```
The `Options` struct already carries a `Jobs` field. We can drop `RenderToImageJ` entirely and just have `RenderToImage` respect `opts.Jobs`.

**Smallest next step**: Modify `v1/quadblock/RenderToImage` to respect `opts.Jobs` and remove `RenderToImageJ`.

## 5. Standardize `ScaleToFit` in `imgutil`
**Severity**: Low | **Effort**: M | **Target**: `v1/halfblock/render.go`, `v1/quadblock/render.go`, `v1/sextant/render.go`, `v1/sparkline/render.go`

Every mode package implements its own `ScaleToFit` function (e.g. `v1/halfblock/render.go:148`, `v1/quadblock/render.go:759`). While they have slightly different aspect ratio logic depending on the mode, they all essentially scale bounds.
```go
func ScaleToFit(img image.Image, cols, rows int) image.Image {
```
This scaling logic, particularly for simple modes, could be factored into `internal/imgutil` (perhaps using `viewgeom`) to reduce duplication.

**Smallest next step**: Extract the shared terminal-to-pixel aspect ratio scaling logic into an `imgutil` helper function and replace it in `v1/halfblock/ScaleToFit`.
