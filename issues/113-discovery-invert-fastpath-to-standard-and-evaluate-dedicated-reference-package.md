# 113 — Invert fastpath to standard and adopt in-package reference testing

**Status**: Closed — Standardized production paths across core render packages and metrics; added in-package reference_test.go parity suites; fixed NRGBA alpha in sextant and sparkline; deleted core.Fastpath, CLI --fastpath, and Makefile double-runs; make test-q1 green
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: 111 (test catalog roadmap), 054 (PR 8/9 review), 055 (luma fastpath drift), 094 (quadblock parity), 102 (dedup fastpaths), docs/Testing.md

---

/goal Invert fastpath terminology and architecture across cati: the optimized code paths become the sole production standard, in-package `reference_test.go` suites maintain naive for-loop baseline parity on small synthetic fixtures, remove runtime `core.Fastpath` branching and `--fastpath` flags, and eliminate the `Makefile` -tags=fastpath0 double-run; or stop and report when blocked on a user decision.

## 1. Architectural Decision & Analysis (Refined by argon:low, 2026-10-24)

### The Architectural Call:
1. **The optimized path is the standard production path**:
   - `*image.RGBA` / `*image.NRGBA` direct slice access, `strconv.AppendUint` ANSI builders, precomputed lookup tables, and integer bitmask solvers become the standard implementation.
   - Remove `core.Fastpath` boolean runtime switches across production code. The generic `image.Image` (`img.At(x, y)`) path remains ONLY as the `default:` type switch branch for non-RGBA/NRGBA image types (e.g. `image.Gray`, `image.Paletted`).
2. **In-Package `reference_test.go` (Option B) over a Monolithic `v1/reference` Package**:
   - **Zero Public API Pollution**: A public `v1/reference` package would leak naive, slow implementations into the public API.
   - **Internal Struct Access**: The algorithms in `quadblock`, `sextant`, and `sparkline` rely heavily on unexported structs and lookup tables (`quadCell`, `compileCell`, `sextantRunes`, `candidate`, `bitCandidate`). A separate package would require exporting dozens of internals or duplicating ~1,500 lines of tables.
   - **True Naive Reference Loops**: In-package `reference_test.go` allows clean, 50–100 line naive nested `for`-loop reference functions (`renderReference`, `renderToImageReference`) directly alongside the production code.
3. **Eliminate the `make test` Double-Run (`-tags=fastpath0`)**:
   - Remove `CATI_FASTPATH=0 ... -tags=fastpath0` from `Makefile`. This immediately **halves total test execution time and CPU load**.
   - Parity is guarded instead by dedicated sub-second `TestReferenceParity` unit tests running on small (8×8 to 32×32) synthetic fixtures (`RGBA`, `NRGBA`, `Gray`, partial alpha, odd dimensions, non-zero `SubImage` bounds).
4. **De-emphasize & Remove `--fastpath` / `CATI_FASTPATH`**:
   - Drop the `--fastpath` persistent CLI flag and `CATI_FASTPATH` env lookup. Users and library consumers get optimal performance without managing internal knobs.

## 2. Entanglement Findings

1. **`v1/halfblock`**:
   - `render.go`: Only 1 branch (`if core.Fastpath { ... direct Pix ... } else { img.At }`). Very clean to standardize.
2. **`v1/quadblock`**:
   - `render.go`: 5 branches across `safePixel`, `Render`, `RenderToImageJ`, and `computeQuadCell`.
   - `render_fastpath_test.go` already implements differential parity testing; convert to `reference_test.go`.
3. **`v1/sextant`**:
   - `render.go`: 6 branches across `samplePixelFast`, `sampleBlock`, `chooseCell`, `chooseBestCell`, `Render`, and `RenderToImageJ`.
   - *Latent Bug Uncovered by Audit*: `sextant`'s direct slice reader copies `nrgba.Pix` without calling `core.NRGBAToRGBA`, unlike `quadblock` (issue 094). Must fix to call `core.NRGBAToRGBA` when standardizing.
4. **`v1/sparkline`**:
   - `render.go` & `solver.go`: 3 branches. When `core.Fastpath=false`, it currently still runs the 128-bit bitwise math solver and only switches pixel reading to `img.At`. In-package `reference_test.go` will provide the genuine naive 2-pass float/integer search reference.
5. **`internal/metrics`**:
   - `metrics.go`: `extractLumaFlat` has 1 branch. `fastpath_test.go` already compares against `extractLumaSimple`.
6. **`cmd/` & `Makefile`**:
   - `cmd/root.go`: Registers persistent flag `--fastpath` and forwards `CATI_FASTPATH`.
   - `cmd/media_benchmark.go`: Currently benchmarks each mode twice (`fast vs simple`).
   - `Makefile`: Runs all test targets twice (`CATI_FASTPATH=0 ... -tags=fastpath0`).

## 3. Implementation Roadmap

### Milestone 1: Standardize Core Render Packages & In-Package Reference Tests (Delivered: commit 61a85a8)
- **Delivered Summary**:
  - Standardized production paths in `v1/halfblock`, `v1/quadblock`, `v1/sextant`, `v1/sparkline`, and `internal/metrics`.
  - Removed `core.Fastpath` checks; direct slice access is now the standard production path; generic `At` fallback remains for non-RGBA/NRGBA images.
  - Fixed NRGBA partial-alpha premultiplication in `v1/sextant` and `v1/sparkline/solver.go`.
  - Added in-package `reference_test.go` suites asserting parity across synthetic fixtures (RGBA, NRGBA alpha variants, Gray, odd sizes, translated SubImages).
  - Updated algorithm documentation (`docs/QuadPixelArt.md`, `docs/RenderPipelines.md`, `docs/SparklinePixelArt.md`).

### Milestone 2: Strip CLI Knobs, `core.Fastpath`, and `Makefile` Double-Run (Delivered: commit 58e0f1c)
- **Delivered Summary**:
  - Deleted `v1/core/fastpath.go` and `cmd/fastpath_flags_test.go`.
  - Removed `--fastpath` CLI persistent flag and `CATI_FASTPATH` env lookup from `cmd/root.go`.
  - Updated `spec/cli.yaml` and `cmd/cli_defaults_test.go` removing the `--fastpath` exception rule.
  - Simplified `cmd/media_benchmark.go` and `cmd/media_benchmark_test.go` to benchmark standard modes in a single pass.
  - Removed `CATI_FASTPATH=0` and `-tags=fastpath0` double-run targets from `Makefile`.
  - Verified with `go test ./cmd/...`, `make test-short`, and `make preflight`.

### Milestone 3: Documentation & Verification
- **Tasks**:
  - Run `HTO=0 make test-q1` to ensure full Quota-1 test suite pass and measure runtime reduction.
  - Verify `make preflight` succeeds cleanly.
  - Close Issue 113 with decisive evidence.
