# 014 — More Unicode goodness

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `docs/SparklinePixelArt.md`

---

     x: 	0 	1 	2 	3 	4 	5 	6 	7 	8 	9 	A 	B 	C 	D 	E 	F
U+1FB0x 	🬀 	🬁 	🬂 	🬃 	🬄 	🬅 	🬆 	🬇 	🬈 	🬉 	🬊 	🬋 	🬌 	🬍 	🬎 	🬏
U+1FB1x 	🬐 	🬑 	🬒 	🬓 	🬔 	🬕 	🬖 	🬗 	🬘 	🬙 	🬚 	🬛 	🬜 	🬝 	🬞 	🬟
U+1FB2x 	🬠 	🬡 	🬢 	🬣 	🬤 	🬥 	🬦 	🬧 	🬨 	🬩 	🬪 	🬫 	🬬 	🬭 	🬮 	🬯
U+1FB3x 	🬰 	🬱 	🬲 	🬳 	🬴 	🬵 	🬶 	🬷 	🬸 	🬹 	🬺 	🬻 	🬼 	🬽 	🬾 	🬿
U+1FB4x 	🭀 	🭁 	🭂 	🭃 	🭄 	🭅 	🭆 	🭇 	🭈 	🭉 	🭊 	🭋 	🭌 	🭍 	🭎 	🭏
U+1FB5x 	🭐 	🭑 	🭒 	🭓 	🭔 	🭕 	🭖 	🭗 	🭘 	🭙 	🭚 	🭛 	🭜 	🭝 	🭞 	🭟
U+1FB6x 	🭠 	🭡 	🭢 	🭣 	🭤 	🭥 	🭦 	🭧 	🭨 	🭩 	🭪 	🭫 	🭬 	🭭 	🭮 	🭯

## Prep

## SetIdeas implementation scope — 2026-09-11

