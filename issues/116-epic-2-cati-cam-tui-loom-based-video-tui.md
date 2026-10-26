# Epic 2: cati cam --tui - Loom-based Video TUI

**Status:** Open
**Category:** Epic
**Priority:** High

## /goal

Build an interactive `ubunatic.com/loom` based TUI for live camera viewing (`cati cam --tui` / `catibrowse cam`), supporting real-time stream controls, mode/resolution toggles, and screenshot exports as both JPEG files and `.ansi` formatted terminal art files.

Exit clause / circuit breaker: Deliver full Loom TUI interface with keybindings (`s` for JPEG screenshot, `a` for .ANSI export, `q` for quit) and verified Loom layout, or report if terminal capabilities/event loop prevents frame rendering at >=15 FPS.

## Out of Scope

- Audio recording or sound pipeline sync.
- Multi-camera simultaneous matrix grid (covered in Epic 3).

## Open Decisions

- Frame buffer refresh rate in Loom vs raw stdout ANSI diffing.

## Task Table

| Ticket | What | Sprint Group | Lead | Reviewer | Status |
|--------|------|--------------|------|----------|--------|
| TBD-1  | Loom TUI widget layout & header status bar | SG1 | Developer | Architect | Open |
| TBD-2  | Snapshot exporter (.JPEG & .ANSI) | SG1 | Developer | Reviewer | Open |
| TBD-3  | Loom keyboard shortcut handler (`s`, `a`, `q`, mode switch) | SG2 | Developer | Reviewer | Open |
| TBD-4  | Validation against `loom check-box` & `loom fmt` assets | SG2 | Developer | Owner | Open |
