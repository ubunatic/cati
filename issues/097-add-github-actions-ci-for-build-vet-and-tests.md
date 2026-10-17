# 097 — Add GitHub Actions CI for build, vet and tests

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Infrastructure
**Related**: [096](096-skip-media-tests-when-git-lfs-assets-are-pointer-files.md), [Testing](../docs/Testing.md), `Makefile`, `Dockerfile`, PR #56 (merged with zero checks)

---

## 1. Problem & Motivation
The repo has no `.github/` directory. PRs, including agent PRs (Jules, Claude),
merge with no automated checks. Fast/simple path parity, goldens, and `go vet`
are only verified when a contributor runs `make test` locally. Several recent
perf PRs changed render hot paths; a broken parity or golden would land
unnoticed.

## 2. Technical Specification / Findings
- **Go version:** `go.mod` says `go 1.26.0`; `Makefile` pins
  `GO_TEST_ENV := GOWORK=off GOTOOLCHAIN=go1.26.0`. CI must use exactly
  1.26.0 (`actions/setup-go` with `go-version-file: go.mod`), because stdlib
  decoders can shift golden pixels between patch releases.
- **Drift:** `Dockerfile` still pins `golang:1.25.0-bookworm`, and its comment
  says `go.mod` requires 1.25.0. Both are stale. Bump it to `golang:1.26.0-bookworm`
  in the same PR so `make docker-test` and CI agree.
- **Test targets (`Makefile`):**
  - `make test` = `go vet ./...` + `go test ./...` + `CATI_FASTPATH=0 go test -tags=fastpath0 ./...`.
    Both passes are needed: `core.Fastpath` is read at init.
  - `make build` builds `cati`, `catiplay`, `catibrowse`.
  - `make preflight` also runs `scripts/demo_widths.go` against the built binary.
  - Optional tags: `catiplay`, `catibrowse`, `integration` (PTY-based TUI tests).
- **Media deps:** `cmd/play.go`, `cmd/media_meta.go`, `internal/audio`, and
  `v1/halfblock/video.go` shell out to `ffmpeg`/`ffprobe`. The `Dockerfile` installs `ffmpeg`.
- **LFS:** `*.mp4`, `*.webm`, and `docs/feedback/*.png` are Git LFS (`.gitattributes`).
  Without `lfs: true` on checkout, `TestProbeAudio` and
  `TestRealVideoPlaybackIntegration` fail (see 096). LFS bandwidth on GitHub
  is metered, so prefer skipping (096) over pulling LFS on every run.
- **License:** `REUSE.toml` exists; `make reuse` runs `reuse lint`.

## 3. Implementation & Verification Plan
**Goal**: Every push to `main` and every PR runs build, vet, and both
fastpath test passes on the pinned toolchain. A red check blocks merge.

Add `.github/workflows/ci.yml` (+ `REUSE.toml` annotation for it):

```yaml
name: ci
on:
  push: { branches: [main] }
  pull_request:
permissions: { contents: read }
concurrency: { group: ci-${{ github.ref }}, cancel-in-progress: true }
jobs:
  test:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@v4          # no lfs: media tests skip via 096
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod, cache: true }
      - run: sudo apt-get update && sudo apt-get install -y --no-install-recommends ffmpeg
      - run: make build
      - run: make test                     # vet + fastpath=1 + fastpath=0
      - run: go run scripts/demo_widths.go -bin ./cati   # preflight render check
      - run: git diff --exit-code          # tests must not rewrite goldens/files
  reuse:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: fsfe/reuse-action@v5
```

Steps:
1. Land 096 first (or in the same PR). Otherwise CI starts red on the LFS
   pointer failures. Do not add `lfs: true` as a workaround.
2. Pin actions to the major tags above. Dependabot for `github-actions` is
   optional (`.github/dependabot.yml`, weekly).
3. Bump the `Dockerfile` base image and comment to Go 1.26.0.
4. Keep `catiplay`, `catibrowse`, and `integration` tags out of the required job.
   Optionally add a non-required `test-all` job (`make test-all`) with
   `continue-on-error: true` until those suites are stable headless.
5. Document CI in `docs/Testing.md`: which targets run, the toolchain pin, and
   the LFS skip policy. Link it from `docs/README.md` if needed.
6. After merge, ask the repo owner to make `ci / test` and `ci / reuse`
   required status checks on `main`. That is a manual GitHub setting; note it in
   the PR body.

Verification:
- Open the PR and confirm both jobs are green on it. Paste the run links in
  the PR.
- Prove that CI fails on a real regression: in a throwaway commit on the PR
  branch, flip one byte in a golden under `testdata/`. Confirm `make test`
  fails in CI. Then revert. Never push the broken commit to `main`.
- `make docker-test` passes locally with the bumped image.
