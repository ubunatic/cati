---
title: "Study: Profiling Test Slowness, Cataloging Bottlenecks, and the Overcrowded Monolith"
date: 2026-10-23
author: Cati Team
status: complete
---

# Profiling Test Slowness, Cataloging Bottlenecks, and the Overcrowded Monolith

On 2026-10-23, prompted by machine contention and test slowdowns across sibling repositories (`loom` and `harnez`), we evaluated `cati`'s test suite performance using the new `gotester` tool, cataloged its bottlenecks, filed issues for actionable remediation, and released `v0.2.13`.

---

## 1. Header & Context
- **Date**: 2026-10-23 / 2026-10-24
- **Scope**: Cross-repo investigation (`../loom`, `../harnez`), test cataloging with `gotester`, issue creation for slowness and `cmd/` modularization, test suite verification under Quota-1, and CLI release.
- **Starting State**: Clean worktree on `main` following issue 110 completion (commit `b2c1895`).
- **Intended Goals**: Identify and catalog slow tests in `cati`, determine whether architectural patterns (e.g. monolithic command packages) are degrading test velocity and caching, file corresponding issues, ensure test greenness, and execute a routine release.

---

## 2. Executive Summary
Using `gotester report ./...`, we measured the exact performance profile of `cati`'s test suite across its 28 packages. The baseline run revealed that two packages consume 83% of the entire suite duration: `cmd` (41.9s wall, 93.3s CPU, 493 MiB RSS) and `internal/audio` (27.1s wall, driven almost entirely by `TestPlayAudio` running for 25.46s).

Furthermore, analyzing `cmd/` revealed a 56-file, ~17,750-line monolith where static CLI rendering, interactive media player, file browsing, SSIM benchmarks, and visual mode sweeps are conflated into a single package. Because Go caches test results per package, any change to a single CLI sub-feature invalidates and re-runs the full 42s suite.

We documented and tracked these findings in **Issue 111** (test suite catalog & roadmap) and **Issue 112** (cmd modularization into `internal/climodes`, `internal/clibrowse`, and `internal/cliplay`). We verified that all tests remain green under Quota-1 (`make test-q1`), and published release **`v0.2.13`**.

---

## 3. What Worked Well
- **Reproducible Performance Metrics**: Rather than guessing why developer turns were slowing down, `gotester report` provided an immediate, objective breakdown of package wall time, CPU time, RSS, and the 20 slowest individual tests.
- **Immediate Root Cause Pinpointing**: `internal/audio/TestPlayAudio` was immediately exposed as a 25.5s idle wait on audio playback streams, which can be tiered or stubbed.
- **Cross-Repo Alignment**: Directly inspecting `../loom/issues/377`, `../loom/issues/378`, `../harnez/issues/881`, and `../harnez/issues/877` enabled us to adopt proven roadmaps and package-splitting patterns instead of inventing bespoke approaches.
- **Seamless Release Recovery**: When `harnez release` hit a transient Forgejo HTTP 500 error during asset upload, running `harnez release --continue` cleanly reused the already-built and minisign-signed `dist/` artifacts without redundant recompilation.

---

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)
- **Initial Timeout Kill (`HTO=0` Oversight)**:
  - *Failure*: The first run of `gotester report` was executed without `HTO=0` and was terminated by Harnez's default 60-second execution watchdog (`exit status 137`) while testing `internal/audio`.
  - *Fix*: Re-ran with `HTO=0 gotester report` as a background task, allowing the full 82.7s measurement pass to complete cleanly.
- **Untracked Artifact Spill**:
  - *Near-Miss*: Running `gotester report` produced a local `testreport/report.md` directory that was not in `.gitignore`. A subsequent `git status` revealed it as untracked.
  - *Fix*: Added `/testreport/` to `.gitignore` before filing the issues, ensuring temporary test run artifacts never pollute git status.
- **Transient Forgejo 500 on Release Upload**:
  - *Failure*: During the initial `harnez release`, Codeberg/Forgejo returned an unexpected HTTP 500 Internal Server Error when creating/uploading release assets via `fj`.
  - *Fix*: The release engine correctly stopped cleanly after committing and tagging. Running `harnez release --continue` resumed the release immediately, reusing the signed `dist/` bundle and completing the upload successfully.

---

## 5. Quality & Invariants Audit

| Dimension | Assessment | Notes |
| :--- | :--- | :--- |
| **Architecture & Separation** | Needs Modularization | `cmd/` has grown into a monolithic 17,750-line package; Issue 112 establishes the plan to split it into `internal/` packages. |
| **Idempotency** | Verified | Running `harnez release --continue` and re-running `harnez index -d .` produced zero spurious changes. |
| **Backward Compatibility** | Preserved | Releases and issue definitions do not break existing CLI interfaces or library contracts. |
| **Test Verification** | 100% Green | Verified via `HTO=0 make test-q1` (both `CATI_FASTPATH=1` and `CATI_FASTPATH=0` passes). |

---

## 6. Efficiency & Velocity Assessment
- **Time to Actionable Roadmap**: In under 15 minutes, baseline performance was quantified, comparisons to sibling repos were completed, and two structured issues (111 and 112) were drafted, indexed, and committed.
- **Release Turnaround**: Release `v0.2.13` completed in ~2 minutes despite cross-compilation across 4 architectures and handling a transient server error via `--continue`.

---

## 7. Key Learnings & Evergreen Upstream
1. **Always Set `HTO=0` for Profiling & Benchmarking**: Any test suite or reporting pass on non-trivial codebases will exceed 60 seconds.
2. **Prevent Monolithic `cmd/` Accumulation**: Command packages should only register CLI flags and route execution. Features (even CLI-specific ones like viewers or browser loops) should start in `internal/<subpackage>` from day one to protect package test caching.
3. **Guard Audio & Media Tests**: Real-time media playback in unit tests will silently balloon test times; media tests must use short fixtures or mock clock providers.

---

## 8. File & Diff Summary
- `issues/111-test-suite-catalog-and-roadmap-lighter-quieter-timed-tests.md`: Created test catalog issue.
- `issues/112-modularize-cmd-package-extract-browser-player-and-modes-into-internal-sub-packages.md`: Created modularization issue.
- `.gitignore`: Added `/testreport/`.
- `issues/README.md`, `docs/README.md`: Updated issue tracker and documentation index.
- `version.go`, `spec/version.yaml`: Bumped version to `v0.2.13`.
- Git Commits:
  - `c7269cb`: `docs(issues): file issue 111 test suite catalog and roadmap using gotester`
  - `64e1653`: `docs(issues): file issue 112 modularize cmd package into internal sub-packages`
  - `3c99921`: `chore(release): bump version to v0.2.13`
