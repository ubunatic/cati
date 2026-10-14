# 096 — Skip media tests when Git LFS assets are pointer files

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Infrastructure
**Related**: `internal/audio/play_test.go`, `examples/mediabrowse/main_test.go`

---

## 1. Problem & Motivation
In a clone without `git lfs pull` (cloud agents, CI without LFS), `go test ./...`
fails instead of skipping, hiding real failures behind environmental noise.

## 2. Technical Specification / Findings
- `assets/*.mp4|webm` and `testdata/baby-60p-nn.webm` are LFS pointers
  (`file` reports "ASCII text").
- `TestProbeAudio` (`internal/audio/play_test.go:50`) fails with
  `ffprobe … exit status 1` for every asset.
- `TestRealVideoPlaybackIntegration` (`examples/mediabrowse/main_test.go:161`)
  fails with `expected video to enter playing state`.

## 3. Implementation & Verification Plan
**Goal**: `go test ./...` is green in a pointer-only clone and still exercises
media when LFS objects are present.

- Add one shared test helper (e.g. `internal/testutil.RequireLFS(t, path)`)
  that `t.Skip`s when the file starts with
  `version https://git-lfs.github.com/spec/v1`; also skip when `ffprobe`/`ffmpeg`
  is missing from `PATH`.
- Use it in both tests and any other test that opens `assets/` media.
- Verify: `go test ./...` passes in a pointer-only clone (skips logged) and
  after `git lfs pull` (tests run).
