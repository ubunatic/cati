# Split CLI, Player, and Browser Binaries

**Status:** In Progress

## Problem

The primary `cati` CLI, interactive media player, and browser currently live in
one command package and one default test surface. That makes player/browser
features slow down work on the public rendering library and the main static CLI.

## Decision

Split the installed entry points:

- `cmd/cati`: primary static renderer CLI.
- `cmd/catiplay`: media/image player.
- `cmd/catibrowse`: file browser with preview.

`cati play` forwards to `catiplay`. `cati browse` forwards to `catibrowse`.
Legacy `cati --play` and `cati -i` remain compatibility aliases during the
migration and forward to the new binaries.

Prefer `catibrowse` over `catilbrowse` for spelling consistency.

## Test Scope

Default verification should focus on core renderers and the `cati` CLI.
Player/browser integration tests should move behind explicit Go build tags.
All binaries should still compile and install by default.

## Migration Notes

The first step establishes binary boundaries and subprocess forwarding while
preserving behavior. A later step should move shared player/browser dependencies
behind the public Cati library API so the binaries no longer share unexported
command-package implementation.
