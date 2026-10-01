## 2026-03-30 - Zero-Allocation Mask Slicing and LUT Bit Indexing in Sextant Block Solvers
**Learning:** Dynamic slice allocations (`make([]uint8, 64)`) inside inner block evaluation loops (`allMasks()`) and switch-statement-based bit extraction (`sextantBit()`) degrade performance in terminal block solvers across all core counts.
**Action:** Pre-allocate static slice references for immutable mask candidate lists and use fixed lookup tables (`[6]uint8`) for bit-mask index mapping in hot rendering loops.

## 2026-03-30 - Direct image.RGBA Slice Indexing and Byte Buffer ANSI Formatting in Quadblock
**Learning:** Calling `image.Image.At(x, y)` inside quadblock cell compilation inner loops causes interface boxing allocations on every pixel access (`color.Color`), producing over 130k allocations per frame. In addition, using `fmt.Sprintf` for ANSI 24-bit RGB sequences (`\x1b[38;2;r;g;bm`) accounts for >25% of all heap allocation objects.
**Action:** Type-assert `*image.RGBA` images in pixel sampling loops to compute direct pixel byte offsets (`img.Pix[off]`), and use `strconv.AppendUint` to format ANSI escape codes directly into reusable `[]byte` line buffers.

## 2026-03-30 - Direct Slice Pixel Sampling and Reusable ANSI Line Buffering in Sextant Renderer
**Learning:** Sextant rendering executed `image.Image.At(x, y)` per subpixel in `avgRegion`, creating over 33k-44k heap interface allocations per frame. Formatting ANSI escapes using `fmt.Sprintf` and `strings.Builder` per line added further allocation churn.
**Action:** Fast-path pixel sampling in `avgRegion` via direct slice indexing (`Pix[off]`) on `*image.RGBA` and `*image.NRGBA`, gate behind `core.Fastpath`, format ANSI escapes directly into a reusable `[]byte` line buffer using `strconv.AppendUint`, and check all-transparent subpixels upfront.

## 2026-03-30 - Fastpath Direct Pix Indexing and Reusable Byte Buffer Formatting in Halfblock
**Learning:** `halfblock` rendering lacked `core.Fastpath` optimizations, leading to interface boxing allocations on every pixel access (`image.Image.At(x, y)`) and heap allocations for every cell escape string (`fmt.Sprintf` and `strings.Builder`), incurring 99k+ allocations and 5.19 MB memory per 256x128 frame.
**Action:** Fastpath `*image.RGBA` pixel sampling via direct `Pix` offsets, pre-allocate contiguous `core.Cell` backing slices for grid rows, and format ANSI 24-bit escape sequences directly into a reusable `[]byte` line buffer using `strconv.AppendUint`.
