# 075 — Unify CLI render geometry contracts and validate route-specific options

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [#008](008-spaghetti-code-analysis.md), [#036](036-add-smart-render-mode-that-selects-the-best-width-by-psnr.md), [#047](047-smart-render-search-timeout.md), [#071](071-respect-explicit-width-and-height-in-static-renders.md), [#072](072-add-aspect-modes-for-cli-source-mapping.md)

---

## 1. Problem & Motivation
CLI render options have different meanings or are silently ignored across static, playback, interactive, browser, and benchmark routes. A holistic sizing update needs one explicit grid contract and a shared geometry plan before adding further flag-specific branches.

## 2. Technical Specification / Findings
Verified on a generated opaque 32x20 PNG with the current CLI (2026-10-03; output measured after stripping ANSI):

| Static flags | Output cells | Finding |
|---|---|---|
| `-H 3` | 32x10 | Explicit preparation calculates zero target width; resize returns the original image. |
| `-W 8` | 8x3 | Width-only uses aspect fitting. |
| `-W 8 -H 8` | 8x8 | Both dimensions stretch the source rather than fit it inside a box. |
| `-W 8 -H 8 --zoom 0` | 8x3 | Even explicit fit zoom bypasses explicit-grid preparation. |
| `-W 8 -H 3 --zoom 1` | 32x10 | Numeric zoom bypasses the dimension constraints. |
| `-W 8 --aspect aligned` | 32x10 | Aligned width-only also reaches a zero target height. |
| `-W 8 --aspect bogus` | 8x3 | Invalid aspect values are accepted. |
| `-W 8 --zoom bogus` | 8x3 | Invalid zoom values silently fall back to fit. |
| `-W -1` | 32x10 | Negative dimensions are accepted. |

`cmd.NewPlay()` with `--play preview` reproduces height-only failure; `-W 8 --pad bad` succeeds, proving padding is not parsed on that route. Preview with `-W 8 --zoom 1` produces 8x3, proving zoom is ignored there. `NewPlay` and `NewBrowse` declare pad/aspect flags but interactive/browser calls do not receive them; browser-to-player forwarding also omits the prescaler. Playback defaults resolved from a real terminal enter explicit stretch preparation even without explicit dimension flags.

Smart preparation is bypassed by explicit height/aligned/nonempty zoom. Native candidate fitting derives height from a rounded-up whole-column fit and resizes the original to that height, losing the fit's transparent-tail treatment. Search stores all reconstructions before scoring and has neither a candidate count cap nor cancellation. Interactive sizing uses different terminal policies: viewers clamp to physical size and reserve two chrome rows; browser uses independent fallback dimensions and its configured height cap. None of these loops registers SIGWINCH; image/browser refresh size on certain events, interactive video retains its initial size.

Subcell scoring and final emission also need a shared cell partition: reconstruction samples candidate bounds across the chosen cell count, but `padSmartImage` embeds candidate pixels into a larger raster before final glyph fitting. That can change the partition used to select glyphs. Preserve the winning cell grid and add blank terminal cells after fitting, or prove pixel padding preserves the scored partition. The existing ANSI validator checks dimensions inferred from the prepared image, not the original CLI request.

Relevant entry points: `cmd/root.go:503`, `cmd/root.go:741`, `cmd/play.go:40`, `cmd/render_pipeline.go:58`, `cmd/smart_render.go:127`, `cmd/interactive.go:1048`, `cmd/browser.go:1083`.

Policy decision required: #071 requests exact output resolution, #072 requests default aspect-preserving fit, and a bounding-box proposal allows smaller content. Reconcile these by separating output canvas dimensions from content mapping; do not equate an exact canvas with stretching source pixels.

## 3. Implementation & Verification Plan
**Goal**: Parse and validate render options once, preserve explicit/auto provenance, and use a pure shared render plan across applicable command routes. Explicit grids constrain the canvas; source padding, aspect mapping, zoom/pan, renderer boundary alignment, smart search, and final cell cropping have documented roles. Every advertised option must either be implemented on its route or rejected clearly.

Define positive/zero dimension semantics, contain/aligned mapping, final-crop behavior, physical-terminal clipping, and mode-specific partial-cell capabilities before implementation. Keep geometry and smart policy authoritative in the spec, derive raster targets from the same plan, and validate emitted dimensions against the plan rather than the prepared image alone. Smart candidates must preserve the fixed canvas and use identical reference/alpha/crop semantics; bound work and retain a valid base fallback.

Verify all four dimension combinations across static, preview/playback, interactive image/video, browser panes, modes, and benchmark routes; include invalid values, zoom/crop combinations, odd cell boundaries, mode switches, terminal resize, and source images with nonzero bounds. Add numerical aspect checks, emitted-grid assertions, and renderer goldens with PNG metadata for changed mapping behavior. Existing geometry/imgutil suites, targeted CLI/smart tests, and tagged player/browser tests passed during review; they do not cover the full flag interaction matrix. `make preflight` also passed, including installation and `go vet ./...`.