Own the experimental sets for [#025](025-spec-driven-render-modes.md) and
[#043](043-compose-named-and-debug-render-modes-from-glyph-set-ids.md), reusing this
existing glyph-expansion ticket rather than filing a duplicate. The historical
sextant phase below is already represented by the current dedicated renderer.

Measured by repository inspection at `33b625b`: the spec has a generated sextant
family but no ninelikes/vmbars/mbars registry entries. The user-authored
[SetIdeas](../docs/SetIdeas.md) proposes these exact inventories (␠ means space):

- 9 ninelikes: ␠ ╺ ╸ ━ ┃ ┣ ┫ ╋ ┳ ┻ ┓ ┏ ┗ ┛ ╻ ╹ ▪ ╏ ╍
- 15 vmbars: ␠ 🬋 🬇 🬃
- 45 mbars: ␠ 🬋 🬇 🬃 ┃ ╻ ╹ ▪ ╏

### Scope, acceptance and verification

- [ ] Define explicit ideal coverage and approximation metadata for every glyph
  using #025's schema; document font-dependent thickness, gaps and fallback.
  Unknown: these shapes have not been established here as exact 3x3 cell masks.
- [ ] Provide a checked-in reproducible glyph probe with named font/tool versions
  and measured coverage errors. Coordinate font research with #009; do not infer
  terminal appearance from a Unicode name alone.
- [ ] Resolve set 15's native thirds versus quad+'s proposed 2x4 approximation:
  exact combination with fourths needs height 12. Specify resampling/error bounds
  if preserving requested 2x4 analysis; keep native and analysis geometry separate.
- [ ] Validate set 9's proposed 3x3 model, including dotted/dashed glyphs. Declare
  approximations or unavailable entries rather than inventing exact masks.
- [ ] Ship sets 9/15/45 through the registry, with debug access to 45 even when
  no named mode uses it. Enable dependent named modes in #043 only once coverage
  and reconstruction tests pass. No automatic experimental default.
- [ ] Test every glyph, bounds, transparency, FG/BG consistency and worker parity;
  assess cati/emojig and geometry fixtures at widths 8–20 with recorded error and
  cost. Add predicted new goldens and update SparklinePixelArt documentation.

Depends on #025 for integration; probes can precede it. Moredots, diagonals,
triangles and higher-resolution experimental families remain outside this delivery.
Run `make test`, `go vet ./...`, `make install`, and `make preflight`; follow the
rendering playbook before any golden update. Preserve SetIdeas unchanged.
- check which fonts support which subset
- document them here (and later in docs/)

## Research
- can we detect the font type used by the current terminal?
- check for main Linux terms

## Make it Optional
- add as new algo
- one algo for 2x3 blocks
- one for triangles and othere geoms
- one for "search for best"

## Phase 1
- ship the dedicated 2x3 sextant mode
- write tests
- add to make demo-xxx cases
- add golden imgs

## Phase 2
- add the remaining geometry families
- repeat the phase 1 verification steps

## FIXME
- keep the sextant geometry tables local for now; consolidate them with later glyph families once the shared scorer stabilizes.
- the current 2x2 mask model is still too coarse for the diagonal-block family. The atlas needs to be treated as a diagonal/line-segment family, not as a plain quadrant mask.
- previous search/scorer experiments were removed from the product surface; keep future diagonal work behind private verification until it passes aspect, gap, and golden-image invariants.

**IMPORTANT** DO NOT BREAK the other algos
**IMPORTANT** Make sure transparent row append works correctly to support 1x1 terminal <-> image asoect matching. 5x5px will always draw 1:1 in the term!
Using halfcells where needed.

## Working Split

- **Main thread**: define the glyph families, geometry sampling model, and mapping order.
- **Subagent**: research practical font and terminal support for the U+1FB00 range, then report which families are safe to prioritize first.

## Glyph Mapping Plan

### 1. Start with the sextant family

The `BLOCK SEXTANT-*` range is the cleanest first step because it is a full 6-bit
coverage family. Treat it as a dedicated `2x3` geometry:

- use a fixed bit order and keep it stable
- score coverage on the same `2x3` sampling grid every time
- prefer exact masks over nearest-neighbour approximations
- add tests for empty, full, single-bit, and corner-adjacent masks

This is the first family worth shipping because it has a compact mask space and
lets us validate the new geometry path without mixing in diagonals.

### 2. Keep diagonal and triangle shapes separate

The diagonal, triangular, and three-quarter block families should not be forced
into the sextant grid. They need their own geometry and sampling rules because
their visible boundary is not row/column aligned.

Recommended split:

- `1FB3C..1FB67`: block diagonals and diagonal composites
- `1FB68..1FB6F`: triangular three-quarter and quarter blocks
- `1FB9A..1FB9F`: triangular half blocks and triangular shade variants

These should likely use a denser mask grid and explicit polygon/half-plane
coverage rather than a simple rectangular bitmask.

### 3. Treat one-eighth blocks as a separate fill family

The vertical and horizontal one-eighth blocks are useful, but they are a
different geometry problem from sextants or triangles. Keep them separate so the
coverage model stays understandable.

Good use cases:

- thin progress bars
- small fill levels
- a future fractional-shade mode

### 4. Leave the line-drawing composites for later

The box-drawing diagonal composites and diamond-like variants are a plausible
later mode, but they should not be the first mapping target. They will need
stronger geometry sampling and font verification before they are worth wiring
into the search path.

### 5. Geometry rules for the other agent

When the other agent evaluates glyphs, the key invariants are:

- preserve source bounds and do not normalize away non-zero origins
- define mask coverage in source-space coordinates, not by terminal cell size
- keep the aspect-ratio correction shared with the existing half-cell logic
- do not collapse all new shapes into the current quad/spark candidate tables
- keep each glyph family isolated until the tests prove the mapping is stable

### 6. Suggested delivery order

1. Sextant renderer and tests
2. Font support report from the research agent
3. Diagonal / triangle geometry sampler
4. Fractional fill family
5. Composite line-drawing family

That order keeps the first commit small, gives us a measurable baseline, and
reduces the risk of mixing geometry bugs with font-coverage problems.
