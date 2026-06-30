# 021 — Golden storage resolution that exactly covers all current & future algos

**Status:** 🟢 Closed  
**Closed:** 2026-06-30  
**Refs:** `cmd/golden_render_test.go` (`upscaleToCharRes`, `goldenRenderToImage`, `TestGoldenTransparentBound`), [SparklinePixelArt.md](../docs/SparklinePixelArt.md), #006, #008, #020

## Resolution

Implemented the 12×24 LCM-derived golden block in `cmd/golden_render_test.go`:

- **`goldenCharBlock()`**: computes minimal aspect-correct `W×H` block from all
  registered render modes via `lcm(CellW)` / `lcm(CellH)`, currently `W=12, H=24`.
- **`upscaleToCharRes()`**: now uses `kX = W/CellW`, `kY = H/CellH` — pure
  integer replication for every mode including sextant (`kX=6, kY=8`).
- **`goldenRenderToImage()`**: trailing `ScaleNN` / halfblock-baseline resize
  removed; all modes produce identical canvas dimensions natively.
- **`TestGoldenTransparentBound`**: updated `halfCharGoldenPx` from hard-coded `4`
  to `blockH/2 = 12`.
- **`TestGoldenBlockIntegerFactors`**: new test asserting `blockW%CellW==0` and
  `blockH%CellH==0` for all modes, and `H==2·W`.
- **`TestUnrepeatLossless`**: new test asserting `unrepeat(upscale(native))==native`
  for halfblock, quad, sextant, and spark — proving no smear is introduced.
- All 295 `render_*.png` goldens regenerated at 12×24 resolution (mechanical
  re-baseline; content unchanged, sextant goldens no longer carry NN smear).
- `docs/SparklinePixelArt.md` golden-pipeline section updated with the new table
  and invariant descriptions.

---

## Problem

The golden comparison canvas is a fixed **4×8 px per character** grid
(`upscaleToCharRes`: `scaleX = 4/CellW`, `scaleY = 8/CellH`). `4 = lcm(1,2,4)`
and `8 = lcm(2,2,8)` — the LCM of the **halfblock / quad / spark** cell
geometries only. Sextant (`2×3`, added later) does **not** tile 8 evenly:

| mode | cell W×H | scaleX, scaleY | per-char result |
|------|---------|----------------|-----------------|
| halfblock | 1×2 | 4, 4 | 4×8 ✓ exact |
| quad | 2×2 | 2, 4 | 4×8 ✓ exact |
| spark | 4×8 | 1, 1 | 4×8 ✓ exact |
| sextant | 2×3 | 2, **8/3 → 2** | 4×6 ✗ |

`8/3` floors to `2`, so a sextant cell becomes `4×6`, then the final
`ScaleNN(refW, refH)` stretches `6 → 8` — a **non-integer vertical resample**.
Sextant is therefore the one family whose goldens carry a lossy NN smear rather
than exact replicated pixels. Every new algorithm whose cell height does not
divide 8 (or width 4) hits the same problem.

## Goal

A storage resolution that represents **every** render mode exactly, with these
invariants:

1. **Physically aspect-correct.** The per-char pixel block must have the same
   width:height ratio as a real terminal cell (≈ `1:2`), so a stored golden
   shows the image at the proportions the user actually sees. (`4×8` already
   satisfies this; the replacement must keep it.)
2. **Uniform storage size per char count.** For a given width (e.g. `80ch`) every
   algorithm's golden has the **identical** pixel dimensions (`width = n·W`,
   `height = rows·H`), so goldens are directly comparable without per-mode
   rescaling.
3. **Low-res algos repeat pixels.** Coarser cell geometries reach the canvas by
   **integer pixel replication** only — never an arbitrary/NN resample. Each
   native pixel maps to an exact `kX × kY` block.
4. **Tests can un-repeat.** Because every upscale is pure integer replication, a
   test can divide back by `(kX, kY)` to recover the native per-cell pixels for
   exact comparison — and assert the replication was lossless (no smear).

## Proposed sizing

Pick the per-char block `W×H` such that `W` is a multiple of `lcm(CellW)`, `H` is
a multiple of `lcm(CellH)`, **and** `H = 2·W` (invariant 1). For the current four
modes (`CellW ∈ {1,2,4}`, `CellH ∈ {2,8,3}`): `lcm(CellW)=4`, `lcm(CellH)=24`.
Smallest aspect-correct block is **12×24**:

| mode | cell W×H | replication kX×kY | result |
|------|---------|-------------------|--------|
| halfblock | 1×2 | 12×12 | 12×24 ✓ |
| quad | 2×2 | 6×12 | 12×24 ✓ |
| spark | 4×8 | 3×3 | 12×24 ✓ |
| sextant | 2×3 | 6×8 | 12×24 ✓ |

All integer factors — no NN stretch anywhere, and `12:24 = 1:2`.

**Future-proofing.** Compute the block from the registered modes' cell dims
rather than hardcoding `4/8`: `W = k·lcm(all CellW)`, `H = 2·W` constrained to a
multiple of `lcm(all CellH)` (raise `k` until both hold). Adding a mode with a
new cell height (e.g. `5`, `16`) automatically enlarges the canvas; a unit test
should assert the chosen block is an integer multiple of every mode's geometry.

## Work

- Replace the hardcoded `4`/`8` in `upscaleToCharRes` with a derived block size
  computed from the mode registry; drop the trailing `ScaleNN` in
  `goldenRenderToImage` once every mode replicates exactly to a common size.
- Add a helper to **un-repeat** (downsample by the known integer factor) and a
  test asserting `unrepeat(render) == native render` — i.e. the canvas adds no
  information and loses none.
- Update `TestGoldenTransparentBound`: the "half a char" bound is `H/2` golden
  rows (12 under a 24-tall block, uniform across modes) instead of the current
  hardcoded `4`.
- Update **every golden-generation entry point** that bakes in the `4×8`
  assumption — they must move to the derived block size together or goldens will
  diverge by writer:
  - `cmd/golden_render_test.go` `-update` path → `render_*.png` (via
    `testhelper.SavePNG`), including the `Algorithm`/`Chars` `tEXt` metadata.
  - `cmd/cli_render_test.go` `-update` path → `cli_*.ansi` (ANSI goldens; their
    per-cell glyph geometry must stay consistent with the px canvas).
  - `internal/sparkline/testhelper` (`GenerateGeometrics`, `SavePNG`) — the
    shared source-image + PNG writer used by both golden tests.
  - any demo/inspection scripts that assume per-cell px (`scripts/demo_widths.go`
    and the `make demo-*` / `preflight` render checks) — verify none hardcode the
    old `4×8`.
- Regenerate all `render_*` and `cli_*` goldens once at the new resolution (a
  single intentional, mechanical re-baseline — call it out in the commit, and
  confirm the diff is *only* a resolution change, not a content change).
- Update [SparklinePixelArt.md](../docs/SparklinePixelArt.md) golden-pipeline
  section and the `upscaleToCharRes` docstring (currently lists only
  halfblock/quad/spark and omits sextant).

## Notes

- This removes the sextant approximation called out after the #020 fix: those 17
  sextant goldens currently bake in the `6→8` NN stretch.
- Relates to #006 (8×8 cell compare for quality metrics) and #008 (viewport
  geometry consolidation) — the same "one geometry path for every mode" goal.
