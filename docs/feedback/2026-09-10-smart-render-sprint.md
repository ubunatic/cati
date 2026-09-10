## Smart render sprint

- The shared fit/reconstruction path made the width search small and kept
  existing algorithms unchanged.
- A spec-backed policy is important: runtime behavior must consume the loaded
  bound, not merely validate it in tests.
- Smart mode is currently static-only because candidate scoring can multiply
  interactive/video frame work; those paths reject the flag explicitly.
- The worktree already contained a large JPEG golden/toolchain migration, so
  it should be reviewed and committed separately from the smart-render change.
- Native stepping is best represented in render-pixel space: the spec declares
  the step, while the mode cell width explains its fractional terminal-column
  meaning. The final canvas remains integer-column output.
- Reusing the normal fit pipeline for native candidate heights avoids silently
  changing aspect and terminal-row behavior while allowing partial cell widths.
- The tracker index generator currently rejects this repository's customized
  three-column table; the new ticket row was synchronized manually without
  rewriting historical rows.
