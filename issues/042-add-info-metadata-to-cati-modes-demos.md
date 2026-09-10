# 042 — Add `--info` metadata to `cati modes` demos

**Status:** Open
**Priority:** P2
**Severity:** S3
**Category:** Feature

## Problem

`cati modes` is a visual comparison tool, but its demo output currently leaves
agents and users to infer what each mode does from the rendered logos. The
command should optionally explain the rendering strategy and expose the shape
vocabulary used by each mode.

## Requested UX

Extend the command with:

```text
cati modes [modes...] --info
```

With no positional modes, the command keeps its current default of displaying
all available modes. Explicit mode names limit both the demo images and their
metadata to the requested modes. `--info` should add, below each mode's test
images:

1. one concise line explaining how that mode renders an image; and
2. a list of every Unicode shape/glyph available to that mode.

The metadata should appear for the normal and smart presentation without
duplicating it unnecessarily when both are shown. Existing output without
`--info`, including `--width` and `--smart`, should remain compatible.

## Scope and constraints

- Inspect the current `cmd/modes` output and mode selection behavior before
  choosing the exact layout and argument validation.
- Derive descriptions, glyph inventories, aliases, and related geometry from
  the mode/spec definitions where possible. Do not maintain a second hardcoded
  inventory in the CLI.
- Resolve generated glyph families (especially sextants) through the same
  source of truth used by rendering; list the actual available shapes rather
  than an unverified abbreviated range.
- Keep the feature informational: it must not change rendering, smart-width
  selection, or the selected mode's algorithm.
- Preserve readable output for narrow terminals and modes with large glyph
  families; define whether glyphs are listed inline, grouped, or wrapped.

## Acceptance criteria

- `cati modes --info` lists all current modes by default and places a concise
  mode explanation and complete shape list below each corresponding demo.
- `cati modes half --info` (and a supported alias) shows only that mode's demo
  and metadata; unknown names fail with the existing style of CLI error.
- `--info` composes correctly with `--width` and `--smart`, including the
  existing side-by-side cati/emojig and normal/smart layout.
- Metadata is sourced from the render-mode/spec definitions and remains
  correct when a mode's glyph-set membership changes.
- Tests cover default and filtered selection, flag composition, stable output
  structure, generated glyph families, and complete inventories without
  requiring fragile ANSI color comparisons.
- User-facing command help and the relevant CLI/rendering documentation explain
  the flag and its output purpose.
- Run the normal `make test` suite; use advanced terminal/player/browser tests
  only if the implementation touches those paths.

## Verification guidance

Capture representative output for all modes and at least one filtered mode.
Verify that every listed shape is actually accepted by the mode's candidate
set or renderer, that no duplicate glyphs are reported, and that the image
bytes remain unchanged when `--info` is omitted. Check wrapping and terminal
readability for the sextant-heavy modes.

## Unresolved questions

- Should metadata be emitted once per mode after the combined normal/smart row,
  or separately under each normal and smart image?
- Should aliases be accepted only as selectors, or also be shown in the info
  header?
- Should the one-line descriptions and display labels live as additional spec
  fields, or should they be derived from existing renderer/colorer/geometry
  fields until the spec needs richer prose?
- What compact, deterministic notation should represent generated sextant
  masks while still satisfying the requirement to list all available shapes?
