# 055 — Metrics Luma Fast Paths Diverge From Simple Path

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: `internal/metrics/metrics.go` (`extractLumaFlat`, `extractLumaSimple`), issue 054

---

## Problem

The typed fast paths in `extractLumaFlat` are not output-identical to the
`At()`-based simple path:

- `*image.NRGBA`: uses raw channels, ignoring alpha; the simple path
  premultiplies. Differs for any pixel with `A < 255`.
- `*image.Gray`, `*image.YCbCr`: different rounding/conversion than
  `At().RGBA()` (small drift).

Metrics feed PSNR/SSIM scoring (`--smart`), so the drift could flip a
candidate ranking between `CATI_FASTPATH=1` and `0`.

## Fix direction

Decide the reference semantics in `extractLumaSimple` first, then make each
fast path match it exactly (e.g. premultiply NRGBA, or route translucent
NRGBA through the simple path) and tighten the parity test tolerance.
Check goldens/`--smart` choices per docs/RenderingBugPlaybook.md.
