# 110 — Website sample images share one pixel size per comparison group

**Status**: Open
**Priority**: P1 (High)
**Severity**: Minor
**Category**: Feature
**Related**: ubunatic.com 075 (media viewer; M2 adds `viewer: fit`, a later milestone adds an A/B compare slider)

---

## 1. Goal

/goal Each comparison group on the website (an original photo and its half, quad and spark renders)
has one pixel size, produced by a make target from the existing samples, published to ubunatic.com
via `uman website sync cati`; or stop and report when a size choice needs an owner decision.

## 2. Problem (owner, 2026-10-23)

In the site's media viewer, and in a plain file browser, the original and its renders differ in
pixel size, so a comparison jumps in size. Equal sizes also make a future A/B slider trivial.

Measured in `website/` (originals in `assets/samples/`, renders in `testdata/<sample>/`):

| Sample | Original | Renders |
|---|---|---|
| soldering practice | 640×480 | 960×720 |
| summer vacation | 1042×1383 | 600×792 |
| darth daughter | 687×1168 | 600×1032 |

Aspect ratios match within about 1%.

## 3. Rules (owner-approved proposal)

- Target size per group: the larger of original and renders.
- Renders are enlarged pixel-exact (nearest neighbor) so terminal cells stay sharp; photos are
  enlarged smoothly, and only where needed (only the soldering original today).
- Where ratios differ slightly, fit and pad or crop by at most a few pixels; say which.
- Source files under `testdata/` and the original samples stay untouched; only the website copies
  change. One make target regenerates them reproducibly; no hand-edited images.
- The gallery blocks in `website/page.yaml` set `viewer: fit` once ubunatic.com 075 M2 supports it.

## 4. Host refinement (2026-10-23, plan review)

Renders scale by an integer factor only, so every terminal cell stays an exact block (a 1.75×
nearest-neighbor scale makes uneven cells). Rule: `k = max(1, round(original_h / render_h))`,
renders scaled ×k nearest neighbor, the photo resized smoothly (up or down) to the render size ×k,
padding the photo by a few pixels where ratios differ. Resulting targets:

| Sample | k | Target | Photo |
|---|---|---|---|
| soldering practice | 1 | 960×720 | 640×480 up 1.5× |
| summer vacation | 2 | 1200×1584 | 1042×1383 up ≈1.15× |
| darth daughter | 1 | 600×1032 | 687×1168 down ≈0.88× |

Owner decision (2026-10-23): renders are not scaled up; the photo is resized to the render size.
So k = 1 for all groups, and summer vacation becomes 600×792 (photo down ≈0.57×), not 1200×1584.

## 5. Delivered

b0f1de6 and 902f6f1: `make website-samples` regenerates the website copies reproducibly (rerun gives
identical hashes); the three comparison galleries set `viewer: fit` and mark renders `pixelated`.
Host-verified sizes in the synced site: soldering 960×720, summer 600×792, darth 600×1032 for the
photo and all three renders. Synced to ubunatic.com (026be1c); staged for owner review.
