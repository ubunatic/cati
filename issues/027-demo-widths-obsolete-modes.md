# 027 — demo_widths.go uses obsolete render mode names

**Status:** ✅ Closed  
**Refs:** `scripts/demo_widths.go`, `Makefile`

---

## Problem

Running `make preflight` failed during the `Checking demo-widths for render errors...` step:

```
Checking demo-widths for render errors...
6   (err)   (err)   (err)   (err)   (err)   (err)   (err)   (err)
...
FAIL: render errors found
```

The underlying script `scripts/demo_widths.go` was trying to execute the `cati` binary with obsolete render mode flags (`-m halfblock`, `-m quad/splithalf`, and `-m spark/quad`). Following the render-mode simplification in commit `75b62bae` (Refs #025), these old mode strings are no longer valid and resulted in CLI errors.

## Root Cause

`scripts/demo_widths.go` had hardcoded definitions for tested modes that did not reflect the new canonical spec-driven names defined in `spec/render_modes.yaml` (`half`, `quad`, `spark+quad`).

## Fix

Updated the mode array in both `runVideoMode` and `main` in `scripts/demo_widths.go` to use the new canonical names:
- `"halfblock"` -> `"half"`
- `"quad/splithalf"` -> `"quad"`
- `"spark/quad"` -> `"spark+quad"`
