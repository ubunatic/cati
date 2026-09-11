# 043 — Compose named and debug render modes from glyph set IDs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [#025](025-spec-driven-render-modes.md), [#014](014-more-boxdrawing-chars-unicode-v13.md), [SetIdeas](../docs/SetIdeas.md), [#042](042-add-info-metadata-to-cati-modes-demos.md)

---

## Problem and measured baseline

At `33b625b`, the spec exposes eight manually enumerated modes: half,
half/split, quad, spark, spark+quad, six, six+half, spark+six. Each chooses a
fixed renderer. `cmd/modes.go` filters registered names and prints spec metadata;
there is no set-ID debug grammar in that command. #042 implemented descriptions
and inventories; extend its completed behavior instead of reopening it.

The user wants human-friendly modes built from reusable sets, arbitrary debug
unions, and a deliberate migration from these bundles. SetIdeas is user-authored
and currently untracked; this ticket captures the essential target independently.

## Target and dependencies

Depends on #025's resolver; experimental modes additionally depend on #014.
Keep mode definitions, aliases, cycle order, descriptions and defaults in spec.

## Pre-implementation baseline and resolutions — 2026-09-11

Baseline at `b8df8a0`: `go test ./...` passes. `docs/SetIdeas.md` is a
pre-existing untracked user file and is preserved byte-for-byte (SHA-256
`d88598cbf1e13e61b29f7b4bed9020f21d045e7db74a61ce131f193ffd272ea0`). The
repository currently has eight spec-defined renderers, string-keyed glyph
sets, and case-folding mode lookup; it has no set-ID resolver or debug
expression grammar.

The following decisions are binding for this implementation:

* Set 0 is the full-cell baseline. `half` means the top/bottom interpretation
  and therefore resolves to `[0,2]`, not the sides `[0,1]` suggested by the
  shorthand table in SetIdeas. The latter is retained as the `sides` set.
* Debug expressions are `d` or `d<comma-separated-set-IDs>`. `d` means `[0]`;
  `d1` means `[0,1]`; `d2` means `[0,2]`; `d1,6,9,44` means `[0,1,6,9,44]`.
  Tokens are decimal IDs, whitespace is not accepted inside an expression,
  set 0 is always inserted, duplicates are removed, and IDs are sorted
  numerically. Empty tokens, signs, non-decimal text, and unknown IDs are
  errors. Named expressions use exact, case-sensitive names and aliases.
* Union grammar is intentionally narrow and unambiguous: a composed name is a
  `+`-joined sequence of registered names or set IDs, with the complete
  registered name tried first. Thus `quad+` and `bars+` are literal names,
  while `quad+six` is a union. Debug `d...` is the only numeric syntax; a
  numeric mode alias is never interpreted as a raw set ID.
* The new canonical `six` family is the 2x6 union family; the native sextant
  family remains available as `2x3` (with `x` retained as its compatibility
  alias for the existing renderer until the registry-backed dispatcher can
  safely replace it). The `3x3` name reports its resolved union geometry,
  not geometry inferred from the name.

### Legacy migration

| Existing canonical name | Existing aliases | Migration decision |
|---|---|---|
| `half` | `h` | Retain; registry-owned `[0,2]`. |
| `half/split` | `hs`, `split` | Retain as compatibility renderer name. |
| `quad` | `q` | Retain; `q` remains case-sensitive. |
| `spark` | `s` | Retain. |
| `spark+quad` | `sq`, `qs` | Retain; trailing `+` is never parsed as composition. |
| `six` | `x` | Retain as a compatibility alias for native sextant rendering during this migration; the registry’s 2x6 family is exposed distinctly. |
| `six+half` | `xh`, `hx` | Retain. |
| `spark+six` | `sx`, `xs` | Retain. |
| historical `halfblock`, `quad/splithalf`, `quad/edge-snap`, `spark/quad`, `spark/best`, `sextant`, `sextant/2x3` | historical spellings | Intentionally rejected by the current CLI surface; they remain documented here so failures are explicit rather than silent aliases. |

Experimental sets 9, 15, and 45 remain registry/debug-only until #014 proves
their coverage and reconstruction contracts. Set 14/44/86/88 metadata may be
listed and resolved, but no named mode silently claims experimental rendering.

### Implemented slice — 2026-09-11

The spec-owned registry and resolver are implemented. CLI `modes --info` lists
the registry compositions and debug expressions, reports normalized IDs,
approximation status, and LCM geometry, while ordinary render demos continue
using the unchanged legacy backends. Library callers can use
`spec.ResolveGlyphSetExpression` directly. Registry-only unions intentionally
return a clear “no safe renderer” error when selected for image rendering;
player/browser dispatch therefore remains on the proven named renderer set.
This is a dependency follow-up for the remaining executable arbitrary-union
dispatcher, mask coverage, reconstruction, and new-mode golden work.

