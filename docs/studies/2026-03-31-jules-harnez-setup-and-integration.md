---
kind: study
title: Jules Harnez Setup and Integration Study
weight: 10
---

# Jules Harnez Setup and Integration Study

## 1. Context

This study documents the process of bootstrapping `ubunatic.com/harnez` within the Jules execution agent environment, locating agent skills, applying multi-harness configurations, and integrating Harnez into Jules' operating environment.

Jules is an autonomous software engineering agent operating within a Linux sandbox environment. The target repository is `cati` (`ubunatic.com/cati`).

## 2. Outcome & Installation Procedure

### Installation Steps & Repeatable Setup Recipe

To install Harnez and configure all agent skills and hooks in a Jules session, execute the following commands:

```bash
# 1. Install Harnez CLI binary
go install ubunatic.com/harnez/cmd/harnez@latest

# 2. Ensure GOPATH bin directory is in PATH
export PATH=$PATH:$(go env GOPATH)/bin

# 3. Apply Harnez global configurations and skill sets
harnez apply

# 4. Initialize repository specific Harnez configuration
harnez init
```

### Agent Workspace & Environment Inspection

- **Home Directory (`HOME`):** `/home/jules`
- **Jules / Claude Agent Directory:** `/home/jules/.claude/`
- **Installed Skill Locations:**
  - `/home/jules/.claude/skills/`
  - `/home/jules/.codex/skills/`
  - `/home/jules/.gemini/skills/`
  - `/home/jules/.prime/agent/skills/`
  - `/home/jules/.pi/agent/skills/`
- **CLI Binary Symlinks:** `/home/jules/go/bin/harnez`, `/home/jules/.claude/bin/⚙`, `/home/jules/.local/bin/harnez-agy`
- **Configuration & Rules:** `/home/jules/.harnez/`

When `harnez apply` is run, Harnez automatically detects active agent environments in `HOME` and syncs skill files (such as `epic`, `issue`, `study`, `sprint`, `review`, `retro`, `wrap`) across `.claude`, `.codex`, `.gemini`, `.prime`, and `.pi`.

## 3. What Went Wrong

During initial installation, running `go install ubunatic.com/harnez@latest` failed with `package ubunatic.com/harnez is not a main package`.
- **Fix:** Corrected package target to `go install ubunatic.com/harnez/cmd/harnez@latest`.
- **PATH Resolution:** GOPATH binary location (`$(go env GOPATH)/bin`) was not initially in `$PATH`, requiring explicit `$PATH` exports in bash invocations.

## 4. Learnings

- `harnez apply` is idempotent and safely provisions multi-agent skill sets and configuration files across Claude, Codex, Gemini, Prime, and Pi agent home directories.
- `harnez init` checks repository structure, configures `.harnez/rules/`, generates issue templates/indexes, and verifies Makefile target integration.
- `loom frame` requires file input rather than direct string flag arguments; using standard input pipes or temporary files ensures clean ASCII/Unicode box frame generation.

## 5. Follow-Up Tickets

1. **Issue 115:** Epic 1: `cati cam` - Camera & Video Capture Support.
2. **Issue 116:** Epic 2: `cati cam --tui` - Loom-based Video TUI.
3. **Issue 117:** Epic 3: `cati live stream & multi-device dashboard`.
