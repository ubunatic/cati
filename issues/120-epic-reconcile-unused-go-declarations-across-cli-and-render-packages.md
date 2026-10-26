# 120 — Epic: reconcile unused Go declarations across CLI and render packages

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Maintenance
**Related**: [108 — shared lint](108-fix-harnez-lint-findings-and-enforce-shared-lint-target-in-ci.md), [112 — command separation](112-modularize-cmd-package-extract-browser-player-and-modes-into-internal-sub-packages.md), [079 — spec ownership](079-spec-policy-ownership-decisions-cli-zoom-options-playback-and-input-labels.md)

---

## 1. Problem & Motivation
At `f178232`, `HTO=0 harnez exec --quota-1 -- gokay check --all --json` returns exit 1: formatting and vet pass, tests pass (35,837ms), but golangci-lint reports 36 unused declarations across command, example and renderer packages. This is distinct from closed #108, which fixed Harnez shell findings and wired shared lint; do not reopen that completed scope or suppress unused checking to conceal this finding.

This is cross-package maintenance, not a single quick fix. Some declarations are only exercised by tests, and renderer helper removal must preserve reference independence. Plan bounded child work before deleting everything mechanically.

## 2. Technical Specification / Findings
### Baseline diagnostic inventory (file:line at f178232)

| Path | Symbols |
|---|---|
| `cmd/browser_config.go` | `loadAltButtonActions` :249; `parseYamlView` :411 |
| `cmd/interactive_render.go` | `renderCfg.preScaleName` :206 |
| `cmd/interactive_test.go` | `abs` :855 |
| `cmd/interactive_zoom.go` | `parseZoomK` :78 |
| `cmd/media_benchmark.go` | fields `mode` :19; `decode` :21 |
| `cmd/modes.go` | `listModes` :316; `printSingleImageDemoItem` :576; `printDemoItem` :581; `autoFitGridCols` :739 |
| `cmd/modes_benchmark.go` | `sortBenchmarkResults` :183 |
| `cmd/modes_metrics.go` | `renderModePair` :181 |
| `cmd/render_pipeline.go` | `renderMode.renderAspectCorrection` :209; `renderMode.renderCellSize` :217 |
| `cmd/ssim.go` | `renderSSIM` :63 |
| `examples/imgbrowser/preview.go` | `max` :677 |
| `examples/mediabrowse/preview_pane.go` | `newMediaPreviewPane` :58 |
| `v1/halfblock/render.go` | `ansiColor` :525 |
| `v1/quadblock/render.go` | `fgRGB` :61; `bgRGB` :65 |
| `v1/render_progress_test.go` | `assertRenderProgress` :9 |
| `v1/sextant/render.go` | `displayMask` :222; `avgRGBA` :321; `topNMask` :474; `rowMask` :508; `colMask` :544; `heuristicMasks` :582; `popcount` :631; `maskOverlap` :868; `cellEscape` :884; `renderCell` :895; `renderRow` :911 |
| `v1/sparkline/render.go` | `blockChar` :1058; `fgRGB` :1131; `bgRGB` :1135 |

Raw check output is retained in `.git/harnez/quota-1-logs/test-20261026T164946.140920951Z.log`; extracted JSON is saved in `.git/harnez/gokay-check-20261026.json` (local evidence, not portable tracked artifacts). The portable inventory and step results are recorded here. The JSON scope reports 174 Go files and `packages: 1`, without per-package test results. The test step passed; inspect the concrete test invocation before interpreting that scope count as a Go package coverage count. No integration/race coverage is claimed by this run.

Representative Tier-2 graph verification: fast generation `2026-10-26T16:50:45Z`, project `home-uwe-projects-cati`. `ansiColor` has no graph callers and simply forces alpha to 255; `parseYamlView` has a test caller `TestBrowser_ParseYaml`; `popcount` also has incoming graph edges. Thus lint's unused set is not equivalent to 36 functions with no references. Coverage reported no recorded issues for the inspected CLI/renderer files; this remains best-effort evidence. Examples were excluded by the fast index and need direct reads or a broader index before remediation. No exhaustive dead-code proof is claimed.

`gokay assess --json` reports 28 packages, 85 code files, 75 test files, 24,107 code lines and 16,769 test lines (ratio 0.696; not coverage). Seven packages have no test files: root/version, three thin CLI binaries, imgbrowser, scripts, and term. This alone is not seven confirmed defects. The cmd hotspot (9,963 nonblank code lines, fan-out 15) remains covered by #112; test posture remains under #111.

Spec assessment reports schema_valid=true, 17 orphan-key candidates, four unspecced constants and eight numeric shadow matches. It supplies no individual paths for the 17 orphan keys, so these remain unverified heuristics, not confirmed unused application objects. Ownership triage is recorded in #079; numeric equality between unrelated durations and layout/glyph values is not duplication evidence.

## 3. Implementation & Verification Plan
### Milestones and bounded child work

1. **Classify and plan**: inspect every listed declaration under normal and player/browser/integration build tags, tests, function values and independent references. Record remove/move-to-test/retain-with-reason decisions. Split the following work into implementation child tickets after that audit, avoiding duplicate work with #112.
2. **CLI child work**: remove obsolete command adapters and benchmark fields, or relocate test-only helpers into `_test.go` while retaining meaningful tests. Coordinate with #112; update relevant evergreens if an interface or data flow changes.
3. **Renderer child work**: distinguish retired production helpers from reference-test dependencies; preserve naive reference independence and all public APIs. Cleanup must not change rendered pixels or regenerate goldens. If a behavior defect emerges, follow the rendering playbook as separate work.
4. **Examples/progress child work**: inspect the excluded example paths directly, remove or relocate genuinely unused constructors/helpers, and retain progress-test intent.
5. **Verification and reconciliation**: after source changes, rerun `gokay check --all --json` under Quota-1 and compare 36 -> 0 unused diagnostics; preserve fmt/vet/test passes. Run relevant tagged checks and `HTO=0 make preflight`, record results and close verified children. Broader work stays open until verified.

### Diagnosis outcome — 2026-10-26

No source remediation attempted: this is Epic-scale work requiring classification and coordinated package cleanup. No ready localized ticket batch was allocated, so the quick-fix sprint was skipped. The last check remains the baseline failure above; no redundant full rerun was made without source changes, in accordance with Quota-1. Existing Harnez advisor-leaf restrictions also prevent dispatching a worker from this session; an authorized host can run the planned implementation sprint.

### Wrap update — 2026-10-26

The role restriction described in the historical diagnosis is resolved by Harnez #953, which is fixed and Closed; this host's background self-test passed without overrides. The 36 unused findings remain unremediated. Resume with milestone 1's classification, preserving test-only helpers and reference independence rather than deleting every lint-listed declaration. Tracker metadata lint is a separate finding, now #121.
