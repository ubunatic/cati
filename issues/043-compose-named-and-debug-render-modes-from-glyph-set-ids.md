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
