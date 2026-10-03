# 076 — Spec ownership audit: confirmed gaps and policy candidates

**Status:** Closed — audit complete; remaining ownership choices moved to a follow-up ticket
**Priority:** P2 (Medium)
**Severity:** Moderate
**Category:** Architecture
**Related:** [004 — spec loose ends](004-spec-loose-ends.md), [025 — render-mode contracts](025-spec-driven-render-modes.md), [057 — experimental-mode filtering](057-honour-spec-experimental-render-modes-across-mode-consumers.md), [075 — CLI geometry contracts](075-unify-cli-render-geometry-contracts-and-validate-route-specific-options.md), [022 — target-aware SVG rasterization (closed)](022-svg-fixed-rasterization-size.md), [Spec System](../docs/Spec.md), [Terminal Input](../docs/Input.md)

---

## 1. Problem & Motivation

The embedded YAML spec is application code and must own its declared content. Duplicate defaults, partially consumed definitions, and code-owned interface policy are different problems: an existing spec value ignored or shadowed by Go is a confirmed ownership gap; a value with no declared owner is a migration candidate requiring a design decision.

**Goal:** Maintain a source-backed inventory with evidence, candidate strength, owning ticket, likely files, and verification requirements so accepted work can be split into bounded changes. This ticket covers audit/refinement only, not implementation. It remains Open while ownership decisions and follow-up disposition are outstanding.

**Audit baseline:** Read-only inspection on 2026-10-03 began at `44c5776` with existing edits to `cmd/root.go` and `internal/viewgeom/viewgeom.go`. Concurrent issue-075 geometry work advanced HEAD to `4442e06` during inspection; CLI declarations and `ZoomSteps` findings were re-checked afterward. Hints use symbols rather than unstable line numbers. Re-check the live repository before implementation; this targeted inventory does not certify that every spec property or action has been audited.

**Strength guide:** Strong = explicit spec ownership or stable interface/policy with a clear declarative use. Moderate = plausible policy/content, but ownership or consumer value needs a decision. Weak = implementation/resource mechanics unless intentionally exposed as product policy. Strength measures suitability for spec ownership, not severity or scheduling urgency. Go retains handlers, parsers, algorithms, and platform integration.

## 2. Technical Specification / Findings

### A. Confirmed gaps in already-declared spec ownership

