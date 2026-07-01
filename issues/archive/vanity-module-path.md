<!-- SPDX-FileCopyrightText: 2025 Uwe Jugel -->
<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
# Issue: Migrate module path to vanity domain ubunatic.com/cati

Status: Done

## Summary

The Go module is currently declared as `codeberg.org/ubunatic/cati` in `go.mod`.
The vanity redirect at `ubunatic.com/cati` is in place, but `go install
ubunatic.com/cati@latest` requires the module path in `go.mod` to match.

## Motivation

Consistent vanity import paths make the project easier to reference and install
without tying the canonical name to a specific forge. Other projects in this
workspace (`claudeconfig`, `vimconfig`, `mdview`) have already migrated.

## Proposed Solution

1. Update `go.mod`: `module ubunatic.com/cati`
2. Update all internal import paths in Go source files
   (`codeberg.org/ubunatic/cati/...` → `ubunatic.com/cati/...`)
3. Verify with `make test`

## Alternatives Considered

Keep the current module path and only expose the vanity redirect for browsing,
not for `go install`. Ruled out: inconsistent with other projects in the workspace.
