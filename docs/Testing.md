---
title: Testing
weight: 35
---

# Testing

The daily test command is:

```bash
make test
```

It runs `go vet ./...` and the default `go test ./...` suite in both fast and
reference modes. This covers the
library packages, renderers, CLI logic, spec integrity, and ordinary package
tests. The Makefile runs these test targets with the pinned Go 1.26.0 toolchain
and ignores enclosing `go.work` files so JPEG-based golden renders are stable.

Use `make test-short` for the fast unit tier, or
`make test-pkg PKG=./internal/input/...` for an isolated, uncached race check.
The short tier disables Go's implicit vet pass; `make lint`, `make test` and
`make preflight` retain vet as the separate static-analysis gate.
Short mode skips media prerequisites and expensive all-mode/sample quality
sweeps; it still checks spec integrity, geometry and PNG goldens. Full tests
retain those sweeps. SSIM variants run as parallel subtests against immutable
source images; golden cases share decoded source fixtures within the test.
Benchmark deadline logic uses a fake clock rather than sleeping.
Short mode also excludes SVG subprocesses and verbose audio/image showcases.
Read-only golden suites run concurrently after tests that change global render
settings; explicit golden updates stay serial.

`internal/ansitest` parses single-cell renderer rows into glyphs, foreground
and background colours, and SGR attributes. Structural assertions compare row
and cell counts as well as style; equivalent combined/separate SGR encodings
compare equal. Parser tests cover resets, indexed/true colours, carriage return,
line erasure and malformed sequences. The known cursor-visibility and mouse
reporting toggles in the viewer prologue preserve painted cell state; screen
switching and unknown controls remain errors. Half-block colour/row tests and the
interactive-versus-static comparison use this helper. Unsupported terminal
operations fail explicitly; it is not a general terminal emulator. Exact byte
comparisons remain appropriate for fast/reference output parity.

Measure uncached timings reproducibly:

```bash
# Full suite per-package wall, CPU, RSS, and 20 slowest tests report:
HTO=0 gotester report ./...
# Report written to testreport/report.md
```

Or json-stream timing analysis via script:

```bash
GOWORK=off GOTOOLCHAIN=go1.26.0 go test -json -count=1 ./... > /tmp/cati-tests.json
go run scripts/test_timings.go /tmp/cati-tests.json
```

The 2026-10-23 baseline (`gotester report`) measured 82.7s total wall time across
28 packages (118.1s CPU, peak RSS 493 MiB). The run is dominated by `cmd` (41.9s,
493 MiB RSS) and `internal/audio` (27.1s, where `TestPlayAudio` took 25.46s).
Cataloging and modularization roadmaps are tracked in issue 111 and issue 112.
Earlier 2026-10-17 baseline took 43.59s in `cmd`: the all-mode smart metadata
sweep took 8.02s, the SSIM sample sweep 7.14s, and the sleeping benchmark test
3.96s. The latter now advances a fake clock and verifies one render per mode
and path. Compare timings with warm build caches and similar host load.
After extraction and test-tier changes, the full fast/reference `cmd` runs
measured 28.61s/29.93s. The final short run measured 4.64s in `cmd` and 9.25s
for the whole `make test-short` invocation with warm build caches. Earlier
invocations with recompilation exceeded ten seconds; host load also matters.

CLI geometry tests invoke this checkout's player command directly for preview
cases, so an older `catiplay` on PATH cannot affect their results. Width uses
`--width`/`-W` and height uses `--height`/`-H`. The real mediabrowse video test
checks play/pause/resume before ticking, then waits up to ten seconds for EOF:
the decoder can drain the entire short clip in a single tick.

Real audio-probe and video tests use `internal/testutil.RequireMedia` to skip
missing fixtures, Git LFS pointer files, or unavailable `ffprobe`/`ffmpeg`
commands. Run `git lfs pull` and install FFmpeg to exercise those tests. Each
audio asset has its own subtest, so a pointer does not hide other available
media. Read errors and decoder failures on present media still fail the test.

**Media test tiers and bounding policy**:
Long-running media or audio playback tests (e.g. streaming real audio over multiple
seconds) must be restricted to integration test suites (`-tags=integration`) and
must not run as part of the default `make test` or `make test-short` tiers.
Standard unit suites must only test typical playback scenarios against sub-second
representative fixtures or mock clocks.

Additional suites are opt-in:

- `make test-player` — tests guarded by the `catiplay` build tag.
- `make test-browser` — tests guarded by the `catibrowse` build tag.
- `make test-integration` — terminal/TUI tests guarded by the `integration`
  build tag; this builds the binaries first. The suite shares one temporary
  binary directory and sends each PTY key after a completed hint row. It drains
  the output channel after process exit, with no wall-clock sleeps; external
  process deadlines scale with `testutil.RaceFactor`. Mode-cycle assertions
  read the embedded spec. Exact gradient convergence covers 1x2/2x2 cells;
  larger geometries have solid-colour and PNG golden coverage.
- `make test-all` — runs the default, player, browser, and integration suites.

Other checks:

- `make lint` — gofmt check, vet, and `harnez lint --check` on tracked Go,
  shell, Markdown and Makefile files. CI and Docker pin the Harnez version
  and build that tool with Go 1.26.5; renderer checks remain on Go 1.26.0.
- `make ci-test` — lint, build, both fastpath test modes, and the
  race-enabled suite, including `catiplay`, `catibrowse` and `integration`
  tests. CI uses Go 1.26.0, FFmpeg and librsvg, and checks out LFS fixtures.
