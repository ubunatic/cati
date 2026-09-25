## 2026-03-30 - Zero-Allocation Mask Slicing and LUT Bit Indexing in Sextant Block Solvers
**Learning:** Dynamic slice allocations (`make([]uint8, 64)`) inside inner block evaluation loops (`allMasks()`) and switch-statement-based bit extraction (`sextantBit()`) degrade performance in terminal block solvers across all core counts.
**Action:** Pre-allocate static slice references for immutable mask candidate lists and use fixed lookup tables (`[6]uint8`) for bit-mask index mapping in hot rendering loops.

## 2026-03-30 - Direct image.RGBA Slice Indexing and Byte Buffer ANSI Formatting in Quadblock
**Learning:** Calling `image.Image.At(x, y)` inside quadblock cell compilation inner loops causes interface boxing allocations on every pixel access (`color.Color`), producing over 130k allocations per frame. In addition, using `fmt.Sprintf` for ANSI 24-bit RGB sequences (`\x1b[38;2;r;g;bm`) accounts for >25% of all heap allocation objects.
**Action:** Type-assert `*image.RGBA` images in pixel sampling loops to compute direct pixel byte offsets (`img.Pix[off]`), and use `strconv.AppendUint` to format ANSI escape codes directly into reusable `[]byte` line buffers.

## 2026-03-30 - Direct image.RGBA Slice Indexing and Reusable Byte Buffer ANSI Formatting in Halfblock
**Learning:** In halfblock rendering, `image.Image.At(x, y)` calls inside `RenderToGrid` and nearest-neighbor scaling (`ScaleNN`/`ScaleToFit`) caused interface boxing allocations for every pixel (99k+ allocs/frame). Furthermore, `fmt.Sprintf` and `strings.Builder` per cell in `Render` generated over 65k string allocations per frame.
**Action:** Fastpath `*image.RGBA` pixel sampling using direct slice offsets (`rgba.Pix[off]`) and write ANSI escape sequences via `strconv.AppendUint` into a reusable `[]byte` line buffer, reducing allocations by 99.9% (99,338 -> 83 allocs/op) and speeding up rendering by 6x.
