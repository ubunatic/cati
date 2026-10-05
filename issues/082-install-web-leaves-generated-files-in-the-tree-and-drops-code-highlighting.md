# 082 — install-web leaves generated files in the tree and drops code highlighting

**Status**: Closed — Built docs in a scratch tree; GOWORK=off GOTOOLCHAIN=go1.26.0 go vet ./...
GOWORK=off GOTOOLCHAIN=go1.26.0 go test ./...
?   	ubunatic.com/cati	[no test files]
ok  	ubunatic.com/cati/cmd	(cached)
?   	ubunatic.com/cati/cmd/cati	[no test files]
?   	ubunatic.com/cati/cmd/catibrowse	[no test files]
?   	ubunatic.com/cati/cmd/catiplay	[no test files]
?   	ubunatic.com/cati/examples/imgbrowser	[no test files]
ok  	ubunatic.com/cati/examples/mediabrowse	(cached)
ok  	ubunatic.com/cati/internal/audio	(cached)
ok  	ubunatic.com/cati/internal/imgutil	(cached)
ok  	ubunatic.com/cati/internal/input	(cached)
ok  	ubunatic.com/cati/internal/metrics	(cached)
ok  	ubunatic.com/cati/internal/pixelart	(cached)
ok  	ubunatic.com/cati/internal/viewgeom	(cached)
ok  	ubunatic.com/cati/spec	(cached)
ok  	ubunatic.com/cati/v1	(cached) [no tests to run]
ok  	ubunatic.com/cati/v1/core	(cached)
ok  	ubunatic.com/cati/v1/halfblock	(cached)
ok  	ubunatic.com/cati/v1/quadblock	(cached)
ok  	ubunatic.com/cati/v1/sextant	(cached)
ok  	ubunatic.com/cati/v1/sparkline	0.309s
ok  	ubunatic.com/cati/v1/sparkline/testhelper	(cached)
?   	ubunatic.com/cati/v1/term	[no test files]
make[1]: Entering directory '/home/uwe/projects/cati'
Synced Go library example to docs/GoLibrary.md and website/index.html.
Scanning directory: docs for markdown files...
Updating SUMMARY.md: docs/SUMMARY.md...
Success! SUMMARY.md updated successfully.
make[1]: Leaving directory '/home/uwe/projects/cati' verifies install-web preserves Git status and highlighting; make check and make install passed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
The web build ran `scripts/generate_summary.go` against the checkout. That generator rewrites
`docs/SUMMARY.md` and `docs/GoLibrary.md`, creates fallback pages in `docs/`, and replaces the
highlighted `GO_EXAMPLE` region in `website/index.html` with unhighlighted source. These generated
changes make a normal website build dirty the checkout and can discard hand-maintained syntax spans.

## 2. Technical Specification / Findings
The initial reproduction attempt found no `install-web` Make target in this checkout. The target
has now been added as a scratch build: it copies the docs, generator inputs, and mdBook assets into
a temporary directory, runs the generator there, and writes the book to `website/book`. The source
HTML and docs therefore remain untouched. `make check` compares Git status before and after this
target so pre-existing edits do not mask new build side effects.

## 3. Implementation & Verification Plan
Verify with `make check` and `make install`; ensure `make install-web` succeeds and the tracked
website example retains its syntax spans.
