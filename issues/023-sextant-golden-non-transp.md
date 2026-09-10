# 023 — Sextant golden non-transparent coverage

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: `cmd/golden_render_test.go`

---

Golden images for sextant algo have no bottom transparency.

testdata/demo_checker_20x20/render_sextant_5ch.png

source is: testdata/demo_checker_20x20/source.png

20x20px --prj on term--> 5x2.5ch

bottom char row must have a halfrow of transparency

other algos 5ch golden have this

sextant algo 5ch golden has not


