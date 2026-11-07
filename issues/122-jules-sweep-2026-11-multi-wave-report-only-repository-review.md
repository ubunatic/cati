# 122 — Jules sweep 2026-11: multi-wave report-only repository review

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Chore
**Related**: [112](112-modularize-cmd-package-extract-browser-player-and-modes-into-internal-sub-packages.md), [120](120-epic-reconcile-unused-go-declarations-across-cli-and-render-packages.md), [111](111-test-suite-catalog-and-roadmap-lighter-quieter-timed-tests.md)

---

/goal Run the waves below with Jules (report-only PRs), check each report's evidence, collect kept reports under `reports/jules/`, and hand them to one synthesis agent that files tickets or quick fixes. Stop and report when the owner has not approved a wave, Jules is unavailable (cutoff 2026-11-11), or GitHub cannot be synced; on repeated failed runs, record why in this ticket and stop.


Deadline: Jules stops taking work on 2026-11-11. Each wave has at most 10 sessions (the account cap).
lmcoder runs on a separate Jules account (confirmed 2026-11-07), so cati has its own cap of about 10 open sessions (finished sessions may count briefly: starts right after a completion still hit HTTP 400) and about 100 jobs/day.
Each run takes 3–40 min, so a wave lasts about 45 min.

## Sync gate (before every wave)
- Compare `git ls-remote origin` with `git ls-remote github` for main. Today GitHub is 10 commits behind (73156cd vs 1ad3255), and it can be fast-forwarded.
- The first `harnez agent start` from `main` fast-forwards GitHub. Start one session first, check that the two remotes match, then start the other 9.
- After each wave, commit the tracking table and the collected reports to Codeberg; the next wave's first start syncs GitHub again.

## Output contract (all runs)
- Report only: each run adds `reports/jules/<slug>.md` in one PR, no code changes. That means no merge conflicts.
- The host checks evidence (skill §4), copies the kept reports to main and closes the PRs.
- After the last wave, one synthesis agent reads every report and decides on tickets, quick fixes and drops.

## Role pre-prompt (prefixed to every run)
You are a senior Go reviewer auditing `cati`, a terminal image/video renderer
(halfblock/quad/sextant/sparkline glyph modes, a spec-driven TUI under `spec/`).
You work like this: read `CLAUDE.md`, `docs/README.md` and `issues/README.md` first; trust code
over docs; open every line you cite and quote it; mark anything unconfirmed `not verified`.
You do: write one Markdown report with checked `path:line` evidence, ranked findings
(severity, effort S/M/L, smallest next step) and deliver it via `submit` as a PR.
You do not: change code, tests, docs or tickets; ask questions or wait for plan approval;
propose work already covered by an open ticket (cite the ticket instead); invent references;
re-litigate the spec rules in CLAUDE.md (spec is the source of truth; no Go fallbacks).

## Wave 1 — Audits of active tickets that may be stale (10)
Ticket selection includes `Backlog`-status tickets, which do not appear in `issues/README.md` (see `issues/BACKLOG.md`).
On 2026-11-07 there are none: BACKLOG.md is empty and no file under `issues/` or `issues/archive/` has a Backlog status.
Re-check before each audit wave, and add any Backlog ticket to the next audit wave, oldest and bug tickets first.
Audit prompt (§6) for each:
004 spec loose ends · 007 ffprobe on hover · 008 spaghetti/viewgeom · 016 worker copies ·
018 spark allocations · 024 split binaries · 057 experimental modes · 059 cmd.Wait hang ·
067 GOTOOLCHAIN pin · 112 modularize cmd

## Wave 2 — Area reviews: structure and modularization (10)
Ideas prompt (§9), one area per run:
1. `cmd/browser*.go`: extraction into internal/browser
2. `cmd/interactive*.go` + `viewer_core.go`: viewer/player boundary
3. `cmd/modes*.go`: modes CLI → internal/modes
4. `cmd/render_pipeline.go`, `smart_render.go`, `render_output.go`: render pipeline
5. `cmd/ssim.go`, `metrics.go`, `internal/metrics`: duplicated quality metrics
6. `v1/core` + `v1/halfblock`: API surface, dead code
7. `v1/quadblock` + `v1/sextant`: shared logic, duplication
8. `v1/sparkline`: complexity hotspots
9. `spec/` loaders + `spec_test.go`: integrity test gaps vs docs/Spec.md §6
10. `internal/*` small packages: cohesion, merge/split candidates

