# Report: w2-halfblock

## Verdict
The `v1/core` and `v1/halfblock` packages have solid structure but contain duplicate logic across parallel rendering, scaling, and progress reporting that can be consolidated to improve maintainability.

## Evidence

1. **Duplicate Progress Check** (Severity: Low, Effort: S)
   `v1/core/progress_reporter.go:31`:
   `	if total > 0 && current > total {`
   Why it helps: Removes a redundant identical conditional block.

2. **Duplicate Worker Orchestration** (Severity: Low, Effort: S)
   `v1/halfblock/render.go:431`:
   `		jobsCh := make(chan int)`
   Why it helps: Centralizes worker WaitGroup orchestration shared by `RenderToGrid` and `RenderToImageJ`.

3. **Duplicate NRGBA Scaling** (Severity: Low, Effort: S)
   `v1/halfblock/render.go:264`:
   `	if srcNRGBA, ok := img.(*image.NRGBA); ok {`
   Why it helps: Removes duplicated nearest-neighbor loops from `ScaleToFit` and `ScaleNN`.

4. **Redundant Context Check** (Severity: Low, Effort: S)
   `v1/halfblock/image_async.go:57`:
   `		if err := ctx.Err(); err != nil {`
   Why it helps: Cleans up duplicated context error checking, as it is already checked on line 42 and implicitly via decoder errors.

5. **Duplicate RGBA Scaling** (Severity: Low, Effort: S)
   `v1/halfblock/render.go:245`:
   `	if srcRGBA, ok := img.(*image.RGBA); ok {`
   Why it helps: Consolidates the RGBA fast-paths in `ScaleToFit` and `ScaleNN`.

## Smallest next step
Apply the proposed deletions and extract `runRowJobs`, `scaleRGBA`, and `scaleNRGBA` helper functions for the duplicates.

## Ticket corrections
No open tickets cover these minor internal cleanup points.
