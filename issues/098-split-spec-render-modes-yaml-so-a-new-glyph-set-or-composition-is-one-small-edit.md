# 098 — Split spec/render_modes.yaml so a new glyph set or composition is one small edit

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: [#025](025-spec-driven-render-modes.md), [#043](043-compose-named-and-debug-render-modes-from-glyph-set-ids.md), [#090](090-support-parameterized-vector-mode-geometries-vec2-vec8.md)

---

## 1. Problem & Motivation
`spec/render_modes.yaml` is 693 lines with eleven top-level sections. Adding one glyph
set and a mode that uses it touches up to seven places in that file:

| Section | Lines | What a new set/composition needs there |
|---|---|---|
| `modes:` | ~160 | only for renderer-backed modes |
| `glyph_sets:` | 15 | name → glyphs, for renderer-backed modes |
| `set_registry:` | ~115 | id, name, geometry, glyphs/masks or `generated:` |
| `compositions:` | ~65 | name → set IDs |
| `composition_order:` | 1 | the name again |
| `composition_info:` | ~20 | the description, keyed by name |
| `experimental:` | 1 | the name again, if experimental |
| `cycle:` | 17 | the name again, if it is in the cycle |
| `vector_shapes:` | ~270 | shape geometry shared by vec2…vec8 |

Two things make this hard: the same set is described twice (`glyph_sets:` by name and
`set_registry:` by ID), and one composition is spread over four parallel lists keyed
by name, so a typo or a forgotten list is easy.

## 2. Proposal
Split by what changes together. One file per glyph set, one record per composition.

```
spec/
  render_modes.yaml          # modes:, renderers:, colorers:, smart:, cycle:
  compositions.yaml          # one record per composition; file order = composition order
  glyph_sets/
    00-fulls.yaml            # one set per file: id, name, geometry, glyphs + masks | generated
    01-sides.yaml
    ...
    56-vec6.yaml             # `generated: vector_linear_splits`
  glyph_shapes/
    vector.yaml              # today's vector_shapes: (points, glyphs, hand-drawn masks)
  schemas/
    glyph_set.schema.json    # one schema for every file in glyph_sets/
    compositions.schema.json
    glyph_shapes.schema.json
```

`compositions.yaml` replaces four lists with one record each:

```yaml
compositions:
  - name: 2x3
    aliases: ["6"]           # if any; today aliases are duplicate composition keys
    sets: [fulls, sides, sextants]   # names, not IDs (IDs stay in the set files)
    info: "Uses full cells, sides and native sextants at 2x3 geometry."
    experimental: false
```

Rules:
- `glyph_sets:` (name map) merges into the set files; renderer-backed modes refer to
  sets by the same names. One description per set.
- The file name prefix is the ID for sorting only; the `id:` field stays authoritative,
  and a test checks they agree and IDs are unique.
- `LoadRenderModes()` keeps its signature and returns the same `RenderModesSpec`, so
  callers do not change; only the loader reads several files (`fs.Glob`). The embed
  pattern in `spec/spec.go` must add the two directories.
- Loader contract (docs/Design.md §3.1) holds per file: missing directory → empty
  inventory; a present but invalid file → load error naming the file.

Adding a new set then means: add `glyph_sets/NN-name.yaml`, add one record to
`compositions.yaml`. Nothing else.

## 3. Open questions
- Whether `cycle:` should become a `cycle: true` flag on the composition/mode record,
  or stay a list because its order differs from the composition order (it does today:
  `2x4` comes first in `cycle:`).
- Whether aliases should move onto the records, which removes the duplicate keys
  (`vector`/`v`/`vec` → `vec6`, `oct` → `octant`) from `compositions:`.
- The working tree had uncommitted edits to `spec/render_modes.yaml` and `cmd/modes.go`
  when this was filed (2026-10-08); land or drop those first.

### Decisions (host, 2026-10-08)
- `cycle:` stays a separate ordered list in `render_modes.yaml` (its order differs).
- Aliases move onto the composition records (`aliases: [vector, v, vec]` on `vec6`,
  `[oct]` on `octant`); the loader expands them so `RenderModesSpec.Compositions` and
  every CLI name resolve exactly as before.
- Do not stage or overwrite other sessions' uncommitted edits in `cmd/modes.go` /
  `spec/render_modes.yaml` (2x4 `[0, 1, 8]` → `[0, 8]`, issue 097 work).

### M1 Pre-Work (host review of dev098 plan)
- Keep the renderer name map `glyph_sets:` in `render_modes.yaml` as mode data; it is
  not the same as the registry (`half` includes `█`, set `halves` does not). No new
  "compatibility view" layer. Only `set_registry:` moves to `glyph_sets/`.
- Work in an isolated worktree of HEAD on a branch. Integration into the main tree:
  `git stash push -- spec/render_modes.yaml`, fast-forward `main`, then re-apply the
  other session's change as an **uncommitted** edit in `compositions.yaml`
  (2x4 sets `[fulls, brailles]`) and drop that stash. Never touch `cmd/modes.go`.
- Milestones: M1 = baselines + split + loader + schemas + tests, committed.
  M2 = docs + full verification + integration + close.

## 4. Implementation & Verification Plan
/goal Split the render-mode spec as proposed (adjusted by the open questions), with no
change in behaviour, and verify it — or stop and report when blocked on a user decision
or denied permission.

Done when:
- `cati modes glyphs` output for every composition and set is byte-identical before
  and after (capture it first), and no golden changes.
- Schemas cover every new file; spec integrity tests check: unique IDs, file prefix =
  ID, every composition set name exists, every set is used by some composition or mode.
- `docs/Design.md` §3.1 file map, `docs/Spec.md` if needed, and
  `docs/SparklinePixelArt.md` references to `render_modes.yaml` sections are updated.
- `go vet ./...`, `go test ./...`, `make preflight` pass.

Re-check the live spec before starting: sections and line counts above date from
2026-10-08.
