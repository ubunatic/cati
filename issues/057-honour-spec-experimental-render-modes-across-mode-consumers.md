# 057 — Honour spec experimental render modes across mode consumers

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Design
**Related**: 056 (`--bench`, first consumer), 025 (spec-driven render modes), 046 (modes scorecard), `spec/render_modes.yaml` `experimental:`, `cmd/media_benchmark.go` `withoutExperimentalModes`

## /goal

`spec/render_modes.yaml` `experimental:` (currently `all+`, `z`, `z+`) is the
single source of truth for which modes are experimental. Every feature that
iterates "all modes" applies one shared filter (moved out of
`cmd/media_benchmark.go`) instead of its own list, and an explicit `--mode`
still selects an experimental mode.

Done when:

- Each all-modes consumer (e.g. `--smart` candidates, the `modes` scorecard,
  mode listings/cycling) is audited and either skips experimental modes or
  documents why it keeps them.
- No Go code hardcodes experimental mode names.
- A test covers the shared filter; docs mention the policy once.

## Notes

- Re-verify the current consumer list against code before starting.
