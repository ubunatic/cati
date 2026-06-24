Adhere to the following conventions.

<!-- claudeconfig:begin Project Summary -->
<!-- claudeconfig:end Project Summary -->

## Development Scripts

Run from project root.

<!-- claudeconfig:begin Language Conventions -->
Adhere to the following conventions.

- Go/Golang @docs/Go.md,
  Modern Go, avoid deps but use Cobra, add tests
- Make/Makefile @docs/Make.md,
  ⚙️ phony sentinel, self-doc help, build dependency pattern
<!-- claudeconfig:end Language Conventions -->

## Asset Generation & Licensing

- Web Assets: If you modify the logo `docs/cati_0001.png`, you must run `make generate` to sync the static inline color coordinates in `docs/index.html`.
- Licensing: The project is licensed under `AGPL-3.0-or-later`. Follow REUSE spec guidelines; license headers should be declared via `REUSE.toml` annotations instead of adding comment blocks to individual source files.

## Project Docs & Issues

- Architecture, design decisions, and pitfalls live in `docs/` — read `docs/README.md` first to orient yourself.
- Bugs, features, and design issues are tracked in `issues/` — read `issues/README.md` for the current status of known problems before starting work.

## General Rules
- Run `go vet ./...` before `make install` — fix all vet errors; do not suppress them.
- Always run `make install` when a feature is ready.
