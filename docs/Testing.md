---
title: Testing
weight: 35
---

# Testing

The daily test command is:

```bash
make test
```

It runs `go vet ./...` and the default `go test ./...` suite. This covers the
library packages, renderers, CLI logic, spec integrity, and ordinary package
tests. The Makefile runs these test targets with the pinned Go 1.25.0 toolchain
and ignores enclosing `go.work` files so JPEG-based golden renders are stable.

Additional suites are opt-in:

- `make test-player` — tests guarded by the `catiplay` build tag.
- `make test-browser` — tests guarded by the `catibrowse` build tag.
- `make test-integration` — terminal/TUI tests guarded by the `integration`
  build tag; this builds the binaries first.
- `make test-all` — runs the default, player, browser, and integration suites.

Other checks:

- `go test -bench=.` — runs performance benchmarks; benchmarks are not part of
  the normal test run.
- `make preflight` — builds the binaries, runs `go vet`, and checks that the
  demo renderer reports no errors.
- `make docker-test` — runs the tests with the pinned Go toolchain, useful when
  validating PNG golden-image changes.

For a real asset speed comparison, run `cati --bench <mediafile>`. Image
renders repeat within a time budget (`--bench-budget`, default 1.5s total,
split across modes), so results appear within about 2s; `--mode` limits the
run to one mode. Videos decode the complete first video stream without
playback pacing or audio. Width and height default to the terminal size, or
80x24 when stdout is not a terminal.

JPEG-derived render goldens are stored per supported Go runtime because JPEG
decoding can differ by a one-level rounding decision between toolchains. The
current sets are suffixed `.go1.25-minus.png` and `.go1.26-plus.png`; the test
selects the matching set from `runtime.Version()` (the boundary is Go 1.26,
when the standard JPEG implementation changed). Generate or refresh them
explicitly:

```bash
GOWORK=off GOTOOLCHAIN=go1.25.0 go test ./cmd -run TestGoldenRenders -update
GOWORK=off GOTOOLCHAIN=go1.26.5 go test ./cmd -run TestGoldenRenders -update
```

Do not use `-update` to conceal an unexpected pixel change. Add a new runtime
set only after confirming that its differences are decoder/toolchain drift.

Visual rendering regressions are covered by PNG golden tests in
`cmd/golden_render_test.go` and renderer fixture tests. Do not regenerate
goldens merely to make a test pass; follow `docs/RenderingBugPlaybook.md`.

The opt-in static renderer search can be exercised with, for example:

```bash
GOWORK=off GOTOOLCHAIN=go1.25.0 go run ./cmd/cati --width 40 --smart image.png
```

`--smart` evaluates candidates from the requested width down through the
spec-defined 10% bound. Native-capable modes use their spec-declared
render-pixel step, which can try sub-cell widths; terminal-column modes retain
whole-column search. The winner is scored with PSNR and centered in the
requested canvas. It is intentionally not enabled for interactive/video frame
loops yet.

## Fast paths

Optimised code paths (direct `Pix` access, append-based ANSI formatting) are
gated by one shared flag, `core.Fastpath`, read once from `CATI_FASTPATH`
(`0` = simple reference paths; default on). The simple path is the reference:
change and reason about it first, then make the fast path follow. Fast paths
must be output-identical; each split has a parity test that toggles
`core.Fastpath`. Check the whole suite on the simple paths with:

```bash
CATI_FASTPATH=0 GOWORK=off GOTOOLCHAIN=go1.25.0 go test ./...
```

Known exception: `metrics.extractLumaFlat` fast paths drift slightly
(issue 055).
