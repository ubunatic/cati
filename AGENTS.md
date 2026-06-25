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
- Before starting any feature or fix, skim any Open/In-Progress issues that touch the area you're working in — the index is short and pays for itself immediately.
- When you change an interface, function signature, or data flow, update the relevant section of the Evergreen doc in the same logical step — not at the end of the session.

## Spec System (`spec/`)

> **`spec/` is application code. Treat it with the same rigour as Go source.**

Full reference: @docs/Spec.md — read it before touching any `spec/` file or its Go loaders.

Rules:
- **Spec is always readable** — Go loaders must not crash or silently degrade when the spec file exists; if the file is missing the app degrades gracefully (raw key names shown, no panic)
- **No Go fallbacks** — do not maintain hardcoded copies of spec content in Go (button labels, action names, key sequences); the spec file is the only source
- **All keys must be specced** — every key that triggers an action must have a `keys:` entry in the button that owns it; undocumented hardcoded keys are a bug (exception: structural keys marked with a comment, e.g. Enter to open, Tab to cycle, `\x03` Ctrl-C safeguard)
- **All objects must be used** — every button in `buttons.yaml` must appear in a view row or `hidden_keys:`; every action in the schema enum must have a Go handler
- **No stale properties** — removing a button means removing it from `views.yaml`, its action from the schema enum (if unused), and its Go handler
- **Write spec integrity tests** — see `docs/Spec.md §6` for the required test matrix; add tests whenever a new action or button is introduced
- **Update spec and Go together** — new action = schema enum + buttons.yaml + views.yaml + Go handler + test, all in the same commit

## General Rules
- Run `go vet ./...` before `make install` — fix all vet errors; do not suppress them.
- Always run `make install` when a feature is ready.
