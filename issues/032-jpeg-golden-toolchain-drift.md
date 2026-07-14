# 032 — JPEG-sourced golden drift across Go toolchain patch versions

**Status:** ✅ Closed
**Refs:** [RenderingBugPlaybook.md](../docs/RenderingBugPlaybook.md), `Dockerfile`, `cmd/golden_render_test.go`

---

## Problem

`TestGoldenRenders` failed on all 50 renders of the 3 JPEG-sourced photo
samples (`sample_darth_daughter`, `sample_soldering_practice`,
`sample_summer_vacation`) while every synthetic PNG-sourced golden (100+
cases) passed cleanly.

## Root Cause

Measured with a throwaway probe (bounds-equal images, pixel-by-pixel RGBA
diff): ~1.8% of pixels differed, every diff exactly 257/65535 per channel —
1 LSB in 8-bit terms — with `image/jpeg` decode as the only step that
distinguishes the two corpora (no other decode/resize step differs between
JPEG- and PNG-sourced cases; the project has no `golang.org/x/image`
dependency and no cgo image path). This is the signature of a Go stdlib
`image/jpeg` rounding difference between whatever exact Go patch produced the
committed goldens and this machine's toolchain — both satisfy `go.mod`'s
`go 1.25.0` minimum, but `go.mod` doesn't pin an exact patch, so different
contributors' installed Go binaries can silently disagree on JPEG decode
output.

## Fix

- Added `Dockerfile` pinning the exact `golang:1.25.0-bookworm` image (and
  bundling `ffmpeg`, closing a separate `ffprobe`-missing test gap as a
  bonus) as the canonical environment for running tests and regenerating
  goldens.
- Regenerated the 50 affected `render_*.png` goldens under `sample_darth_daughter/`,
  `sample_soldering_practice/`, `sample_summer_vacation/` using that pinned
  toolchain. Verified via `-update` + `git checkout` on everything outside the
  predicted 3 folders (per the playbook's "regenerate-all, revert-all-but-intended"
  mechanic) that no other golden changed.
- Added `make docker-test` / `make docker-update-goldens` so golden
  regeneration always happens against the pinned image going forward.

## Golden Impact

50 `render_*.png` files under the 3 `sample_*` folders (24/30/50/80ch ×
5 modes). No synthetic-source golden changed.

## Follow-up

Not fully closed by this fix: pin the exact toolchain via a `toolchain`
directive in `go.mod` too, so `GOTOOLCHAIN=auto` enforces the same Go patch
outside Docker as well.
