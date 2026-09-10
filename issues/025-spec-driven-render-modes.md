# 025 — Spec-driven render modes, glyph families, geometry, and colorers

**Status:** 🔄 In Progress  
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Refs:** [009](009-explore-more-sparkline-rendering-modes.md), [014](014-more-boxdrawing-chars-unicode-v13.md), [021](021-golden-storage-resolution-all-algos.md), [docs/Spec.md](../docs/Spec.md), [docs/System.md](../docs/System.md)

## Summary

## Current implementation scope — 2026-09-11

This section supersedes the historical target tables and incomplete checklists
below. Implement the reusable set registry and normalized rendering contract from
[SetIdeas](../docs/SetIdeas.md). User-authored design input is currently untracked;
preserve it and arrange its inclusion separately before relying on that link in a
fresh checkout. Named modes and migration belong to [#043](043-compose-named-and-debug-render-modes-from-glyph-set-ids.md);
experimental mask definitions belong to [#014](014-more-boxdrawing-chars-unicode-v13.md).

### Measured baseline

At `33b625b`, `spec/render_modes.yaml` defines eight modes and six string-keyed
glyph sets. `spec/load.go` uses `map[string][]string` for glyph sets; modes carry
string references and explicit cell/optional analysis dimensions. The spec is
already loaded, but `v1/sparkline/render.go` still selects Go candidate bundles in
`FindBestCell` and geometry in `defaultCellGeometry`. `cmd/modes.go` separately
expands glyph inventories, including a generated sextant marker. A declarative
list therefore does not yet drive arbitrary candidate composition.

### Scope and acceptance

- [ ] Define schema-validated reusable sets with stable IDs and plural names:
  0 fulls, 1 sides, 2 halves, 4 quads, 6 sextants, 14 vbars, 44 bars,
  86 morebars, 88 allbars. Reserve 9 ninelikes, 15 vmbars and 45 mbars for #014.
  Copy the exact inventories from SetIdeas; visible `␠` denotes actual space.
- [ ] Store explicit native coverage geometry, glyph identity, and exact versus
  approximate coverage metadata. IDs are identifiers, not geometry encodings.
  Go owns scoring and executable mask operations; YAML owns set membership and
  declared contracts, without shadow tables.
- [ ] Normalize unions deterministically: associative, commutative, idempotent,
  duplicate-free coverage with a stable representative/tie policy. Keep empty
  and full coverage distinct even if ANSI can emit both using a colored space.
  Complement masks are not interchangeable unless FG/BG and transparency
  semantics are preserved and tested.
- [ ] Derive exact common grids by per-axis LCM. Validate bounds and resource
  limits before allocating. Keep sampling geometry, analysis geometry, and
  terminal aspect correction distinct. Approximation must be declared explicitly;
  never silently round a thirds-based mask to fourths.
- [ ] Drive candidate selection, ANSI output, image reconstruction and reported
  inventory from the same resolved union. Preserve optimized existing renderers
  only where their candidate/coverage contract demonstrably agrees.
- [ ] Expose a usable library resolution/rendering path without requiring cmd
  imports. Test zero-value/default behavior and retain documented old APIs.

### Dependencies and verification

Foundation for #043; standard sets can ship before #014. Coordinate shared
geometry with #013/#008 without expanding this into their quality/viewport
refactors. Test registry schema, unknown IDs, duplicate IDs/names, invalid grids,
missing-spec behavior, all union laws, exact half/quad/sextant mask coverage,
and deterministic ordering. Exercise cropped/odd/transparent fixtures, ANSI versus
reconstruction and serial/worker parity. Prove dimensions at widths 8–20 and
benchmark high-grid unions. Run `make test`, `go vet ./...`, `make install`, and
`make preflight`; update Spec, GoLibrary, SparklinePixelArt and RenderPipelines
documentation. Predict any golden changes before regeneration per the playbook.

## Historical implementation notes

Render modes are currently defined in Go across the mode registry, CLI parser,
renderer implementations, tests, and docs. The next render-mode work should move
the stable mode contract into `spec/` as YAML so new glyph families and geometry
combinations can be added without scattering names, aliases, cell geometry, and
candidate sets across the codebase.

The spec should remain declarative. Go should still own executable renderer
implementations, scoring math, color quantization, transparency penalties, and
performance details.

## Status Quo

Current exposed render cycle:

```text
halfblock -> quad/splithalf -> quad/edge-snap -> spark/quad -> spark/best -> sextant/2x3
```

Current CLI aliases:

| Aliases | Current target |
|---|---|
| `h`, `half`, `halfblock` | `halfblock` |
| `qs`, `quad`, `quad/splithalf`, `splithalf` | `quad/splithalf` |
| `qe`, `quad/edge-snap`, `edge-snap` | `quad/edge-snap` |
| `sq`, `spark`, `spark/quad` | `spark/quad` |
| `sb`, `spark/best` | `spark/best` |
| `xs`, `sextant`, `sextant/2x3` | `sextant/2x3` |

Current glyph families in use:

| Current mode/family | Cell geometry | Glyphs |
|---|---:|---|
| `halfblock` | `1x2` | ` `, `▀`, `▄`, `█` |
| `quad/splithalf`, `quad/edge-snap`, and other quad algorithms | `2x2` | ` `, `▘`, `▝`, `▖`, `▗`, `▀`, `▄`, `▌`, `▐`, `▚`, `▞`, `▛`, `▜`, `▙`, `▟`, `█` |
| `spark/vert` package mode | `4x8` | `▁`, `▂`, `▃`, `▄`, `▅`, `▆`, `▇`, `█` |
| `spark/quad` | `4x8` | `spark/vert` plus quad glyphs |
| `spark/sextant` package mode | scorer candidate set | 60 U+1FB00 sextant glyph candidates |
| `spark/best` | scorer candidate set | `spark/quad` plus 60 sextant candidates |
| `sextant/2x3` | `2x3` | 60 native sextant glyphs plus `▌` and `▐` for Unicode-missing full-column masks |

Important catch: `six`/`sextant` must keep `▌` and `▐` as quasi-2x3 column
glyphs. Unicode does not provide native U+1FB00 glyphs for the pure left-column
and right-column sextant masks, and the current dedicated sextant renderer maps
those two masks to the existing half-block column characters.

## Target Mode Surface

The intended user-facing mode set is:

| Name | Aliases | Cell | Analysis | Glyph contract |
|---|---|---:|---:|---|
| `half` | `h` | `1x2` | omitted = `1x2` | ` `, `▀`, `▄`, `█` |
| `half/split` | `hs` | `2x2` | omitted = `2x2` | ` `, `▀`, `▄`, `▌`, `▐`, `█` |
| `quad` | `q` | `2x2` | omitted = `2x2` | half + side + all quadrant glyphs |
| `spark` | `s` | `4x8` | omitted = `4x8` | half + side + full/space + vertical and horizontal fractional fills |
| `six` | `x` | `2x3` | omitted = `2x3` | native sextant/quasi-2x3 set, including `▌` and `▐` |
| `six+half` | `xh` | `2x6` | omitted = `2x6` | `six` plus half-block split candidates |
| `spark+quad` | `sq` | `4x8` | omitted = `4x8` | `spark` plus quad glyphs |
| `spark+six` | `sx` | `4x24` | omitted = `4x24` | `spark` plus `six` glyphs on a common analysis grid |

Notes:

- `quad` can stay at `2x2`; it does not need a `4x4` layout.
- `six+half` uses `2x6`, the least common multiple grid of `six` (`2x3`) and
  `half` (`1x2`).
- `spark+six` uses `4x24`, the least common multiple grid of `spark` (`4x8`)
  and `six` (`2x3`).
- The `analysis` field should be optional and default to `cell`. It exists for
  future cases where layout geometry and scoring geometry differ.

## Proposed Spec Shape

Add a new spec file, likely `spec/render_modes.yaml`:

```yaml
modes:
  - name: half
    aliases: [h]
    renderer: halfblock_exact
    cell: { w: 1, h: 2 }
    glyph_sets: [half]
    colorer: top_bottom

  - name: half/split
    aliases: [hs]
    renderer: mask_scorer
    cell: { w: 2, h: 2 }
    glyph_sets: [half, side]
    colorer: fg_bg_sse

  - name: six+half
    aliases: [xh]
    renderer: mask_scorer
    cell: { w: 2, h: 6 }
    glyph_sets: [six, half]
    colorer: fg_bg_sse

glyph_sets:
  half: [" ", "▀", "▄", "█"]
  side: ["▌", "▐"]
  quad: ["▘", "▝", "▖", "▗", "▚", "▞", "▛", "▜", "▙", "▟"]
  spark_vertical: ["▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"]
  spark_horizontal: [] # fill with left/right 1/8..full block chars
  six:
    generated: sextant_2x3_with_columns
```

Colorers belong in the spec as named strategies, not implementation details.
Good spec-level names include:

- `top_bottom`
- `left_right`
- `two_color_nearest`
- `fg_bg_sse`
- `luma_split`
- `edge_snap`
- `pca2`

Go should keep the actual algorithms: PCA math, k-means iterations, SSE weights,
transparent-pixel penalties, neighbor blending, and fallback decisions.

## Implementation Tasks

- [x] Add `spec/render_modes.yaml`.
- [x] Add typed loader support in `spec/` using `gopkg.in/yaml.v3`, following the current spec-loader conventions.
- [x] Add spec integrity tests:
  - every mode has a unique name
  - every alias is unique and resolves
  - every mode has positive `cell.w` and `cell.h`
  - omitted `analysis` defaults to `cell`
  - explicit `analysis` has positive dimensions
  - every referenced `glyph_set`, `renderer`, and `colorer` is known to Go
  - cycle order contains only defined modes
- [x] Replace hardcoded CLI alias parsing with spec-backed mode lookup.
- [x] Replace hardcoded render cycle names with spec-backed cycle order.
- [x] Keep `renderCfg{}` as the default `half`-compatible mode identity, or migrate carefully with tests that preserve zero-value behavior.
- [x] Implement `half/split` as a `2x2` mode using only half + side glyphs.
- [x] Rename/display `halfblock` as `half` and remove old mode aliases from the accepted CLI surface.
- [x] Rename/display `quad/splithalf` or the intended default quad implementation as `quad` / `q`.
- [x] Rename/display `spark/quad` as `spark+quad` / `sq`.
- [x] Rename/display `sextant/2x3` as `six` / `x`.
- [x] Rename/rework `spark/best` as `spark+six` / `sx`.
- [x] Add `six+half` / `xh`.
- [x] Add/verify horizontal fractional block glyph masks for `spark`.
- [ ] Update docs:
  - `docs/System.md`
  - `docs/SparklinePixelArt.md`
  - `docs/QuadPixelArt.md`
  - `docs/GoLibrary.md` if public mode names change
- [ ] Update golden image metadata names and expected mode list. Do not regenerate goldens blindly; follow `docs/RenderingBugPlaybook.md`.
- [ ] Update CLI, interactive, browser, and line-width tests for the new mode names and aliases.

## Risks / Open Questions

- `six+half` and `spark+six` now use the requested `2x6` and `4x24` cell
  geometry with rational fit specs (`2:3` and `1:3`) so square sources keep the
  same terminal row count as the other modes. The older integer `viewgeom.Spec`
  surface still exists for interactive zoom/pan math and should be consolidated
  with `V2Spec` when the geometry split is removed.
- Renaming modes will touch tests, golden metadata, docs, demo scripts, and user
  muscle memory. The accepted CLI mode surface intentionally keeps only the
  consistent canonical aliases from `spec/render_modes.yaml`.
- Golden and ANSI fixtures still need deliberate update/review for the changed
  spark-family contracts. Current non-golden mode/spec/aspect tests pass.

## Status

🔄 In Progress
