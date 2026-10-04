# 081 — Replace file:///home links in docs and book with repo-relative or Codeberg URLs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Documentation
**Related**: uman issue 032 (leak scan), ubunatic.com issue 054

---

## 1. Problem & Motivation

`uman website scan cati` blocks `uman website sync` with 2 `file-url` findings in `website/book/RenderPipelines.html:204` and `website/book/print.html:439`. They are generated from `file:///home/uwe/projects/cati/...` links in `docs/RenderPipelines.md` (7 links) and `docs/perf/2026-09-24-halfblock-direct-rgba-fastpath.md` (3 links); issues 044-046 contain more. The username is fine (owner policy), but the links point at files on the author's disk and are dead for public readers of the book.

## 2. Proposed fix

- Replace each `file:///home/uwe/projects/cati/<path>` link with a repo-relative link (inside docs) or `https://codeberg.org/ubunatic/cati/src/branch/main/<path>`.
- Regenerate the book/website, then run `uman website scan cati`; expect no `file-url` findings.
- Optional: lint docs for `file:///` in the docs build.

## 3. Acceptance

- `uman website scan cati` reports clean.
- Links in RenderPipelines on the published site resolve.
