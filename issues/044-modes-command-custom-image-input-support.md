# 044 — Modes Command Custom Image Input Support

**Status**: Closed (Implemented `-i / --image` and positional path support in `cmd/modes.go`)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [`cmd/modes.go`](file:///home/uwe/projects/cati/cmd/modes.go), [`docs/System.md`](file:///home/uwe/projects/cati/docs/System.md)

---

## 1. Problem & Motivation
Currently, `cati modes` only evaluates render modes on the embedded `cati` logo and `emojig-icon.svg`. Users and developers need to evaluate how different render modes, geometry sets, and smart candidate solvers perform on arbitrary user-provided images and photos.

## 2. Technical Specification
- Add `-i` / `--image` repeatable flag (and support for positional image path arguments) to `cati modes`.
- **Single Image Mode**: If one image is passed (`cati modes -i image.png`), render that image per mode, showing standard vs `+smart` side-by-side.
- **Image Pair Mode**: If two images are passed (`cati modes -i left.png -i right.png`), render the pair side-by-side.
- Retain default embedded `cati` + `emojig` pair when no custom image is supplied.
- Ensure image loading supports PNG, JPEG, SVG (via `halfblock.LoadImage`), and WebP if available.

## 3. Implementation Plan
1. Add `--image` / `-i` flag to `modesCommand()` in `cmd/modes.go`.
2. Update `runModesDemoSelectedFiltered` to load specified image(s) or fallback to embedded assets.
3. Update `renderModePairRaw` to support single-image layouts.
4. Add CLI unit tests in `cmd/modes_test.go` verifying custom image evaluation.
