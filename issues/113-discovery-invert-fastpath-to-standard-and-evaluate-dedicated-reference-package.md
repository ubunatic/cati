# 113 — discovery: invert fastpath to standard and evaluate dedicated reference package

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Architecture
**Related**: 111 (test catalog roadmap), 054 (PR 8/9 review), 055 (luma fastpath drift), 094 (parity), 102 (dedup fastpaths), docs/Testing.md

---

/goal Reassess and explore disentangling the "fastpath" vs "standard/fallback" rendering paths across the library and CLI: invert the terminology and architecture so that the optimized path is the standard default and the naive path is the "reference" path (evaluating a dedicated `reference` package vs per-package reference implementations), keep reference checks fast and isolated without ballooning test durations on large assets, and de-emphasize internal fastpath knobs/flags so end-users and library consumers interact only with the clean, optimized standard; or stop and report when blocked on a user decision.

## 1. Problem & Motivation
Currently, `cati` maintains a split between the "standard path" and "fastpath":
- The fast path (`core.Fastpath`, gated by `CATI_FASTPATH` and `--fastpath` flags) contains optimized rendering (direct `Pix` memory access, append-based ANSI builders, lookup tables).
- The non-fast path is treated as the "default/standard" fallback in name, even though in practice `core.Fastpath` defaults to `true`.
- In `Makefile`, `make test` runs the entire test suite *twice* (once default `CATI_FASTPATH=1`, once `CATI_FASTPATH=0` with `-tags=fastpath0`), doubling test execution time and CPU load across 28 packages.
- CLI flags (`--fastpath`, `--fastpath=false`) and environment variables expose this internal architectural distinction prominently to users and consumers who should simply receive the standard, optimized behavior out of the box.

## 2. Architectural Vision & Discovery Questions

### Architectural Vision:
1. **Terminology Inversion**:
   - The current "fastpath" is the production **standard path**. Users and library callers should only know and interact with this path.
   - The old naive/fallback path becomes the **reference implementation** (clean, naive, easy-to-read algorithms using simple for-loops and straightforward if-then logic).
2. **Dedicated Reference Package vs Per-Package Reference**:
   - *Option A (Dedicated package)*: A standalone package (e.g. `ubunatic.com/cati/v1/reference` or `internal/reference`) housing naive baseline implementations for each mode (halfblock, quadblock, sextant, sparkline). Tests compare standard implementations against this reference using small, fast fixtures to verify baseline correctness.
   - *Option B (In-package reference)*: Keeping reference routines within each respective package (`v1/quadblock`, `v1/halfblock`, etc.) as unexported test helpers or isolated files (`reference_test.go`), avoiding runtime dispatch branches (`if core.Fastpath ...`) inside production code.
3. **Test Decoupling**:
   - Reference implementations are slow by design; they should **never** be run on large test assets or video files during routine test sweeps.
   - Eliminate the blunt full-suite double run (`CATI_FASTPATH=0 go test ./...`). Instead, run dedicated differential parity tests on small synthetic inputs (e.g. 8x8 or 16x16 pixel blocks) to prove that the standard optimized algorithm matches the naive reference.
4. **Surfacing & Clean API**:
   - Deprecate or hide `--fastpath` from top-level CLI help and public library configuration. Callers should not have to think about reference vs fast paths.

### Discovery Scope for Advisor/Investigation:
1. **Entanglement Audit**:
   - Audit `v1/halfblock`, `v1/quadblock`, `v1/sextant`, `v1/sparkline`, `internal/metrics`, and `cmd/`.
   - Where are `core.Fastpath` branches embedded, and how tightly coupled are they with data structures?
2. **Dedicated Reference Package Feasibility**:
   - Evaluate trade-offs between a single `reference` package vs package-local reference implementations in test files.
   - What shared types (geometry, ANSI, color) are needed by a naive reference implementation?
3. **Test Suite Impact**:
   - Quantify time saved by removing the full `fastpath0` tag pass from `make test` and replacing it with sub-second differential unit tests.

## 3. Implementation & Verification Plan
1. **Advisory Discovery**:
   - Produce a structured assessment of fastpath coupling across `v1/` and `internal/`.
   - Propose the preferred design (dedicated `reference` package vs test-only reference implementations).
2. **Prototyping / Migration**:
   - Decouple production render paths from `core.Fastpath` branching.
   - Create naive reference algorithms and differential parity tests on small fixtures.
3. **CLI & Documentation Clean-up**:
   - Remove or deprecate `--fastpath` CLI flag prominence.
   - Update `docs/Testing.md` and `docs/GoLibrary.md`.
