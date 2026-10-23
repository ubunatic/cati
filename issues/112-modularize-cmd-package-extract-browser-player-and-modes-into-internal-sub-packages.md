# 112 — modularize cmd package: extract browser, player, and modes into internal sub-packages

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Refactor
**Related**: 024 (split binaries), 111 (test catalog roadmap), loom 377, loom 378, harnez 877, harnez 881

---

/goal Modularize the monolithic `cmd` package by extracting browser, interactive player, and modes command logic and tests into `internal/` sub-packages, leaving thin Cobra commands in `cmd/`, so touching one feature area does not bust Go's test cache for the entire 18,000-line `cmd` package; or stop and report when blocked on a user decision.

## 1. Problem & Motivation
Following the pattern identified in loom (issues 377, 378) and harnez (issues 877, 881), `cmd` has become an overcrowded monolith:
- 56 Go files and ~17,750 lines of code.
- Go caches test results per package. Because static CLI rendering, golden tests, interactive browser, media player, SSIM benchmarks, and modes sweeps all share `package cmd`, any change to one sub-feature forces the entire 42-second, 493 MiB RSS test suite to re-run.
- Issue 024 separated the binary entrypoints (`cmd/cati`, `cmd/catiplay`, `cmd/catibrowse`), but noted that a follow-up step was needed to move implementation dependencies behind library/internal packages.

## 2. Technical Specification / Findings

### Candidate sub-packages for extraction:
1. **Browser (`internal/clibrowse` or expanding `internal/browser`)**:
   - Files: `cmd/browser.go` (1,346 lines), `cmd/browser_config.go` (579 lines), `cmd/browser_menu.go` (377 lines), `cmd/thumbqueue.go` (185 lines), and tests `cmd/browser_test.go` (232 lines).
   - ~2,700 lines total.
2. **Modes command & inspection (`internal/climodes`)**:
   - Files: `cmd/modes.go` (747 lines), `cmd/modes_test.go` (691 lines), `cmd/modes_cards.go` (335 lines), `cmd/modes_probe_test.go` (274 lines), `cmd/modes_benchmark.go` (247 lines), `cmd/modes_probe.go` (234 lines), `cmd/modes_glyphs_test.go` (232 lines), `cmd/modes_filter.go` (211 lines), `cmd/modes_metrics.go` (205 lines), `cmd/modes_glyphs.go` (140 lines), `cmd/modes_presets.go` (116 lines).
   - ~3,400 lines total.
   - Dominated by heavy sweeps (`TestModesCommandInfoAllModesAndSmartMetadataOnce` 14.0s, `TestModesFilterFlags` 6.2s, `TestModesCommandDemo` 5.6s).
3. **Player & interactive viewer (`internal/cliplay`)**:
   - Files: `cmd/play.go` (387 lines), `cmd/interactive.go` (679 lines), `cmd/interactive_render.go` (385 lines), `cmd/interactive_modes.go` (251 lines), `cmd/interactive_zoom.go` (183 lines), `cmd/play_test.go` (123 lines), `cmd/interactive_test.go` (878 lines).
   - ~2,900 lines total.
4. **Benchmarks (`internal/clibench` or test-only helper)**:
   - `cmd/media_benchmark.go`, `cmd/media_benchmark_test.go`, `cmd/ssim_bench_test.go`.

### Scope & Constraints:
- Pure refactoring / file moves with minimal behavioural changes.
- `cmd/` keeps flag definitions, Cobra command registrations, help strings, and exit code routing.
- Packages placed under `internal/` to avoid expanding the public library API contract.
- Verified incrementally: one feature extraction per milestone/commit with cache-invalidation checks.

## 3. Implementation & Verification Plan
1. **Milestone 1 — Extract Modes Command (`internal/climodes`)**:
   - Move modes CLI logic and modes tests out of `cmd/`.
   - Leave thin `modesCmd` in `cmd/modes.go`.
   - Verify: `go test ./internal/climodes ./cmd`, build and vet pass.
2. **Milestone 2 — Extract Browser (`internal/clibrowse`)**:
   - Move file browser navigation, thumbnail queue, and browser tests out of `cmd/`.
   - Leave thin `browseCmd` in `cmd/browser.go`.
   - Verify: `go test ./internal/clibrowse ./cmd`, build and vet pass.
3. **Milestone 3 — Extract Interactive Player (`internal/cliplay`)**:
   - Move player loop, zoom, and interactive viewer tests out of `cmd/`.
   - Leave thin `playCmd` in `cmd/play.go`.
   - Verify: `go test ./internal/cliplay ./cmd`, build and vet pass.
4. **Cache & Performance Verification**:
   - Run `gotester report` to measure new per-package timing and RSS profile.
   - Run cache hit probe: touch one file in `internal/climodes` and confirm that `go test ./...` only re-runs `internal/climodes` and `cmd`, leaving the rest cached.