### Registry repair milestone — 2026-09-11

The resumed sprint reproduced the expected red baseline: `go test -count=1
./spec ./cmd` failed only because `d1` still expected the old, incorrect 1x2
geometry after sides became 2x1. The registry now carries validated row-major
coverage masks. Every explicit inventory must have one correctly-sized binary
mask per glyph; generated sextant and Unicode-bar inventories have named,
validated generators. Composition references, IDs, set names and listing order
are validated while loading the embedded spec.

The four three-quarter quadrant masks were corrected (`▛=1110`, `▜=1101`,
`▙=1011`, `▟=0111`). Experimental sets 9, 15 and 45 retain an explicit
approximation marker and handpicked masks; set 15 uses 2x4 coverage so its split
middle bars remain distinguishable. Sets 86 and 88 use exact 8x8 bar semantics.
Numeric aliases `1`, `2` and `4` now resolve as mode aliases, with `2` following
the binding half decision `[0,2]`. `docs/SetIdeas.md` remains untouched at SHA-256
`d88598cbf1e13e61b29f7b4bed9020f21d045e7db74a61ce131f193ffd272ea0`.

| Short | Name | Set IDs |
|---|---|---|
| f, 1 | full, 1x1 | 0 |
| h, 2 | half, 1x2 | 0,2 (see ambiguity below) |
| q, 4 | quad, 2x2 | 0,1,2,4 |
| Q | quad+ | 0,1,2,4,14,15 |
| b | bars | 0,1,2,4,44 |
| B | bars+ | 0,1,2,4,86 |
| x | six | 0,1,2,4,6 |
| 6 | 2x3 | 0,1,6 |
| 9 | 3x3 | 0,1,6,9 |
| a | all | 0,1,2,4,6,9 |
| A | all+ | 0,1,2,4,6,9,44 |
| z | z | 0,1,2,4,6,9,86 |
| Z | z+ | 0,1,2,4,6,9,88 |

## Acceptance criteria

- [ ] Resolve names and aliases consistently in CLI, modes demo/listing, library,
  player and browser. Preserve case: q/Q, b/B, a/A and z/Z are distinct.
- [ ] Support `d` = [0], `d1` = [0,1], `d2` = [0,2], and comma-separated
  expressions such as `d1,6,9,44`, always including set 0. Normalize repeated
  IDs and ordering, reject unknown IDs and malformed/empty tokens with useful errors.
- [ ] Define composed-name grammar against registered names/sets; support union
  composition without mistaking the literal suffix in `quad+` or `bars+` for a
  missing operand. Record an unambiguous grammar and test it before shipping.
- [ ] Keep half as the default. Distinguish omitted mode filters (list all modes)
  from explicit `all` (render the named union). Numeric modes are aliases, never
  raw set IDs outside debug syntax.
- [ ] Record a complete migration table for every existing canonical name and
  alias. Decide compatibility aliases versus intentional removals, including
  `spark`, `spark+quad`, `six+half`, and `spark+six`. Explicitly document that new
  `six` is 2x6 while `2x3` preserves the native family. No silent alias collisions.
- [ ] `cati modes --info` derives descriptions, sets, unique shapes and actual
  geometry from resolved modes. Preserve filtering, width, smart comparison and
  `--smart -w 0 --info` listing behavior. Debug modes must be inspectable too.
- [ ] Update completion/help, cycle controls, specs/schemas, library docs, demos,
  README and relevant evergreen rendering docs together.

## Design ambiguities to resolve explicitly

SetIdeas labels `1x2` as half but references [0,1] (sides); [0,2] matches the
stated half intent. Confirm and record that interpretation before implementation.
The name `3x3` deliberately describes a family but its union is 6x3; expose actual
geometry rather than inferring it from the alias. `quad+` requests approximate
2x4 coverage for sextant-derived set 15; #014 must define that approximation.
Set 86 omits left/right hairline widths, despite wording about vertical glyphs;
the explicit glyph inventory is the target. Do not rewrite SetIdeas to hide these
questions. Exact geometry follows #025; font appearance remains approximate.

## Verification

Table-test every name/alias and debug expression, including case collisions,
malformed expressions and union equivalence. Test CLI and library consistency,
all-versus-omission listing, no-image listing, smart centering and width/row
invariants at widths 8–20 with cati/emojig plus geometric/transparent fixtures.
Run player/browser integration checks from docs/Testing.md where dispatch changes.
Add new-mode goldens with descriptive metadata; explain old golden changes before
regeneration, preserving both JPEG toolchain families. Run `make test`,
`go vet ./...`, `make install`, `make preflight`, and Harnez tracker validation.
