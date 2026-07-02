# 028 — V2 unconstrained static fit aspect mismatch

**Status:** ✅ Closed  
**Refs:** `internal/viewgeom/v2.go`, `cmd/render_pipeline.go`

---

## Problem

Rendering a rational-aspect v2 mode without an explicit width/height while
stdout is not a terminal could fail source-aspect validation:

```bash
img=assets/samples/sample-001-soldering-practice-2025.jpg
cati "$img" -m=x | wc -l
```

The `six` mode produced a `render aspect mismatch` error with a viewport of
`2560x360` for a `640x480` source.

## Root Cause

`fitDimsRatio` used the wrong no-constraints branch for rational v2 modes:
`rawW = srcW * AspectNum` and a derived height of `srcH * AspectDen / AspectNum`.
For `six` (`AspectNum:AspectDen = 4:3`) that made the corrected viewport aspect
far too wide.

## Resolution

The unconstrained v2 branch now derives render width as
`srcW * AspectNum / AspectDen`, keeps source height as the starting height, and
then lets the existing render-cell snap handle quantization. Added
`TestV2FitNoConstraintsPreservesAspect` for `six`, `six+half`, and `spark+six`.
