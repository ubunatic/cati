## 2026-03-30 - Zero-Allocation Mask Slicing and LUT Bit Indexing in Sextant Block Solvers
**Learning:** Dynamic slice allocations (`make([]uint8, 64)`) inside inner block evaluation loops (`allMasks()`) and switch-statement-based bit extraction (`sextantBit()`) degrade performance in terminal block solvers across all core counts.
**Action:** Pre-allocate static slice references for immutable mask candidate lists and use fixed lookup tables (`[6]uint8`) for bit-mask index mapping in hot rendering loops.

## 2026-03-30 - Direct image.RGBA Slice Indexing and Byte Buffer ANSI Formatting in Quadblock
**Learning:** Calling `image.Image.At(x, y)` inside quadblock cell compilation inner loops causes interface boxing allocations on every pixel access (`color.Color`), producing over 130k allocations per frame. In addition, using `fmt.Sprintf` for ANSI 24-bit RGB sequences (`\x1b[38;2;r;g;bm`) accounts for >25% of all heap allocation objects.
**Action:** Type-assert `*image.RGBA` images in pixel sampling loops to compute direct pixel byte offsets (`img.Pix[off]`), and use `strconv.AppendUint` to format ANSI escape codes directly into reusable `[]byte` line buffers.

## 2026-03-30 - Direct image.RGBA Sampling and Fast Byte Buffer Formatting in Halfblock
**Learning:** In halfblock terminal rendering loops, calling `image.Image.At` in inner pixel sampling and scaling functions (`RenderToGrid` / `ScaleNN`), along with using `fmt.Sprintf` and `strings.Builder` per cell in `Render`, caused over 99k heap allocations per frame and severely degraded throughput.
**Action:** Use direct byte slice indexing on `*image.RGBA.Pix` during sampling and nearest-neighbor scaling, and format ANSI true-color sequences directly into a reusable byte slice (`[]byte`) using `strconv.AppendUint` before writing to `io.Writer`.
