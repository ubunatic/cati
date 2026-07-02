# 029 — V2 default static fit uses full terminal width

**Status:** ✅ Closed  
**Refs:** `cmd/render_pipeline.go`, `internal/viewgeom/v2.go`

---

## Problem

In a real terminal, plain static rendering without explicit `--width`, `--height`,
or `--zoom` behaves differently across render modes:

```bash
cati -m=h <img>
cati -m=q <img>
cati -m=s <img>
```

fit inside the terminal box and end up around the expected visible width, while:

```bash
cati -m=x <img>
```

uses the full terminal width for the `six`/v2 path. The issue is not the
`stdout | wc -l` no-terminal case; it is the default real-terminal render path.

## Suspected Root Cause

Legacy modes (`h`, `q`, `s`) use `imgutil.FitDims(...)`, which treats terminal
width and terminal height as a fit box.

V2 modes (`x`, `xh`, `sx`) use `V2Spec.Fit(...)`, which is width-primary. It can
record height overflow as `CutH`, but the static render path ignores that and
still renders at the full requested width. So `x` can fill the terminal width
where height should have constrained the fit.

## Fix Plan

1. Reproduce with a real PTY/fixed terminal size, not `wc`, using a 640×480
   image and modes `h q s x xh sx`.
2. Add a static pipeline regression: with no `--width`, `--height`, or `--zoom`,
   all modes should fit inside the same terminal box and produce the same
   terminal-cell footprint.
3. Change the v2 static path so `x`, `xh`, and `sx` fit inside both terminal
   width and terminal height, matching `h`, `q`, and `s`.
4. Keep width-primary v2 behavior only where intentionally needed, by adding a
   fit mode or a separate method instead of overloading `V2Spec.Fit`.
5. Verify:
   - `cati -m=x <img>` no longer uses full terminal width when height should
     constrain it.
   - `h q s x xh sx` agree on default fit size.
   - `make test`
   - `make preflight`
   - `make install`

## Resolution

- Modified `V2Spec.Fit` to pass `rows` to `fitDimsRatio`, changing it to a fit-inside-both path.
- Added `V2Spec.FitWidthPrimary` to retain the original width-primary fitting logic.
- Added `TestV2FitInside` to `internal/viewgeom/v2_test.go` and `TestAllRenderModesStaticFitInsideTerminalBox` to `cmd/root_test.go` to verify visual agreement and prevent regressions.
- Updated `docs/System.md` with the new fit behavior.
