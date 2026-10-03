# 074 — Add Cobra CLI completion for files and flags

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**:

---

## 1. Problem & Motivation
Shell auto-completion for `cati` is currently limited or blocked in common workflows:
1. Positional file completion (`cati <file>`) does not seamlessly suggest image files or directories because Cobra's root command has subcommands (`play`, `browse`, `modes`), causing default shell completion to prioritize or stall on subcommand matching rather than falling back to file/directory path completion.
2. Key CLI flags lack value completions and contextual descriptions, requiring users to look up help text manually.

Adding rich Cobra completion hooks will significantly improve CLI ergonomics for shell users (Bash, Zsh, Fish, PowerShell).

## 2. Technical Specification / Findings
The primary focus is the root `cati` command (`cati <file>` and `cati --<flags>`).

### Positional File Completion
- Provide a `ValidArgsFunction` (or appropriate `cobra.ShellCompDirective`) on `root` that completes:
  - Subcommands when matching subcommand prefixes (`play`, `browse`, `modes`)
  - Files matching supported image extensions (`.png`, `.jpg`, `.jpeg`, `.svg`, `.webp`, `.gif`, `.bmp`, `.tiff`, `.tif`) and directories (e.g. `cobra.ShellCompDirectiveFilterFileExt` / `cobra.ShellCompDirectiveDefault`).

### Flag Value Completions
Register flag completion functions (`RegisterFlagCompletionFunc`) on the root command for:
- `--play`, `-p`: `once` (play once and exit), `repeat` (loop continuously), `preview` (render frame previews).
- `--aspect`: `default` (aspect-preserving fit), `aligned` (preserve source pixels and pad incomplete cells).
- `--mode`, `-m`: available render mode aliases and names (`half`, `quad`, `spark`, `six`, `hs`, `sq`, `xh`, `sx`, `b24`, `b24bg`, etc.), generated from spec / render mode registry.
- `--prescaler`, `-S`: `nearest-neighbor` / `nn`, `pyramid`.
- `--crop`, `-c`: common crop specs (`auto`, `center`, `l,t`, `c,m`, `W:H`, `W:H:X:Y`).
- `--zoom`, `-z`: zoom presets (`0` (fit), `1` / `1:1` (original scale), `w` (term width), `h` (term height)).

Other verbs and subcommands (`cati play`, `cati browse`) can follow in subsequent passes.

## 3. Implementation & Verification Plan
**Goal**: `/goal Add Cobra shell completion functions for positional file arguments and flags (--play, --aspect, --mode, --prescaler, --crop, --zoom) on the root cati command, verify with automated completion tests, or stop and report when blocked on a user decision or denied permission.`

### Plan
1. Register `ValidArgsFunction` on `root` in `cmd/root.go` to handle positional file and directory completion without breaking subcommand dispatch.
2. Register custom completion functions for candidate flags (`--play`, `--aspect`, `--mode`, `--prescaler`, `--crop`, `--zoom`).
3. Add automated tests in `cmd/` using Cobra's `__complete` hidden command or `ValidArgsFunction` directly to verify suggestions and directives.
4. Verify shell completion behavior across bash/zsh test harnesses.
