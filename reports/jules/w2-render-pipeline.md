# w2-render-pipeline

## Verdict
The render pipeline in `cmd/` mixes output rendering with validation, uses bare string formatting for human-readable numbers, lacks clear interface separation for rendering configurations, tightly couples zoom string parsing with pipeline sizing, and spreads output string checks across gated helpers. While issue #112 handles extracting packages, these specific cleanups improve modularity and reduce coupling within the render pipeline components themselves.

## Evidence

1. **Output Rendering and Validation Coupling (Severity: Low, Effort: S)**
`cmd/render_output.go:57`: `func renderCheckedGated(w io.Writer, vp image.Image, rc renderCfg, gate *renderCheckGate) error {`
The output function `renderCheckedGated` takes a validation gate `renderCheckGate` and performs structural ANSI validation (`validateRenderedANSI`) internally, coupling rendering and validation loops.
*Smallest next step*: Refactor `renderCheckedGated` into a composition of a raw writer and a validating decorator/middleware that wraps `io.Writer`.

2. **Primitive Obsession in Human-Readable Formatting (Severity: Low, Effort: S)**
`cmd/media_meta.go:164`: `func humanSize(n int64) string {`
Metadata helpers like `humanSize`, `formatDuration`, and `formatBitrate` use hardcoded `fmt.Sprintf` and manual math for display logic within the `cmd` package, missing a dedicated formatting type/utility.
*Smallest next step*: Introduce an `internal/format` package or type with Stringer methods to handle size, duration, and bitrate display formatting instead of raw functions.

3. **Tight Coupling in Render Pipeline Sizing (Severity: Medium, Effort: M)**
`cmd/render_pipeline.go:130`: `func explicitZoomRenderTarget(srcW, srcH, termCols, termRows int, rc renderCfg, initialZoom string) (int, int, bool) {`
The `explicitZoomRenderTarget` function parses raw "w", "h", and string `k` (`viewgeom.ParseZoomK(initialZoom)`) zooming directly inside the sizing calculation logic, mixing parsing rules with geometry logic.
*Smallest next step*: Parse `initialZoom` into a typed configuration struct (`viewgeom.ZoomConstraints`) early, and pass the struct to `explicitZoomRenderTarget`.

4. **Implicit Interface for Render Configuration (Severity: Medium, Effort: S)**
`cmd/render_pipeline.go:209`: `func (m renderMode) renderAspectCorrection() (num, den int) {`
`cmd/render_pipeline.go:217`: `func (m renderMode) renderCellSize() (cellW, cellH int) {`
`renderMode` implements these geometry functions, but they are not formalized as an interface, causing pipeline functions to rely heavily on concrete `renderCfg`/`renderMode` structs.
*Smallest next step*: Define a `RenderGeometry` interface covering `renderAspectCorrection()` and `renderCellSize()` to decouple geometry calculations from concrete configurations.

5. **Panicking over Errors in Shared Helpers (Severity: Medium, Effort: S)**
`cmd/render_pipeline.go:148`: `func fitRenderedImage(img image.Image, cols, rows int, rc renderCfg) image.Image {`
`fitRenderedImage` delegates to `fitRenderedImageChecked` and unconditionally panics on error (`panic(err)` at line 151). The pipeline should not panic internally.
*Smallest next step*: Bubble the error up to the caller in `fitRenderedImage` (modifying callers accordingly) and remove the panic wrapper.

## Ticket corrections
No corrections to existing tickets. These proposals focus on internal decoupling and refactoring rather than the package-level extractions planned in #112.