## Wave 3 — Cross-cutting categories (10)
Ideas prompt, one category across the whole repo:
1. Dead code and unused exports (complements epic 120)
2. Error handling: swallowed errors, panics, error wrapping
3. Concurrency: goroutine leaks, channel shutdown, ffmpeg/ffprobe process lifecycle
4. Allocation and performance hotspots in render loops
5. Test quality: assertion rigor, sleeps, slow tests, golden coverage (vs 111)
6. Hardcoded values that belong in `spec/` (spec rule violations)
7. API naming and consistency across render modes
8. Makefile, build and release tooling hygiene
9. Docs drift: docs/*.md claims vs code
10. Security and robustness: input files, path handling, terminal escape injection

## Wave 4 (optional) — Full-repo reviews and follow-ups (≤10)
- 2 whole-repo architecture reviews (top-10 refactors, one each for `cmd`+`internal` and `v1`)
- Up to 8 re-runs or audits for the most important leads from waves 1–3

## Tracking
One tracking issue ("Jules sweep 2026-11") with a `## Jules runs` table
(kind, session, Jules ID, PR, state), filed via /issue and updated as each wave goes.

## Jules runs

| Wave | Kind | Session | Jules ID | PR | State |
|---|---|---|---|---|---|
| 1 | audit | jules-audit-004 | | | failed: clone (LFS) |
| 1 | audit | jules-audit-007 | | | failed: clone (LFS) |
| 1 | audit | jules-audit-008 | | | failed: clone (LFS) |
| 1 | audit | jules-audit-016 | | | failed: clone (LFS) |
| 1 | audit | jules-audit-018 | | | failed: clone (LFS) |
| 1 | audit | jules-audit-024 | | | failed: clone (LFS) |
| 1 | audit | jules-audit-057 | | | failed: clone (LFS) |
| 1 | audit | jules-audit-059 | | | failed: clone (LFS) |
| 1 | audit | jules-audit-067 | | | failed: clone (LFS) |
| 1 | audit | jules-audit-112 | | | failed: clone (LFS) |
| 1 | audit | jules-audit-004-r2 | 1796877461217116899 | | failed: clone (LFS) |
| 1 | audit | jules-audit-004-r3 (jules-base) | 9972900510142540389 | | failed: clone (LFS) |
| 1 | audit | jules-audit-004-r4 (HARNEZ_JULES_BRANCH=jules-base) | 13018607114825096742 | #63 (closed) | done: partly done, 8/10 refs, report kept |
| 1 | audit | jules-audit-007-b | 2979654456957508536 | #64 (closed) | discarded: scope (added generate_report.py); host verified still valid (media_meta.go:136 probes every file) |
| 1 | audit | jules-audit-059-b | 9393155128620679498 | #65 (closed) | done: still valid, 3/3 refs, report kept |
| 1 | audit | jules-audit-018-b | 8993073000837387833 | #66 (closed) | done: partly done, 5/5 refs, benches reproduced, report kept |
| 1 | audit | jules-audit-057-b | 5636256323960672811 | #67 (closed) | done: partly done, 8/8 refs, report kept |
| 1 | audit | jules-audit-005-e | 2332396799955557010 | #68 (closed) | done: still valid, 4/5 refs, report kept |
| 1 | audit | jules-audit-112-c | 10484382285619105045 | #71 (closed) | done: still valid, 5/6 refs, report kept |
| 1 | audit | jules-audit-016-b | 13291937613804283066 | #70 (closed) | done: partly done, 8/8 refs, report kept |
| 1 | audit | jules-audit-067-b | 17471314065596726627 | #69 (closed) | done: already done, 5/5 refs; ticket 067 closed (ccae5a4) |
| 1 | audit | jules-audit-008-b | | | pending |
| 1 | audit | jules-audit-024-b | | | pending |

## Blocker (2026-11-07)
All 12 runs failed with "Jules encountered an error when cloning the repo." In the Jules VM log,
`git clone ... https://github.com/ubunatic/cati -b main` succeeds, but the LFS smudge fails:
`batch response: Unable to parse HTTP response for POST http://git@192.168.0.1:8080/ubunatic/cati.git/info/lfs/objects/batch: unexpected EOF`
(first file `assets/baby-180p-nn.mp4`; 14 files are LFS-tracked).
GitHub itself is fine: a fresh local clone runs `git lfs pull` from GitHub without error. The fault is Jules's git proxy.
Workaround tried: a GitHub-only branch `jules-base` with `.lfsconfig` `fetchexclude = *`. Jules ignored it, because it always clones `-b main`.
The branch was deleted.
Owner decision needed to unblock: (a) commit `.lfsconfig` with `fetchexclude=*` on main (fresh clones and CI would then get LFS pointer files and need `git lfs pull` for media); or (b) make harnez/Jules start from a non-main branch (harnez's `jules.sync_branch` or Jules's `startingBranch`; unverified whether that is configurable); or (c) drop the sweep.

**Fix (2026-11-07):** start runs with `HARNEZ_JULES_BRANCH=jules-base` (harnez `internal/subagent/jules.go:28`).
`jules-base` is a GitHub-only branch: main plus `.lfsconfig` (`fetchexclude = *`). It is never merged.
Before each wave, rebuild it from the current main. Run jules-audit-004-r4 completed in 3m44s with PR #63.
