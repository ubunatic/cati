Adhere to the following conventions.

<!-- claudeconfig:begin Project Summary -->
<!-- claudeconfig:end Project Summary -->

## Development Scripts

Run from project root.

<!-- claudeconfig:begin Language Conventions -->
Adhere to the following conventions.

- Go/Golang @docs/Go.md,
  Modern Go, avoid deps but use Cobra, add tests
- **Prefer enums over bools** — every boolean parameter that selects between
  two modes is a future third mode waiting to happen. Define a named type with
  iota constants instead. Exceptions: flags that are truly on/off (e.g.
  `fullComp`, `useQuad` may be acceptable early but reconsider at the third
  occurrence).
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
- **Golden Image Verification**: Any new visual rendering mode or algorithm must have a corresponding golden image test under `testhelper` or `testdata` with descriptive metadata (like algorithm name and parameters) embedded as custom PNG `tEXt` chunks.
- **Rendering bugs & golden changes**: follow @docs/RenderingBugPlaybook.md — prove the root cause with numbers before touching code, predict which goldens change, and regenerate a golden only when you can name the wrong pixel and why. Never `-update` to make the suite green.

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
- Run `go vet ./... && make install` when a feature is ready — this is the authority on build correctness.
- Run `make preflight` before committing — it runs `go vet` and verifies demo scripts render without errors (`make test` is the full test suite, run separately).
- LSP diagnostics are hints only; `go build` / `make` output is authoritative. Do not fix or report errors that `go build` does not reproduce.
- Issues found during work go in `issues/` immediately — don't save them for the end.
- After implementing a plan, run `/evergreen` to update docs and close issues in the same commit.
- **After any rendering algorithm change**, update the relevant section of `docs/SparklinePixelArt.md` in the same commit — do not defer to the end of the session.
- **After context compaction / session resume**: run `git status` and `git log -5` before touching any file — cheapest way to recover ground truth without re-reading files you already read.
