# 122 — Jules sweep 2026-11: multi-wave report-only repository review

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Chore
**Related**: [112](112-modularize-cmd-package-extract-browser-player-and-modes-into-internal-sub-packages.md), [120](120-epic-reconcile-unused-go-declarations-across-cli-and-render-packages.md), [111](111-test-suite-catalog-and-roadmap-lighter-quieter-timed-tests.md)

---

/goal Run the waves below with Jules (report-only PRs), check each report's evidence, collect kept reports under `reports/jules/`, and hand them to one synthesis agent that files tickets or quick fixes. Stop and report when the owner has not approved a wave, Jules is unavailable (cutoff 2026-11-11), or GitHub cannot be synced; on repeated failed runs, record why in this ticket and stop.


Deadline: Jules stops taking work on 2026-11-11. Each wave has at most 10 sessions (the account cap).
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
