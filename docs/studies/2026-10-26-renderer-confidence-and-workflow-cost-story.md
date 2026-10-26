---
kind: story
title: "Renderer Confidence and the Cost of the Workflow"
date: 2026-10-26
baseline: 8dcf63b
head: 0f866fd
status: review-blocked
review: "Not performed; host agent-role guard rejected launch (issue 119)"
---

# Renderer Confidence and the Cost of the Workflow

## 1. Header & Context

This follows [Terminal Pixels, Measured and Made](2026-10-04-terminal-pixels-with-agents-story.md), dated October 4 and introduced in `8dcf63b`. No existing study has `kind: story`; that document is the newest overall project account by content. The later Jules performance note is a single-contributor study, not the baseline. Scope is the complete reachable git range `8dcf63b..0f866fd`, through October 26, before this story's own changes.

The previous account described unified geometry, spec ownership, optimized/reference parity, and ticketed agent work. It explicitly left geometry edge cases (#078) unresolved. This interval expanded glyph rendering, repaired correctness boundaries, improved verification and CI, and exposed the cost of keeping command logic and slow tests together. Dates in filenames are not reliable chronology: the Jules setup study says March 31 but entered this range in `3a6f95e` on October 26.

**Review:** Not performed. Writer: OpenAI Codex. The attempted dedicated read-only reviewer was `claude:sonnet:high` (Anthropic), selected from the installed lineup below Opus and above Haiku. Harnez rejected startup because it classifies this session as an advisor leaf forbidden from managing agents. No reviewer was created; no findings were independently checked or fixed. [Issue 119](../../issues/119-complete-second-vendor-review-of-october-26-project-story.md) tracks host review and finalization. This story remains review-blocked.

## 2. Executive Summary

Quality **improved**, supported by stronger visual fixtures, repaired alpha/bounds handling, structural ANSI checks, CI/container verification, and independent reference-to-golden tests. Agentic flow improved locally in the last two rendering sprints, but **did not improve uniformly** across the interval: repeated timeout mistakes, slow suites, duplicated issue numbers, and incomplete command separation consumed avoidable effort.

The interval contains **203 commits**, including **16 merge commits**. Of 187 non-merge commits, 107 touch only `docs/` and/or `issues/`; 80 touch something else. That second category includes code, specs, CI, assets and website work, not 80 independent features. Net changes outside `issues/` are **233 files, +13,417/-6,817 lines** (+6,600 net).

Counted conservatively as four user-visible capability groups: vector modes, octant rendering, glyph/mask inspection and reconstruction probes, and the new website presentation/content contract. Parameterized vectors and inspection options are included in their parent group. Engineering deliveries such as spec defaults and CI are listed separately rather than inflating that feature count.

### Deliveries against the previous baseline

| Area | Delivered evidence | Remaining qualification |
|---|---|---|
| Glyph rendering | Vector and octant modes (#083/#084); vector straight bars (#091); inspection/probes (#095) | vec2..vec8 landed (`d35aa86`), but #090 remains Open; vec3/vec4 tuning #096 and dot-only braille #097 remain Open |
| Correct pixels | Aspect formulas #087/#088; vector masks #089; octant mapping #093; custom-mask reconstruction #092; quad alpha/bounds #094; luma parity #055 | New modes initially needed repairs; delivery alone did not establish visual correctness |
| Specs | CLI defaults in `spec/cli.yaml` (`1ef498a`); glyph sets, compositions and shapes split (#098) | Existing ownership-policy ticket #079 remains Open |
| Verification | Vec/oct goldens #086; media prerequisites #100; integration fixes #103/#104; ANSI structure #107; lint #108 | Historical pass records are evidence, not a fresh full-suite run for this story |
| Developer workflow | Short/package tiers #105; CI/container #101; parallel CI #109; extraction #106 | #111 test roadmap and #112 larger command extraction remain Open |
| Rendering architecture | Standard production paths and private reference tests #113; direct reference goldens #114 | Removed `--fastpath`, `CATI_FASTPATH` and `core.Fastpath` are compatibility changes |
| Publication | Scratch-tree docs generation #082; source links #081; README support claims #080; website comparison sizes #110 | Camera/live epics #115–#117 are plans and ANSI mocks, not working camera features |

Tracker comparison uses final issue identities, including local 115 renamed to 118. There are **38 newly numbered tickets (081–118)**: 29 Closed and nine still Open (085, 090, 096, 097, 111, 112, 115, 116, 117). Three baseline tickets also moved to Closed: 034, 055, 080. Thus **32 tickets closed** in this endpoint comparison. Complete closure list: 034, 055, 080–084, 086–089, 091–095, 098–110, 113, 114, 118. This counts state changes, not every close command or milestone.

Tags added in the range are **v0.2.11, v0.2.12, v0.2.13, v0.2.14**. These are local git evidence; remote release-asset availability was not rechecked. The [profiling study](2026-10-23-profiling-test-slowness-and-cmd-modularization.md) records successful v0.2.13 publication after upload recovery. The previous story's #078 remains Open, as do broader geometry, smart-search, media-lifecycle and quality work in the active index.

## 3. What Worked Well

Private in-package references avoided exposing internal lookup tables or adding production switches solely for tests. [Issue 113's sprint](2026-10-24-two-agent-lean-sprint-issue-113-fastpath-standardization.md) records two developer turns with no host code intervention. [Issue 114's sprint](2026-10-24-two-agent-lean-sprint-issue-114-reference-golden-testing.md) then verified the reference implementations directly against representative goldens instead of relying exclusively on transitive parity.

Diagnostic artifacts made visual questions concrete. Glyph cards, reconstructed-pixel probes, directional goldens, and the documented octant visual approval turned ambiguous output into reproducible evidence. The [vector continuity study](2026-10-07-vector-glyph-continuity.md) measured why diagonals beat missing straight bars and rejected two unsuccessful mode experiments rather than presenting them as completed features.

Test tiers, media prerequisite checks and containers made verification more practical. Extraction into `internal/browser`, `internal/modes` and `internal/ansi` was useful incremental separation (#106), even though it did not complete #112. The final integration preserved both remote histories and the local Sscanf error check without source conflicts.

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)

| Failure or near-miss | How it was caught | Exact response and limit |
|---|---|---|
| New vector/octant modes had incorrect masks and geometry | Numeric glyph inspection, mapping audits, directional renders and owner visual approval | Corrected aspect formulas (#087/#088), vector masks (#089), octant mappings (#093), and later added vec/oct goldens (#086) |
| Optimized output diverged on alpha and nonzero bounds; custom reconstruction lost supplied masks | Terminal alpha demonstration, parity tests and reconstruction probes | `a691aa5` repaired quad parity and mask preservation; later reference work fixed sextant/sparkline NRGBA handling (#113) |
| `halfvec`/`sixvec` failed visually despite plausible geometry | HUD crop and neighboring-edge inspection | Dropped experiments; added missing eighth bars (`b412bc4`); neighbor-aware selection remains unimplemented, not claimed delivered |
| A reference test reused the production sextant solver | Issue 114 refinement and diff review | Replaced delegation with independent all-64-mask scoring in `d5f9fcb` |
| Full tests were not reliably healthy before cleanup | #099 records 23 existing failures; integration and container checks exposed further problems | Fixed obsolete CLI flags and playback checks (`5ce50a0`), missing-LFS/tool handling (`68b11ff`), root package mismatch (`c720887`), PTY expectations (`8078566`), and preview-loader lifecycle (`ddf181a`) |
| Tests consumed real playback time and invalidated a large command-package cache | October 23 gotester profile | Bounded audio to 250ms (`424e634`); #111/#112 retain remaining work; removing double suite execution in #113 addressed a separate source of cost |
| Host omitted `HTO=0` more than once | Profiling and #113 preflight were killed by the 60s watchdog | Reran with `HTO=0`; the existing rule was already available, so this was an execution lapse, not missing guidance |
| Generated docs/report artifacts dirtied the tree | Git-status checks and publication verification | Scratch-tree website generation (#082), ignored `/testreport/`, and immutable golden fixtures (`4b91911`) |
| Release upload failed with Forgejo HTTP 500 | Release command failure recorded in profiling study | `harnez release --continue` reused signed artifacts; no claim that retry eliminated the external fault |
| Independent remotes reused ticket numbers | Earlier 095/096/097 renumbering commits; this session's two-sided history comparison | Earlier tickets became 102/100/101; this session preserved GitHub camera epic 115 and renamed the closed local check to 118, linking its original fixing commit |

This session also checked an absent optional `.harnez/rules/Local.md` with `git show`, which returned an avoidable error. It was recorded through tool feedback; the merged rules were then read through `harnez read`, which handles that optional absence. Feedback recording happened after another tool action, later than the skill requests. During preflight the host also made several short output waits with no new output; that repeated the kind of low-value waiting already criticized by the older orchestration post-mortem. No data loss was observed, but neither behavior merits a smooth-flow claim.

The owner explicitly redirected the initial Codeberg-only pull to include GitHub. The prior account contains no normalized correction/retry counts, and this session does not provide transcripts for every intervening sprint. Counts of owner interventions or agent time saved would therefore be invented. No revert-labelled commit appears in the range; that does not establish that no experiments were abandoned.

## 5. Quality & Invariants Audit

| Dimension | Verdict | Evidence and boundary |
|---|---|---|
| Architecture & module separation | Improved, incomplete | #106 extracted cohesive helpers; #113 keeps reference code test-only. #112 still tracks substantial command-package extraction |
| Idempotency | Locally supported | #082 records clean scratch builds; #098 records byte-identical CLI glyph output and unchanged goldens. This session's preflight left no tracked build changes. No universal rerun proof |
| Backward compatibility | Changed | Removed runtime fastpath flag/environment switch/core symbol and retargeted vec aliases to vec6. These are intentional changes; calling compatibility an unconditional PASS would hide them |
| Test coverage & verification | Improved | #086/#107/#114 add different forms of checking; #101/#109 add CI/container paths. #113/#114 report full Quota-1 and preflight passes. Today's merge passed build, install, vet and demo-render preflight |
| Lint state | Historical enforcement, limited current observation | #108 records lint checks and CI wiring; current `go vet` passed. No fresh complete lint or full test suite was run for this documentation task |
| Visual correctness | Better bounded evidence | Independent representative goldens and numeric probes strengthen confidence; they do not cover every mode, font, terminal, alpha case or glyph combination |

The #113 study labels compatibility PASS despite removing interfaces; #114 calls representative golden cases complete coverage. This follow-up narrows both claims. Likewise, removing the second whole-suite invocation eliminates duplicated execution, but does not by itself prove an exact 50% wall-time saving under different cache states.

## 6. Efficiency & Velocity Assessment

Across roughly 22 calendar days, 203 commits average about nine commits/day, including merges and documentation churn. That is activity, not a throughput or productivity benchmark. The prior story counted 255 commits over six weeks; the windows and work mix differ, so comparing raw totals cannot establish acceleration.

Measured results retained with their original scope:

- #111 records audio package wall time falling **27.1s → 0.9s**, with `TestPlayAudio` **25.46s → 0.39s**, after a 250ms fixture. This is a concrete reduction in avoidable waiting.
- #105 records a **9.25s warm short tier**. The October 23 **82.7s gotester baseline** is a different run/configuration, not a paired before/after benchmark of that tier.
- #109 records CI about **8m30s → 6m** (run 39150205143), around 29% less elapsed time, not 29% less total CPU use.
- #113 reports a **55s Quota-1 pass** and approximately 11s short tests; #114 reports representative reference-golden package runs below one second each. These historical observations are not a fresh common-hardware benchmark.

Bounded two-agent development appears smoother at the end of the interval than the older polling/concurrency account, but review quota exhaustion forced host review in #114. Timeout repetition and current short wait calls show incomplete adoption of process rules. The defensible flow verdict is **mixed, with a better final architecture**, rather than a monotonic rise in agent efficiency. No complete token, cost, or active-human-time ledger supports a numeric savings claim.

## 7. Key Learnings & Evergreen Upstream

1. Keep naive references private and independent, and compare representative reference output directly with goldens. `docs/Testing.md` and `docs/GoPerformance.yaml` already capture the new architecture; no additional policy copy is needed here.
2. Prove the wrong glyph/pixel numerically before changing masks or goldens. Retain probes and failed experiments as research evidence. `docs/RenderingBugPlaybook.md` already supplies the rule.
3. Start substantive command features in `internal/<feature>` and bound real media work. `AGENTS.md` already contains both rules; #111/#112 are the execution work, not another rule-writing exercise.
4. Check both development remotes before integration and ticket allocation; preserve histories and identify renumbered fixes explicitly. `docs/Git.md` already describes independent development and cross-remote fetching. This session demonstrates why following it matters.
5. Separate measured outcomes from inferred workflow quality. Record representative coverage and intentional interface removal honestly instead of copying unconditional PASS verdicts from sprint reports.

### Follow-Up Tickets — extraction gate

**New ticket allocated: [119 — complete second-vendor review](../../issues/119-complete-second-vendor-review-of-october-26-project-story.md).** It is outside the measured interval and is not included in the 38 new-ticket count above. Other actionable remaining delivery work already has owners in the tracker: #111 (test roadmap, including reconciling now-removed double-run assumptions), #112 (command boundaries), #090 (reconcile delivered vector work with its still-Open status), #096/#097 (visual work), and #115–#117 (camera/live plans). Their existing status is preserved; this story does not silently close partially delivered work. No allocated worker could run, so the host completed extraction review and records the outstanding independent gate in #119.

Neighbor-aware glyph selection remains a design option under broader image-quality work (#006), not a commitment to build all alternatives in the continuity study. Cross-remote fetch discipline, HTO usage, and avoiding repetitive waits are existing rules, so no duplicate policy ticket is allocated. Fresh performance benchmarking, live release-asset verification and broad backlog closure are non-goals of this retrospective. The read-only reviewer must confirm whether any additional actionable finding needs a ticket before finalization.

## 8. File & Diff Summary

Reproduce interval size with `git diff --shortstat 8dcf63b..0f866fd -- . ':!issues'`. Commit classification uses `git log --no-merges --name-only` and counts a commit as docs/tracker-only only when every changed path begins `docs/` or `issues/`.

| Exclusive path category | Files | Added | Removed |
|---|---:|---:|---:|
| `docs/` | 49 | 1,980 | 977 |
| `*_test.go` or `testhelper/` | 49 | 3,828 | 884 |
| Other Go files | 52 | 5,986 | 3,742 |
| `spec/` after Go/test classification | 31 | 915 | 139 |
| Other `testdata/` fixtures | 25 | 0 | 0 |
| `website/` | 5 | 244 | 972 |
| Everything else | 22 | 464 | 103 |
| **Total outside `issues/`** | **233** | **13,417** | **6,817** |

Git reports 25 fixture files and three website assets as binary; their bytes are excluded from line totals. Generated indices, synchronized Harnez docs/rules, release version files and moved content are included in their path buckets. These are diff lines, not a count of newly authored application logic. In particular, splitting command files contributes additions and removals without equivalent behavior growth. Current tracked Go size is 174 files, including 79 test files, and 43,390 lines; the prior story reported 132/65/38,202 respectively.

Key delivery commits: `8bbb055` (vector/octant), `a691aa5` (parity/reconstruction), `e900675`/`ffc87d5` (inspection/probes), `1ef498a` (spec defaults), `f3b0153`/`936d480`/`984ea2d` (spec split), `6dd6fdc` (CI/container), `9d3ab8d` (internal extraction), `73d9f34` (parallel CI), `424e634` (audio bound), `61a85a8`/`58e0f1c` (standard paths), `935934d`/`d5f9fcb` (reference goldens), and `0f866fd` (GitHub integration).

Important files include `spec/glyph_sets/`, `spec/glyph_shapes/`, `spec/compositions.yaml`, `cmd/modes_glyphs.go`, `cmd/modes_probe.go`, `internal/{browser,modes,ansi}/`, `v1/*/reference_test.go`, `testdata/`, `.github/workflows/ci.yaml`, `Makefile`, `website/page.yaml`, and the testing/rendering evergreens. The current story change adds this document and its generated documentation-index entry; its eventual commit is outside the measured interval.