| Candidate | Current evidence | Recommendation / owner | Likely files and verification |
|---|---|---|---|
| Settings defaults | All **six** defaults, including `preview_videos`, repeat in `config.yaml` and `controls.yaml`. `loadSpecConfigDefaults` additionally seeds Go defaults (height **20**, jobs **0**, frames **10**) differing from YAML (**40**, **4**, **8**), then conditionally overlays valid fields. | **Strong.** Choose one canonical default owner; controls does not currently supply runtime defaults. Preserve user overrides; resolve intentional differences such as jobs=0 auto versus control min=1. | `spec/config.yaml`, `spec/controls.yaml`, their schemas; `spec/load.go` (`ConfigDef`, `ControlDef`); `cmd/browser.go` (`loadSpecConfigDefaults`, `loadConfig`, `saveConfig`); `cmd/browser_test.go`, `cmd/spec_test.go`. **There is no `cmd/config.go`.** Test default agreement, overrides, and missing/invalid spec behavior. |
| Partial controls consumption | `loadControls` fixes six keys, types, order, bounds, and enum values in Go; YAML overrides only bounds/values for those keys. `ControlDef.Default`, `.Set`, and `.Get` are loaded but not used there; `applySettingsDelta` dispatches by key and hardcodes a 100 ms delay step. | **Strong** for inventory/types/bounds and binding fidelity; **moderate** for declaring adjustment steps/order. [004 §D](004-spec-loose-ends.md#d-controlsyaml--declared-but-not-read) correctly notes bounds are read, but that is not full consumption. Wire declared bindings/defaults or explicitly remove/redefine obsolete fields. | `spec/controls.yaml`, `spec/schemas/controls.schema.json`, `spec/load.go`, `cmd/browser.go` (`ControlSpec`, `loadControls`, `applySettingsDelta`, `drawSettingsPage`); browser/spec tests. Verify added/removed controls, ordering, type fidelity, and handler completeness. |
| Input shadow tables / silent degradation | `DefaultSpec` duplicates aliases, mouse fields, terminal sequences, and tokenizer rules from `input.yaml`. `parse` starts with defaults and restores tokenizer rules if none parsed, returning nil error. Several cmd consumers discard `input.Load` errors. | **Strong, confirmed no-fallback violation.** Distinguish missing spec from present-but-invalid spec; graceful degradation must not restore a second copy of spec content. Preserve raw/unrecognized key representation and structural Ctrl-C safeguards. | `internal/input/input.go` (`DefaultSpec`, `Load`, `parse`), `spec/input.yaml` and schema if the contract changes; `cmd/input_tester.go`, `cmd/browser.go`, `cmd/interactive.go`; `internal/input/input_test.go`, `cmd/spec_test.go`, `cmd/interactive_test.go`. Many tests depend on `DefaultSpec`: migrate fixtures deliberately. Test valid variants, missing files, malformed content, and omitted sections. |
| Input declarations not consumed | `parse` ignores `signals`, mouse enable/disable sequences, and `btn_no_button`. `MouseEnable*`/`MouseDisable*` return literals; `IsDrag`/`IsMove` compare button to 3. OS signal registration is separate in cmd. | **Strong** for declared sequence/field fidelity; **moderate** for the signal contract. Protocol origin does not excuse shadowing an existing spec field. Consume it or deliberately remove/reclassify it. Resolve OS signals by portable names/constants, not universal YAML signal numbers. Resize behavior belongs with [075](075-unify-cli-render-geometry-contracts-and-validate-route-specific-options.md). | `spec/input.yaml`, `spec/schemas/input.schema.json`, `internal/input/input.go`, `cmd/input_tester.go`, `cmd/interactive.go`, `cmd/browser.go`; input/spec tests. Verify loaded sequence fidelity and declared-versus-supported signals; preserve public mouse helper behavior. |
| UI content/style shadows | `getAboutView` contains a second About page, including `Version: 1.0.0` and shortcut prose, on load failure. `loadStyle` seeds caps, scrollbar glyphs, and other content before selective spec overlays. | **Strong** for content already owned by `about.yaml`/`style.yaml`. Remove shadow content while defining graceful missing-spec behavior; keep generic formatting mechanics in Go. | `cmd/browser.go` (`getAboutView`, `loadStyle`), `spec/about.yaml`, `spec/style.yaml`, `spec/load.go`, `spec/schemas/style.schema.json`; browser/spec tests. About has no dedicated schema in the current inventory: decide validation coverage if changing its contract. |
| Button theme tokens ignored | `buttons.yaml` declares `style: primary/secondary/danger`; `theme.yaml` defines tokens. `loadButtons` consumes text only; `loadStyle` has a matching FIXME. | **Strong; already specified.** Continue under [004 §C](004-spec-loose-ends.md#c-themeyaml-style-tokens--stored-but-not-applied), rather than creating another theme feature. | `cmd/browser.go` (`loadButtons`, `loadStyle`, `drawBottomMenu`), `spec/load.go` (`ButtonDef`, `LoadTheme`), `spec/buttons.yaml`, `spec/theme.yaml`, schemas only if changed; browser/spec tests for token resolution and rendered styles. |

### B. Policy/content candidates requiring an ownership decision

| Candidate | Current evidence | Strength / recommended boundary | Likely files and verification |
|---|---|---|---|
| CLI inventory, defaults, help, completion | `New`, `NewPlay`, and `NewBrowse` in `cmd/root.go` repeat flag declarations, defaults, hidden flags, and `NoOptDefVal`. `spec/cli.yaml` has only bench flag long names, descriptions, and handlers; **no default/type fields**. `TestSpecCLIBenchmarkFlags` checks names/handlers and Cobra presence, not help/default fidelity. Other flags live in `cmd/modes.go`. | **Strong** for stable metadata. Expand the contract deliberately with command-specific differences; derive declarations or comprehensively check fidelity. Wiring stays in Go. Reuse render-mode metadata rather than copying it into a CLI spec. | `spec/cli.yaml`, `spec/schemas/cli.schema.json`, `spec/load.go` (no typed CLI loader today), `cmd/root.go`, `cmd/modes.go`, `cmd/completion.go`; `cmd/root_test.go`, `cmd/cli_flags_test.go`, `cmd/completion_test.go`, `cmd/spec_test.go`. Render-mode completion already derives from the registry. Route semantics/validation belong with [075](075-unify-cli-render-geometry-contracts-and-validate-route-specific-options.md). |
| Zoom policy / shadow defaults | `zoom_levels.yaml` owns levels, strategy name, and upscale policy. `ZoomSteps` hardcodes min k=0.125 and named-strategy thresholds/steps; YAML comments describe them. `cmd/interactive.go:loadZoomLevels` seeds a different Go ladder. `spec.LoadZoomLevels` seeds upscale=true and silently skips invalid numeric levels. | **Strong** for removing shadows and rejecting invalid data explicitly. **Moderate to strong** for declarative thresholds/steps: named executable strategies may legitimately stay in Go; decide whether their numbers are tunable policy. Keep rounding/deduplication and k-to-zoom math in Go. | `spec/zoom_levels.yaml`, its schema, `spec/load.go`, `cmd/interactive.go` (`loadZoomLevels`, `zoomSteps`), `internal/viewgeom/viewgeom.go` (`ZoomSteps`); viewgeom/interactive/spec tests. Cover cutoff, strategy boundaries, invalid strategy/levels, and missing spec. |
| Accepted option tokens / crop alignment | `parseCropSpec`/`parseAutoCropSpec` own a/auto/1/true, alignment aliases, and center/middle defaults. `completeCropSpecs` repeats help/examples; other completion tables repeat play/aspect/prescaler/zoom choices. | **Moderate**, rising to **strong** for stable lists reused in validation/help/completion. Prefer a CLI option-values section or an existing domain owner; a new crop spec is not yet justified. Keep grammar, integer parsing, and offset math in Go. | `cmd/crop.go`, `cmd/completion.go`, `cmd/root.go` (`validateCommonFlags`, `parsePlayMode`), `internal/viewgeom/viewgeom.go` (`ParseZoomK`); chosen YAML/schema/loader; root/completion/CLI tests. Coordinate mapping/crop semantics with [075](075-unify-cli-render-geometry-contracts-and-validate-route-specific-options.md). Examples need not enumerate every numeric expression. |
| Playback fallback rates | `cmd/play.go` uses 15 FPS for image playback and video probe/invalid-rate fallbacks. `cmd/interactive.go:interactiveVideo` starts with 24 FPS before probing. These are separate route defaults. | **Moderate to strong** for user-visible fallback policy. Decide whether 15 versus 24 is intentional before unifying. Distinguish explicit FPS, image defaults, native video FPS, and probe failure. Scheduling arithmetic stays in Go. | `cmd/play.go`, `cmd/interactive.go`, `cmd/root.go` (FPS help/default); proposed playback section in `spec/config.yaml` or CLI spec plus schema/loader; play/interactive/root tests. Verify override and probe success/failure on both routes. |
| Input display vocabulary | `seqNames`, `MouseName`, and `EventName` hardcode canonical key, mouse, and event labels. `cmd/input_tester.go` displays event names; `input.yaml` has no display-label section. | **Moderate.** Specify stable names if useful to UI/help consistency. Keep enum IDs, Unicode/hex formatting, and generic unrecognized-state representation in Go; not every diagnostic needs YAML. | `internal/input/input.go`, `cmd/input_tester.go`, `spec/input.yaml` or `spec/labels.yaml` and corresponding schema/loader; input/spec tests. Assess library API implications: standalone `MouseName` currently receives no spec. |
| SVG unconstrained raster default | `SVGMaxDim=2048` is used when **both target dimensions are unconstrained**, including the context loader. `RasterizeSVGWithTarget` accepts explicit targets; [022](022-svg-fixed-rasterization-size.md) is closed. It is **not a universal 2048px ceiling**. | **Weak to moderate.** Retain as a documented library/resource default unless product policy needs a declarative owner. Do not reopen the resolved target-scaling bug or force cmd configuration into a reusable loader. | `v1/halfblock/svg.go` (`svgRasterTarget`, `fitSVGTarget`, `RasterizeSVGWithTarget`), `v1/halfblock/image_async.go`, `cmd/thumbqueue.go`; SVG/async image tests. If accepted, first define owner, schema/loading, and constrained-versus-unconstrained behavior. |

### B2. Re-check after audit (2026-10-03, HEAD `875f311`)

Committed work since the audit (`4877f2c..875f311`, issue 075 M2–M4, 077, 078, aspect modes) changed these findings:

- **075 is closed** (`8e1bc04`). Links to 075 above are historical; route semantics, crop mapping, and resize now need a new owner ticket (or 078 for aspect edge cases) if accepted.
- **New Go-owned token table:** `cmd/completion.go` `aspectModes` (`354c8d0`) is the single list of `--aspect` values, descriptions, and aliases (`pixel`/`raw`/`1:1`, `contain`/`fit`) for validation, help, and completion. It removes duplication within Go but adds a **moderate-to-strong** candidate to the "Accepted option tokens" row: an aspect-mode spec owner. Flag help strings in `New`/`NewPlay`/`NewBrowse` still hardcode `default|aligned|pixel|contain` separately.
- **CLI inventory row still holds:** flag declarations remain repeated across the three constructors; `cmd/root.go` grew further.
- **Playback fallback rates unchanged:** `cmd/play.go` still uses 15 FPS fallbacks (refactored, same literals).
- No commit touched `spec/`, `internal/input`, `cmd/browser.go`, or `ZoomSteps`; sections A and the other B rows are unchanged.

### C. Existing render-contract work and corrected historical claims

- **Renderer registry omission is stale:** `spec/render_modes.yaml:renderers` now includes both `sparkline_six_half` and `sparkline_spark_six`; `TestSpecRenderModesIntegrity` recognizes them. Do not propose adding them again.
- **Strong for identities/inventories/declared geometry; weak for executable algorithms.** Remaining dispatch/coverage alignment belongs under [025](025-spec-driven-render-modes.md). Hints: `spec/render_modes.yaml`, its schema, `spec/load.go`, `cmd/render_pipeline.go`, `cmd/modes.go`, `v1/sparkline/render.go`, `v1/sextant/render.go`, `cmd/spec_test.go`, `spec/glyph_sets_test.go`. A Go switch implementing a renderer ID is expected; distributed implementation alone does not prove a violation. Identify a specific ignored field or mismatched contract first.
- **Experimental filtering belongs under [057](057-honour-spec-experimental-render-modes-across-mode-consumers.md):** `withoutExperimentalModes` in `cmd/media_benchmark.go` already reads `Experimental` from the spec. Audit all-mode consumers in `cmd/smart_render.go`, `cmd/modes.go`, and cycling in `cmd/interactive.go` before claiming they ignore it. Preserve explicit experimental-mode selection.

**Exclusions:** Keep scoring, color quantization, mask execution, rounding, scheduling, one-use arithmetic, and test fixtures in Go. External protocol/platform constants may also stay in Go, but an already-declared field must be consumed, deliberately removed, or explicitly classified as documentation with validation. A literal or handler switch is not by itself evidence of a violation.

## 3. Disposition & Verification Plan

Re-checked at HEAD `6ca59d5` on 2026-10-03. Dispositions are audit decisions, not blanket migration approval.

| Candidate | Disposition | Rationale / boundary |
|---|---|---|
| A — Settings defaults | **Accept** | Go seeds height/jobs/frames differently from YAML, and six values repeat in config/controls. Make runtime defaults canonical and preserve user overrides. |
| A — Partial controls consumption | **Accept** | Go fixes inventory/order/types and fallback bounds; loaded bindings/defaults are partly unused. Consume declared control metadata and test add/remove/change fidelity. |
| A — Input shadows/silent degradation | **Accept** | `DefaultSpec` mirrors aliases/protocol/tokenizer data and malformed content restores tokenizer rules. Missing spec should yield raw-key behavior without shadows; present-invalid must error. |
| A — Input declarations not consumed | **Accept** | Mouse sequences and `btn_no_button` are ignored; portable OS signal registration remains Go-owned and needs explicit treatment. |
| A — UI content/style shadows | **Accept** | About fallback prose and style seed values duplicate `about.yaml`/`style.yaml`; retain graceful missing-spec behavior without duplicate content. |
| A — Button theme tokens | **Already tracked** | Issue [004 §C](004-spec-loose-ends.md#c-themeyaml-style-tokens--stored-but-not-applied) owns applying declared theme tokens. |
| B — CLI inventory/defaults/help/completion | **Defer** | Stable metadata may merit spec ownership, but command-specific contract needs design; CLI inventory work also owns `cmd/root.go`. |
| B — Zoom policy/shadows | **Defer** | Remove-vs-declare strategy thresholds needs an owner decision; named executable strategies may remain Go-owned. |
| B — Accepted option tokens/crop alignment | **Defer** | Ownership boundary and domain spec are undecided; examples need not enumerate numeric grammar. |
| B — Playback fallback rates | **Defer** | 15 FPS and 24 FPS are distinct route defaults; intent must be decided before unification. |
| B — Input display vocabulary | **Defer** | Stable labels may merit a spec, but library API ownership needs design. |
| B — SVG unconstrained raster default | **Exclude** | 2048 is a documented library/resource default only when both target dimensions are unconstrained, not a universal product ceiling. |
| B2 — Aspect-mode table | **Already in progress** | Another developer owns aspect-mode spec work; do not duplicate it here. |
| B2 — CLI inventory | **Defer** | Same command-specific contract decision as the §B CLI candidate. |
| B2 — Playback fallback rates | **Defer** | Same 15/24 FPS route decision as the §B playback candidate. |
| C — Renderer registry omissions | **Exclude as stale** | Current `render_modes.yaml` includes both sparkline renderer entries and integrity coverage recognizes them. |
| C — Experimental filtering | **Already tracked** | Issue [057](057-honour-spec-experimental-render-modes-across-mode-consumers.md) owns the all-mode consumer audit. |
| C — Historical geometry references to 075 | **Historical/closed** | Issue 075 is closed; new geometry scope needs a current owner rather than reviving its links. |

Prioritize accepted shadow/fidelity gaps as bounded follow-ups: input, settings/controls, then UI. For each accepted implementation, update YAML/schema/loader/consumer/tests together where the contract changes; tests must prove changed spec values affect behavior. Keep missing-file degradation distinct from invalid-present errors. Preserve user overrides, public APIs, command-specific behavior, and structural-key exceptions. Run relevant package/CLI tests, `go vet ./...`, `go test ./...`, and `make install`; use repository-supported schema validation and do not assume `make validate-spec` exists.

### M2 — Input shadow tables

Implemented in the M2 commit: removed `DefaultSpec` and parser fallback tables; missing `input.yaml` now yields an empty spec for raw-key handling, while malformed or incomplete present content returns an error. Browser, viewer, and input-test entry points surface loader errors. Existing mouse enable/disable sequences and no-button value are loaded from YAML. Tests cover missing/invalid specs and prove a changed alias changes behavior. Verification passed: `go vet ./...`, `go test ./...` (no `--- FAIL` output), and `make install`.

**Audit acceptance:** Every retained candidate has evidence, a strength rationale, an owner or ownership question, file/test hints, and a disposition. Remove stale findings rather than converting them into work; synchronize the issue index. This targeted inventory is not an exhaustive repository-wide audit; remaining accepted gaps and deferred ownership decisions are follow-up work.

**M2 host review:** accepted. Full suite green on review. An empty (missing-file) spec still consumes unmatched bytes one by one, so Ctrl-C (`\x03`) still passes through.

### M3 — Settings defaults

**Pre-Work / Required Refinements (from M2 review):**

1. **Public mouse helper regression:** `MouseEvent.NoButton` is a new exported field whose zero value is `0`. A `MouseEvent` built outside `ParseMouse` (library users, test literals) now reports `IsDrag()==false` for a left-button drag (`Button==0`) and `IsMove()==true` for it. §3.4 requires preserving public APIs. Fix without a Go copy of the spec value: e.g. `ParseMouse` resolves drag/move into the event when it parses (it has the spec), so the helpers no longer compare against a field that literals leave at 0. Document the chosen contract in `docs/Input.md`. Add a test with a struct-literal left drag.
2. **Dual parsing:** `parse` now runs `yaml.Unmarshal` for validation and then the old line parser for values. Optional if cheap: decode values from the unmarshalled document instead, so one parser defines the format. Skip if it grows M3 beyond one commit; note the decision here.

Then implement M3 as planned (`config.yaml` canonical for runtime defaults; tests prove a changed spec default changes the loaded config and user overrides still win).

**M3 pre-work decisions:** Preserve `MouseEvent`'s exported struct shape and existing `Button==3` no-button API convention. `ParseMouse` interprets the spec-declared code and normalizes it at the loader boundary; helper behavior for external struct literals remains based on the 0–2 held-button range. A struct-literal regression test covers left drag and move. Dual parsing is deferred: replacing the legacy line parser is unrelated to settings defaults and would expand this bounded milestone.

**M3 implementation:** `config.yaml` is the only owner of initial settings values; duplicate `default` values were removed from controls. The config loader validates required fields and ranges, returns invalid-spec errors to browser startup, and has tests proving a changed YAML default changes loaded settings while user overrides win. Verification passed: `go vet ./...`, `go test ./...` (no `--- FAIL` output), and `make install`.

**M3 host review:** accepted. The mouse helpers now use the public SGR "no button = 3" convention, and `ParseMouse` maps the spec's value onto it, so struct literals behave as before. The effective defaults are unchanged (40/4/8): the old Go seeds were always overwritten by valid YAML values. `controls.yaml` no longer carries `default:`, so `config.yaml` is the single owner. Dual parsing in the input loader is deferred (recorded by the developer).

### M4 — Controls fidelity

**Pre-Work / Required Refinements (from M3 review):** none blocking. Keep `ControlDef.Set`/`.Get` either consumed (dispatch by declared binding) or removed together with their YAML/schema fields; do not leave them loaded-but-unused.

Then implement M4 as planned: the control inventory, order, type and bounds come from `controls.yaml`; the 100 ms delay step is either declared in the spec or justified in a code comment as mechanics. Tests: adding/removing/reordering a control in a fixture changes the settings page; a control with no Go handler fails an integrity test.

**M4 implementation:** `LoadControlsFrom` preserves YAML declaration order and validates control shape; `loadControls` builds the settings inventory directly from the spec, gracefully leaves it empty when absent, and returns malformed-spec or missing-handler errors to browser startup. `set`/`get` bindings dispatch through a registered handler table. Integer bounds and enum values come from the spec; the 100 ms delay step is documented as a UI input mechanic. Fixture tests cover changed inventory, order, bounds, enum values, and missing handlers. Design data flow is updated in `docs/Design.md`.

**M4 host review:** accepted. Order comes from the YAML node order; type, bounds, enum size and bindings are validated; a control without a Go handler is an error.

### M5 — Input declarations

**Pre-Work / Required Refinements (from M4 review):**

1. `controlHandlers` registers each handler under the bare key as well as `set_<key>`/`get_<key>`, so `set: preview_height` would also pass validation. Register only the declared binding names, and add a negative test for a bare-key binding.

Then implement M5 as planned: consume or deliberately remove the declared-but-unused input fields (`signals` and any others still unread after M2). For signals, resolve OS signals by portable names (`os.Interrupt`, `syscall.SIGWINCH`, ...), not by YAML signal numbers; if the YAML block stays, test that every declared signal is supported. Tests must fail when a spec value changes or is removed.

**M5 implementation:** The pre-work registry now contains only explicit `set_<key>` and `get_<key>` binding names. Input signals are parsed by portable names and exposed by event; browser and interactive/play paths register declared quit signals, while the input tester registers declared resize signals. Numeric signal values were removed from the input YAML/schema. Tests check all embedded declarations, changed signal behavior, unsupported names, and rejection of bare-key control bindings.

**M5 host review:** accepted with one required fix. Signals are now resolved by portable name and registered from the spec; bare-key control bindings are rejected.

### M6 — UI content/style shadows

**Pre-Work / Required Refinements (from M5 review):**

1. **Empty signal list relays every signal.** With a missing `input.yaml`, `Load` returns an empty spec, `SignalsFor(EventQuit)` returns nil, and `signal.Notify(sigs)` with no signals subscribes to *all* signals, including the Go runtime's SIGURG preemption signal, so playback and the viewer would quit at random. Guard all four call sites (`cmd/browser.go`, `cmd/interactive.go`, `cmd/play.go`, `cmd/input_tester.go`), ideally through one helper: register nothing when the list is empty (Ctrl-C still arrives as byte `\x03` in raw mode). Add a test that an empty spec yields no subscription.
2. Note only: `GOOS=windows go build ./...` already failed before M5 (`internal/audio` uses SIGSTOP/SIGCONT), and `syscall.SIGWINCH` adds one more failure. Windows is not a release target, so no action is needed.

Then implement M6 as planned: remove the second About page in `getAboutView` and the seeded caps/glyph content in `loadStyle`; a missing spec degrades without mirrored content, and invalid present content is an error. Test that a changed `about.yaml`/`style.yaml` value changes the output.

**M6 interrupted (host):** stopped mid-milestone on user request and committed as WIP `81139f5`. It was parked for release v0.2.9 (`0ca0e38`) and then restored (`f46d201`).

**M6 Pre-Work / Required Refinements (added after interruption):**

3. **Style spec no longer loads.** The new strict decoder (`KnownFields(true)`) rejects the `$schema:` line in `spec/style.yaml` (`field $schema not found in type spec.StyleSpec`). `TestHintBarAndBottomBarUseStyleNotHardcodedReverseVideo` fails, and the browser would fail at startup. Add a `Schema string \`yaml:"$schema"\`` field like the other spec structs, and add a test that loads the real embedded `style.yaml`/`about.yaml` without error.
4. Re-check the WIP diff in `cmd/browser.go`, `cmd/interactive.go`, `cmd/play.go`, `cmd/input_tester.go`, `cmd/linecap_test.go`, `cmd/browser_test.go` and `spec/load.go` before continuing: it was cut off mid-edit.

**M6 implementation:** A shared signal helper avoids calling `signal.Notify` for empty declarations at all four call sites. The About view fallback and style caps/glyph seeds are removed; missing specs yield empty content/style, while malformed or invalid present specs return errors to browser/interactive callers. Strict style loading accepts `$schema` through `StyleSpec.Schema`. Tests cover the embedded style/about specs, fixture-driven button/about output, empty-signal non-subscription, graceful missing specs, and invalid present specs.

**M6 host review:** accepted (`81139f5` + `7aa6fe3`). Full suite green on review. The installed `cati browse` starts and renders with no spec errors. Empty signal lists no longer subscribe to every signal (`notifySignals`). The About and style fallbacks are gone, and a missing spec degrades to empty content.

**Sprint result:** all accepted §A gaps are done (M2–M6). Still open: §B deferred policy decisions (CLI inventory, zoom thresholds, option tokens, playback FPS 15/24, input display names) and the input loader's dual parsing (YAML validation plus the line parser).
