# Rendering Bug & Golden-Change Playbook

How we diagnose visual/geometry rendering bugs and change golden images **safely**.

> **One rule above all: prove the bug numerically before you touch code, and
> change a golden image only once you can name the wrong pixel and say why.**

Golden PNGs are the project's ground truth for what every render mode produces.
A careless `-update` silently rewrites that truth and hides regressions forever.
This playbook is the discipline that prevents it. It is written from a real fix
(spark/quad garbled bottom row → cross-mode half-cell unification, issue
[#015](../issues/015-spark-bottom-row-halfcell-fit.md)); the steps below are the
exact sequence that worked.

---

## The loop

```
1. Reproduce + isolate   → smallest input that shows it; find a control that does NOT
2. Prove the root cause  → numbers, not screenshots; explain every "good" and "bad" case
3. Predict golden impact → list exactly which goldens change, before editing code
4. Fix the root          → no band-aids; remove earlier band-aids the root fix obsoletes
5. Confirm               → live output + unit invariants + ONLY the predicted goldens differ
6. Update goldens        → only when 100% sure; regenerate-all, revert-all-but-intended
7. Close the loop        → docs + tests + issue in the same commit
```

Each step has a gate: **do not advance until the current step's gate is green.**

---

## 1. Reproduce and isolate

- Find the **smallest** input that reproduces. Shrink width/size until the bug is
  trivially inspectable (e.g. `-w 6` instead of `-w 24`).
- Find a **control that does _not_ reproduce.** In #015, `-w 5` was clean but
  `-w 6` was broken — that contrast is the single most valuable clue, because the
  fix must explain *both*.
- Capture the raw bytes, not just the picture. For ANSI output:
  `cati … | cat -v | tail -1` shows the exact escape sequences and glyphs of the
  offending row. Decode the block characters (`▀ ▌ ▚ …`) — the *choice* of glyph
  is the symptom.

**Gate:** you have a one-line repro command and a one-line near-miss that differ
by a single parameter.

## 2. Prove the root cause — numbers first

Screenshots show *that* it's wrong; only numbers show *why*. Build a tiny probe
that runs the **real** production function (not a reimplementation) over the
repro and the control, and prints the geometry/decision for each.

- Put the probe at `scripts/<name>_probe.go` with `//go:build ignore`, run it
  with `go run`, and **delete it before committing** (it is a diagnostic, not a
  test — the test comes in step 7).
- The proof must explain **every** observed case: each "good" width *and* each
  "bad" width must fall out of the same formula. In #015 the bug was exactly
  `rem ∈ {5,6,7}` and the good cases were `rem ∈ {0,4}` and `rem<4` — the probe
  showed that mapping precisely.
- Look for an **invariant** that should hold but doesn't. #015's was
  `CellW/(AspectX·CellH) = 1/2` for every mode ⇒ identical continuous height ⇒
  the modes *must* agree; the probe proved they didn't and pinpointed the
  floor-before-snap as the only divergence.

**Gate:** a formula predicts the bug for every repro and every near-miss, and you
can state the wrong value in one sentence.

## 3. Predict golden impact — before editing code

With the root cause as a formula, you can compute which inputs hit it. Enumerate
the golden corpus and mark which goldens fall in the affected range **on paper**,
before changing a line. In #015 we computed `rem` for every golden source and
predicted exactly 2 (then 4) goldens would change — and nothing else.

**Gate:** a concrete list of "goldens that will change" and a one-line reason for
each. If your prediction later turns out wrong (more or fewer change), **stop** —
your root-cause model is incomplete; return to step 2.

## 4. Fix the root, not the symptom

- Fix the cause the proof identified, at the layer where it originates (in #015,
  the geometry decision in `FitDims`, not the renderer).
- **Remove band-aids the root fix makes obsolete.** #015 had an earlier
  symptom-suppression patch in `render.go`; the root fix made it dead weight, so
  it was reverted. Leaving both is how a codebase rots.
- Prefer the change that makes a broken case *representable* over one that hides
  it (snap geometry to a valid glyph boundary > suppress a colour after the fact).

**Gate:** the diff touches the layer the proof named, and no compensating hack
remains downstream.

## 5. Confirm — three independent checks

1. **Live output** — re-run the original repro (and the near-miss) against the
   rebuilt binary; the bytes are now clean and the control is unchanged.
2. **Codified invariant** — add/extend a unit test that asserts the *property*,
   not a pixel (e.g. `TestFitDimsUnifiedGeometry`: all modes agree;
   `TestFitDimsHalfCellInvariant`: `extH ∈ {0, CellH/2}`). This is what stops the
   bug from returning.
3. **Golden diff matches the prediction** — run the golden test (without
   `-update`) and confirm the set of failing goldens is **exactly** the list from
   step 3 — no more, no less.

**Gate:** all three green, and the failing-golden set equals the prediction.

## 6. Update goldens — only when 100% sure

You may regenerate a golden only when you can point at the **specific wrong
pixel** in the old one and say why it was wrong. In #015 we showed the old
halfblock/quad goldens had 4 transparent bottom rows (a `▀` half-row) where the
true geometry is a full row — and proved it by reading the alpha channel.

Mechanics (the `-update` flag rewrites **all** goldens, including byte-only
re-encodes):

```sh
# regenerate everything, then keep ONLY the intended files
go test ./cmd/ -run TestGoldenRenders -update
git checkout -- $(git diff --name-only testdata | grep -vE '<intended-file-regex>')
git status --porcelain testdata   # must list exactly the intended goldens
```

Then verify the new goldens encode the *intended* geometry (re-read the pixels),
not just "the test passes now."

> If you are not certain a golden was wrong, **do not touch it.** A red
> `TestGoldenRenders` on a file you can't justify is a signal to keep
> investigating, never a reason to `-update`.

**Gate:** every changed golden has a recorded "which pixel, why" justification,
and a human approved the change for visual goldens.

## 7. Close the loop — same commit

- Update the relevant evergreen doc (for render-algorithm changes,
  [SparklinePixelArt.md](SparklinePixelArt.md) / [QuadPixelArt.md](QuadPixelArt.md))
  in the **same logical step**, not at the end.
- Record the bug, root cause, and golden impact in `issues/` with a `Refs #NNN`
  in the commit.
- Commit message states the root cause and names the regenerated goldens and why.

**Gate:** `go vet ./...`, `make preflight`, and `go test ./...` are green, and the
commit carries code + tests + docs + golden justification together.

---

## Anti-patterns this playbook exists to prevent

- **`-update` to make the suite green.** That doesn't fix a bug; it ratifies it.
- **Screenshot-driven fixing.** "Looks better now" is not a root cause and won't
  survive the next aspect ratio.
- **Band-aid stacking.** Suppressing a symptom downstream of an unfixed cause
  leaves two things to misunderstand later.
- **Reimplementing the function in the probe.** Prove against the real code path,
  or you prove nothing about production.
- **Fixing without a near-miss.** If you can't point at a similar input that
  works, you don't yet understand the boundary.