- `make container-test` — runs the same checks in the pinned Dockerfile image
  using podman when available, otherwise docker. The container filesystem is
  read-only apart from `/tmp` and the host output directory
  (`CONTAINER_OUTPUT`, default `.container-test-output/`).
  The host supplies the tracked lint-file manifest in that directory because
  the image omits `.git`; direct image runs can supply `CATI_LINT_FILES` too.
  A runner that restricts rootless namespace setup can use a local Podman API
  service via `CONTAINER_ENGINE_ARGS="--remote --url unix:///path/to/socket"`.
  Keep its socket in a runner-visible directory (for example the ignored
  output directory) and stop the temporary service afterwards. Quota enforcement
  stays enabled around the test command.
- `go test -bench=.` — runs performance benchmarks; benchmarks are not part of
  the normal test run.
- `make preflight` — builds the binaries, runs `go vet`, and checks that the
  demo renderer reports no errors.
- `make docker-test` — runs the tests with the pinned Go toolchain, useful when
  validating PNG golden-image changes.

Container tests set `CATI_CONTAINER_TEST=1`. Audio playback skips because the
container has no host audio device. CI includes tagged PTY integration tests;
podman/docker provide `/dev/ptmx` and a private `/dev/pts` mount. The ordinary
`make test` suite remains untagged. No tests require a GPU or network access
(toolchain/module downloads happen during setup).
Media probing and video decoding still run when FFmpeg and real LFS assets are
available. `internal/testutil.RequireMedia` skips missing fixtures, LFS pointer
files, or missing FFmpeg tools, while decoder errors on present media fail.

For a real asset speed comparison, run `cati --bench <mediafile>`. Each mode
is measured with fast paths on and off (`core.Fastpath`) and the row shows the
speed-up. The row also compares the first image render from each path and marks
output parity as `ok` or `DIFF`, with the first differing byte and differing
byte count. Video output is hashed per frame on the fast pass and compared with
the corresponding simple-pass frame; a differing-frame count appears on the
row. Any mismatch is summarized and makes the command exit non-zero. Image
renders repeat within `--bench-budget` (default 3s total,
split across modes and paths); every mode renders at least once, so slow modes
at large sizes can extend the run. `--mode` limits the run to one mode;
otherwise the spec's `experimental` modes (`all+`, `z`, `z+`) are skipped. Videos decode
the complete first video stream, without playback pacing or audio. Width and
height default to the terminal size, or 80x24 when stdout is not a terminal.

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

## Interactive aspect demo

Run `scripts/demo_aspect.sh` after `make install`. It resolves assets relative
to its own location and calls `cati` from PATH. Its 24 cases cover `default`,
`aligned` and `pixel` on Doom at widths 12/54/107/110/160 and the vacation photo
at widths 12/54/110. Images render before captions and commands, with dashed
separators; Up/Down or `k`/`j` navigate, Enter advances, and `q` or EOF exits.
Keys act immediately without Enter. Navigation remains at either boundary,
allowing backward inspection from the final case.

Verification from the aspect session (2026-10-03): all 24 real renders passed;
CSI (`ESC [ A/B`) and SS3 (`ESC O A/B`) navigation, first/last boundaries,
backward navigation, quit/EOF and shell syntax were checked. A child PTY also
verified `1 → 2 → 1 → quit` with actual terminal key capture. Direct host PTY
attempts were stopped and killed by Harnez process-group handling; that was a
harness failure, not evidence of a demo failure. When this occurs, record the
failure and verify through an independently controlled child PTY with a bounded
wait and cleanup. Do not claim terminal verification based only on piped input.

Geometry coverage lives in `internal/viewgeom/planner_test.go`, CLI regression
coverage in `cmd/root_test.go`, and sampling/annotated PNG golden coverage in
`cmd/pixel_aspect_test.go`. The aspect implementation passed `HTO=0 make test`
in both fast and reference modes; existing goldens remained unchanged. Later
script-only changes used real renders, navigation checks and `make preflight`.
These checks do not replace visual review: report gaps with the caption's case
number and exact printed command. See [SparklinePixelArt.md](SparklinePixelArt.md)
for the policy, geometry examples and remaining limitations.

The opt-in static renderer search can be exercised with, for example:

```bash
GOWORK=off GOTOOLCHAIN=go1.26.0 go run ./cmd/cati --width 40 --smart image.png
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
(`0` = simple reference paths; default on). `cati`, `catiplay`, and `catibrowse`
accept `--fastpath=false` to disable these paths or `--fastpath=true` (also
`--fastpath`) to enable them, overriding the environment for that invocation.
An omitted flag preserves the environment default. Forwarded playback/browser
commands inherit the selection. `cati --bench` still measures both paths.
The simple path is the reference:
change and reason about it first, then make the fast path follow. Fast paths
must be output-identical; each split has a parity test that toggles
`core.Fastpath`. Check the whole suite on the simple paths with:

```bash
CATI_FASTPATH=0 GOWORK=off GOTOOLCHAIN=go1.26.0 go test -tags=fastpath0 ./...
```

Known exception: `metrics.extractLumaFlat` fast paths drift slightly
(issue 055).

Fast-path payoff, measured with `cati --bench` on one photo at 120x50:
half 3.92x (2026-09-24); earlier measurements (2026-09-23) were quad 2.07x,
bars+ 1.43x, bars 1.34x, and other modes about 1x.
Rerun `--bench` before and after fast-path work to confirm a real gain
rather than relying on micro-benchmarks alone.
