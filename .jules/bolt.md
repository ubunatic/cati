## 2026-03-30 - Zero-Allocation Mask Slicing and LUT Bit Indexing in Sextant Block Solvers
**Learning:** Dynamic slice allocations (`make([]uint8, 64)`) inside inner block evaluation loops (`allMasks()`) and switch-statement-based bit extraction (`sextantBit()`) degrade performance in terminal block solvers across all core counts.
**Action:** Pre-allocate static slice references for immutable mask candidate lists and use fixed lookup tables (`[6]uint8`) for bit-mask index mapping in hot rendering loops.

## 2026-03-30 - Direct image.RGBA Slice Indexing and Byte Buffer ANSI Formatting in Quadblock
**Learning:** Calling `image.Image.At(x, y)` inside quadblock cell compilation inner loops causes interface boxing allocations on every pixel access (`color.Color`), producing over 130k allocations per frame. In addition, using `fmt.Sprintf` for ANSI 24-bit RGB sequences (`\x1b[38;2;r;g;bm`) accounts for >25% of all heap allocation objects.
**Action:** Type-assert `*image.RGBA` images in pixel sampling loops to compute direct pixel byte offsets (`img.Pix[off]`), and use `strconv.AppendUint` to format ANSI escape codes directly into reusable `[]byte` line buffers.

## 2026-03-30 - Direct Slice Pixel Sampling and Reusable ANSI Line Buffering in Sextant Renderer
**Learning:** Sextant rendering executed `image.Image.At(x, y)` per subpixel in `avgRegion`, creating over 33k-44k heap interface allocations per frame. Formatting ANSI escapes using `fmt.Sprintf` and `strings.Builder` per line added further allocation churn.
**Action:** Fast-path pixel sampling in `avgRegion` via direct slice indexing (`Pix[off]`) on `*image.RGBA` and `*image.NRGBA`, gate behind `core.Fastpath`, format ANSI escapes directly into a reusable `[]byte` line buffer using `strconv.AppendUint`, and check all-transparent subpixels upfront.

## 2026-03-30 - Stack-Allocated Color Arrays and Closure Elimination in Quadblock Compiler
**Learning:** Returning `[]color.RGBA` from `collectUnique` and allocating temporary slices (`[]*quadCell{left, above}`) inside per-cell quantisation loops generated >8,000 heap allocations per frame in quadblock rendering. Instantiating per-cell closure helpers (`cellAt`) further increased GC heap escapes.
**Action:** Return fixed array structures `([4]color.RGBA, int)` from small color-gathering helpers, inline neighbor pointer checks (`left` and `above`), eliminate inner closure declarations, and pre-allocate ANSI line byte buffers with target line width capacity.

## 2026-03-30 - Zero-Allocation Byte Buffer ANSI Escape Formatting in Sparkline Renderer
**Learning:** Using `fmt.Sprintf` and `strings.Builder` per grid cell in `sparkline.Render` generated >2,200 heap allocations and 245 KB allocation churn per rendered frame.
**Action:** Format ANSI 24-bit RGB escape sequences into reusable `[]byte` line buffers with `strconv.AppendUint` and `utf8.AppendRune` under `core.Fastpath`, reducing heap allocations to 32 allocs/op and cutting render latency by >40%.

## 2026-10-04 - Closed-Form RGB Distance Scoring and Early Exit in 2x3, Six, and S2 Solvers
**Learning:** Evaluating all 64 sextant masks by iterating per-pixel distances (`rgbaDist2`) and calling virtual pixel lookups in 2x3 cell solvers accounted for >90% of rendering CPU time. In candidate solvers, iterating candidates when a cell is completely transparent or has an exact 0-error match added unnecessary candidate search loops.
**Action:** Precompute mask bit index lookup tables (`maskBitIndices[64]`), compute total pixel color sums once per cell, evaluate closed-form squared RGB error equations, add direct 2x3 slice sampling for `*image.RGBA`/`*image.NRGBA`, and add early exits for transparent cells and perfect 0-error candidate matches in `v1/sparkline` and `v1/sextant`.

## 2026-10-05 - Frequency-Based Candidate Weights and Direct Pixel Fastpaths in Quadblock
**Learning:** Re-counting pixel matching coverage and neighbor continuity by iterating over pixel slices for every candidate pair in `pickBestPair()` consumed 26.7% of quadblock CPU execution time. Variadic slice allocation headers in 2-pixel color averaging (`avgRGB()`) added an additional 10.3% flat CPU overhead.
**Action:** Compute candidate frequency counts in a single pass in `collectUnique()`, evaluate pair weights `weights[k] = counts[k]*4 + m[k]` directly over fixed candidate arrays (`[4]color.RGBA`), introduce `avgRGB2()` for 2-pixel color averaging, and fastpath 2x2 subpixel sampling on `*image.RGBA` images in `computeQuadCell()`.

## 2026-10-05 - Lock-Free Wavefront Scheduling and Direct Slice Fastpath in Parallel Quadblock
**Learning:** Enforcing quadblock cell dependency constraints via per-diagonal channels and WaitGroup barriers (`wg.Wait()` per diagonal) causes severe thread contention (220+ barriers/frame) and destroys CPU cache prefetching.
**Action:** Replace per-diagonal channel barriers with a lock-free wavefront scheduler using atomic row progress counters (`atomic.Int32`) and direct `*image.RGBA` / `*image.NRGBA` 2x2 subpixel sampling in `computeQuadCell`, increasing parallel rendering throughput by 35-47%.

## 2026-10-08 - Uniform Opaque Pixel Fastpath and Deferred Cell Struct Construction in Sextant Block Solvers
**Learning:** In 2×3 sextant block solvers, evaluating all 64 candidate masks on uniform blocks where all non-transparent pixels match accounts for redundant candidate search overhead. Additionally, constructing intermediate `cellResult` structs for every candidate mask before checking `better` score conditions causes unnecessary register spill and branch overhead in hot evaluation loops.
**Action:** Short-circuit uniform non-transparent blocks upfront in `chooseBestCell`, compute integer luma (`2126R + 7152G + 722B`) for preferred masks, unroll index sum aggregation via `switch entry.count`, defer `cellResult` struct assembly until after mask selection, and prune candidates with `if score > bestScore { continue }`.
