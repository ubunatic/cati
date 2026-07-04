# 030 — Website assets: out-of-dir references and LFS pointer pitfalls

**Status:** ✅ Closed (2026-07-04)

## Problem

The published site at `ubunatic.com/cati` had a broken hero video:

1. `website/index.html` referenced `../assets/reels/webm/cati-demo.webm`,
   i.e. a path **outside** `website/`. `uman website sync` only copies the
   `website/` dir into the publishing repo, so the path 404'd on the live site.
2. `website/cati.webm` was a stale 1.4 KB stub (likely a truncated/failed reel
   render), unreferenced by the page. Since `uman website sync` never deletes
   files in the publishing repo (see uman issue 003), such stale files persist
   on the live site forever once synced.
3. `*.webm` is LFS-tracked in this repo. Any workflow that copies website
   files from a non-checked-out state (CI, bare clone) would publish LFS
   pointer files instead of media.

## Fix applied

- Copied `assets/reels/webm/cati-demo.webm` → `website/cati-demo.webm` and
  changed the `<source>` src to the relative `cati-demo.webm`.
- Removed the stale `website/cati.webm` stub (the already-synced copy in
  ubunatic.com was removed manually in the publishing repo).

## Rule going forward

**`website/` must be self-contained**: every asset a page references must live
inside `website/` and be referenced by a relative path. No `../` escapes.

## Follow-ups

- [ ] Consider a `make check-website` target (or uman feature) that greps
      `website/index.html` for `src=`/`href=` values escaping the dir.
- [ ] Regenerate `website/cati-demo.webm` when the demo reel changes —
      it is a copy of `assets/reels/webm/cati-demo.webm`, not a link.
