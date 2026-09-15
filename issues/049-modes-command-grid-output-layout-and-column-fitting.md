# 049 — Modes Command Grid Output Layout and Column Fitting

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `cmd/modes.go`, `cmd/modes_test.go`, `issues/042-add-info-metadata-to-cati-modes-demos.md`, `issues/044-modes-command-custom-image-input-support.md`

---

## 1. Problem & Motivation

`cati modes` outputs all mode demos in a single vertical list (1 demo card per row). On standard or wide terminal screens (e.g. 120–240 columns), each demo card occupies only a fraction of the available horizontal space (typically 24–40 columns), forcing extensive vertical scrolling to review or compare render modes.

Introducing a multi-column grid layout enables visual side-by-side comparison of multiple render modes on a single screen without scrolling.

---

## 2. Technical Specification

### 2.1 CLI Flags
- Add `--cols` (or `-c`) to `cati modes`:
  - Type: `string` (or int with special `"auto"` string parsing)
  - Default: `"auto"`
  - Allowed values: `"auto"` (or `"0"`), `"1"`, `"2"`, `"3"`, ..., positive integer $N$.

### 2.2 Layout & Auto-Fit Behavior
- **`--cols auto` (Default)**:
  - Query current terminal width (e.g. `term.GetSize` / terminal cols).
  - If output is piped or terminal width cannot be determined, default to 1 column.
  - If terminal width allows, choose $K \in [1, 3]$ columns such that $K \times (\text{cardWidth} + \text{gap}) \le \text{termWidth}$.
- **`--cols N`**:
  - Explicitly render $N$ demo cards per row.

### 2.3 Grid Rendering Engine
- Demo cards in each grid row must be stitched line-by-line:
  - Pad shorter cards with empty lines so row heights remain uniform.
  - Maintain horizontal gutter / separator spacing between columns.
  - Correctly align ANSI color escape codes without breaking column margins.
- Support both pair mode (cati logo | emojig logo) and custom single-image demos (`-i <image>`).

---

## 3. Implementation Plan

1. **Flag definition**:
   - Register `--cols` in `modesCommand()` in `cmd/modes.go`.
2. **Terminal width query**:
   - Determine terminal width with fallback for non-TTY / piped output.
3. **Grid formatter in `cmd/modes.go`**:
   - Implement `formatCardsGrid`, `formatGridRowLines`, and `autoFitGridCols` that aggregates items into rows and joins lines side-by-side.
4. **Unit and layout testing**:
   - Add unit tests in `cmd/modes_test.go` verifying 1-col, 2-col, 3-col stitching, ANSI preservation, and auto-column derivation.

---

## 4. Acceptance & Verification Criteria

- [x] `cati modes` defaults to `--cols auto`, showing 1 to 3 columns based on terminal width.
- [x] `cati modes --cols 1` forces traditional single-column vertical list.
- [x] `cati modes --cols 2` and `--cols 3` render 2 and 3 columns side-by-side respectively.
- [x] Grid formatting preserves all header labels, metric stats (`ssim`, `ms`), and ANSI colors without line wrap glitches.
- [x] `make test` and `make preflight` pass cleanly.
