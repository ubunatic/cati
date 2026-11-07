## Verdict
The `cmd/ssim*.go` and `cmd/metrics*.go` files duplicate identical render dispatch logic. The SSIM pipeline in `computeQuality` computes luminance arrays twice per frame. Lastly, `BoxDownscale` creates a performance bottleneck by relying on `img.At()` interface calls without fast paths for concrete image types.

## Evidence
1. **Duplicate Dispatch (Severity: Low, Effort: S)**: `cmd/metrics.go:24`: ```	case rc.useGlyphs():``` — initiates a `switch` that replicates the rendering dispatch logic identically to `renderSSIM`.
2. **Inconsistent Resize (Severity: Medium, Effort: S)**: `cmd/ssim.go:64`: ```	if rc.useGlyphs() {``` — duplicates the render dispatch and contains inconsistent spark-only resize logic compared to `computeQuality`.
3. **Double Luma Extract (Severity: Low, Effort: S)**: `cmd/metrics.go:64`: ```	refLuma := metrics.LumaGrid(ref)``` — calls `extractLumaFlat` to compute grids, while line 70 (`SSIMLuminance`) independently recomputes the flat buffer from RGBA pixels.
4. **Interface Call Overhead (Severity: High, Effort: S)**: `internal/metrics/metrics.go:275`: ```					r, g, b, _ := src.At(sb.Min.X+sx, sb.Min.Y+sy).RGBA()``` — `BoxDownscale` invokes the slow `At()` method in a nested loop without bypassing it via type assertions for `*image.RGBA`.

## Smallest next step
1. Extract a shared `renderVP(vp image.Image, rc renderCfg) image.Image` function in `cmd` to unify dispatch and bounds-resizing.
2. Introduce `metrics.SSIMLuminanceFromGrids(ref, rend [][]float64) float64` to reuse the luminance matrices computed in `computeQuality`.
3. Add a type switch in `BoxDownscale` to directly iterate `Pix` slices for `*image.RGBA` and `*image.NRGBA`.

## Ticket corrections
No ticket corrections are necessary.
