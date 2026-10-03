# 076 — Audit spec-driven approach violations and classify candidates

**Status:** Open
**Priority:** P2 (Medium)
**Severity:** Moderate
**Category:** Architecture
**Related:** [004](004-spec-loose-ends.md), [025](025-spec-driven-render-modes.md), [057](057-honour-spec-experimental-render-modes-across-mode-consumers.md), [Spec System](../docs/Spec.md)

---

## 1. Problem & Motivation

The YAML spec is intended to be the source of truth for application behavior and content, but a read-only audit found several places where source code still owns policy, user-facing values, or duplicate defaults. This ticket inventories those candidates and recommends how strongly each belongs in the spec. It records likely files to change so follow-up work can be split into bounded changes.

This is an audit, not authorization to move values. Before implementation, confirm each candidate against its owning behavior and existing tickets; keep algorithm mechanics and external protocol constants in Go as described in `docs/Spec.md`.

## 2. Technical Specification / Findings

| Candidate set | Evidence / likely code locations | Likely spec location and files to touch | Spec-worthiness |
|---|---|---|---|
| CLI flag inventory and defaults | `cmd/root.go` declares the Cobra flag surface and defaults; `spec/cli.yaml` currently describes only `bench` and `bench_budget`. | Expand `spec/cli.yaml` and `spec/schemas/cli.schema.json`; likely consumers in `cmd/root.go` and CLI help/registration tests. | **Strong.** Names, defaults, and descriptions are stable user-facing interface. Keep Cobra wiring in Go, but derive or integrity-check its declarations against the spec. |
| Zoom ladder policy | `spec/zoom_levels.yaml` provides levels and strategy name, while `internal/viewgeom/viewgeom.go` still hardcodes minimum zoom, extension breakpoints, and increments (`0.125`, `2`, `5`, `15`, `32`, `64`, and step sizes). | `spec/zoom_levels.yaml`, its schema, loader/types in `spec/load.go`, and `internal/viewgeom/viewgeom.go`. | **Strong.** These are product zoom choices and the file already claims ownership. Keep rounding/deduplication mechanics in Go. |
| Config defaults duplicated across specs | `spec/config.yaml` and `spec/controls.yaml` repeat defaults for preview height, view mode, max jobs, video frame count, and preview delay. Runtime loading and settings application are in `cmd/config.go` and `cmd/browser.go` (confirm exact ownership when implementing). | Choose one canonical owner, likely `controls.yaml` for settings-backed values; adjust `spec/config.yaml`, schemas, loaders, and config initialization. | **Strong.** Duplicate defaults can silently drift. Preserve any defaults that are intentionally distinct and document why. |
| Input event display names | `internal/input/input.go` hardcodes labels such as “Focus Gain”, “Focus Loss”, “Resize”, “Quit Signal”, and “Unknown”; `spec/input.yaml` defines events and token rules but not their display names. | `spec/input.yaml`, `spec/schemas/input.schema.json`, `internal/input/input.go`, and input spec tests. | **Moderate.** Promote names used in UI/debug output; keep internal enum identifiers and dispatch mechanics in Go. “Unknown” may remain a generic code fallback if it represents an invalid/unrecognized state rather than owned content. |
| Crop syntax and alignment defaults | `cmd/crop.go` owns accepted aliases/grammar and defaults such as center/middle alignment. | Likely a new crop section in an existing suitable spec or a new `spec/crop.yaml` plus schema; `cmd/crop.go`, loader/embed integrity tests. | **Moderate.** Alignment and accepted user-facing tokens are policy; parsing mechanics stay in Go. Decide ownership before adding a new spec file. |
| Playback defaults | `cmd/play.go` falls back to 15 FPS in multiple paths. | Likely `spec/config.yaml` or a playback section in `spec/cli.yaml`, corresponding schema, and `cmd/play.go` plus CLI tests. | **Moderate.** A default playback rate is user-visible behavior. Keep frame scheduling and timing math in Go. |
| Fixed SVG rasterization ceiling | `v1/halfblock/svg.go` defines `SVGMaxDim = 2048`; `cmd/thumbqueue.go` consumes the SVG probe/scaling path. | Consider a media/rendering policy section in an existing spec; update schema, loader, SVG loader and relevant callers/tests. | **Moderate to weak.** It affects output and resource use, but may be an implementation safety bound. Specify only if it is an intentional product limit rather than a defensive decoder constraint. |
| Render mode runtime contract gaps | `spec/render_modes.yaml` declares modes and some renderer IDs, but `renderers` omits entries for declared modes such as `sparkline_six_half` and `sparkline_spark_six`; actual dispatch and geometry remain partly distributed in `cmd/render_pipeline.go`, `cmd/root.go`, `v1/sparkline/render.go`, `v1/sextant/render.go`, and related backends. | Continue under [025](025-spec-driven-render-modes.md): complete/validate the registry in `spec/render_modes.yaml` and its schema/loader, and reconcile cmd/backend dispatch. | **Strong for stable mode identities/contracts; weak for executable algorithms.** Renderer implementation, scoring, color quantization, and performance mechanics should stay in Go. |
| Button theme styles not consumed | Buttons carry `style` references in `spec/buttons.yaml`, and tokens exist in `spec/theme.yaml`, but ticket [004](004-spec-loose-ends.md) records that button rendering ignores them. | Follow up under [004]: `cmd/browser.go`/button rendering plus `spec/theme.yaml` and relevant UI tests. | **Strong**, already specified; this is an unconsumed spec value rather than a missing value. |

These are candidate groups from a source/spec audit, not a claim that every numeric literal is spec content. Keep external protocol values (for example terminal protocol bit masks), algorithm constants tied to scoring/geometry internals, test fixtures, and one-use arithmetic in code unless they are deliberately exposed as product policy.

## 3. Implementation & Verification Plan

- Review the candidate groups and confirm ownership before editing any code or spec.
- Avoid duplicating work already tracked by [004](004-spec-loose-ends.md), [025](025-spec-driven-render-modes.md), and [057](057-honour-spec-experimental-render-modes-across-mode-consumers.md); link follow-up work to those tickets where appropriate.
- For each accepted spec change, update YAML, JSON Schema, loader/consumer, and integrity tests together. Do not add Go fallbacks that mirror spec values.
- Keep excluded algorithm mechanics and protocol constants in Go, with a short rationale where a nearby spec value could be mistaken for their owner.
- Verify schema conformance, loader fidelity, consumption, and the relevant package/CLI tests after implementation.
