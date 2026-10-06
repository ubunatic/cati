# 089 — Fix vector mode rune mask mapping for Symbols for Legacy Computing

**Status**: Closed — Corrected vector mode rune masks using exact perimeter geometry and complement pairs
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #088, #087

---

## 1. Problem & Motivation
In vector mode (`-m vec` / `-m vector`), rendered images exhibited inverted or mirror-reflected subcells. For example, rendering `assets/samples/sample-003-darth-daughter.jpg` with `-m vec -W 10` mapped diagonal edges to opposing orientations, creating visual fragmentation and inverted shapes.

## 2. Technical Specification / Findings
1. In `spec/load.go:vectorRuneMask`:
   - `0x1FB9A` (`🮚`, `UPPER AND LOWER TRIANGULAR HALF BLOCK`, black hourglass) and `0x1FB9B` (`🮛`, `LEFT AND RIGHT TRIANGULAR HALF BLOCK`, black bowtie) were completely inverted and coded as 45-degree diagonal splits (`y < x` and `x+y < 4`) instead of hourglass/bowtie shapes.
   - The range `U+1FB3C..U+1FB6F` (`Symbols for Legacy Computing`, Teletext / Minitel G3 smooth mosaic block diagonals) was evaluated with ad-hoc synthetic formulas. Many had inverted axes or filled the opposite half-plane (e.g. `LOWER LEFT` diagonals filled the upper-right or lower-right region).
   - Higher indices `0x1FB4C..0x1FB5B` were mapped using an arbitrary linear raster fill `(x*4+y) >= step`.
2. Exact Geometry:
   - Each of the 44 block diagonals defines a cutting line between 2 perimeter endpoints (`UPPER LEFT`, `UPPER CENTRE`, `UPPER RIGHT`, `UPPER MIDDLE LEFT`, `LOWER MIDDLE LEFT`, `LOWER LEFT`, `LOWER CENTRE`, `LOWER RIGHT`, `UPPER MIDDLE RIGHT`, `LOWER MIDDLE RIGHT`).
   - The filled half-plane is strictly defined by the named corner (`LOWER LEFT`, `LOWER RIGHT`, `UPPER LEFT`, `UPPER RIGHT`).
   - All 22 pairs of opposite diagonals form exact complementary masks (`m1 ^ m2 == 0xFFFF`).
   - All 10 triangular blocks (`1/4`, `3/4`, hourglass, bowtie) have rotational symmetry and exact complement relationships.

## 3. Implementation & Verification Plan
1. Replaced `vectorRuneMask` in `spec/load.go` with exact, precomputed 16-bit bitmasks derived from the mathematical boundary equations with supersampled area integration and exact complement pairs.
2. Verified with `go test -v ./spec` including explicit complement assertions for hourglass and bowtie.
3. Verified full test suite `HTO=0 go test ./...`.
4. Verified CLI rendering `cati assets/samples/sample-003-darth-daughter.jpg -m vec -W 10`.
