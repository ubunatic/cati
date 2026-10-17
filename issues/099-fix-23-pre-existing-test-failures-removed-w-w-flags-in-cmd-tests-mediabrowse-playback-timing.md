# Fix pre-existing test failures on main

**Status**: Open
**Priority**: P1
**Category**: Bug

## Problem
`go test ./...` on main (64d516e, also before the GitHub merge) has 23 failing (sub)tests:

- `cmd`: tests still pass `-w`/`-W` (e.g. `modes --sort ssim -w ...`, `TestCLIDoom1S2Render`), which the CLI no longer accepts: `unknown shorthand flag: 'w' in -w`.
- `examples/mediabrowse`: `TestRealVideoPlaybackIntegration` sees `[halfblock, paused]`, wants playing (timing/state).

Failing tests:

```
--- FAIL: TestCLIDoom1S2Render (0.24s)
--- FAIL: TestCLIDoom1S2Render/aspect_pixel_preview_width_54_3x3 (0.02s)
--- FAIL: TestCLIDoom1S2Render/play_preview_-H_67_s2 (0.02s)
--- FAIL: TestCLIDoom1S2Render/play_preview_width_only_aspect_aligned_halfblock (0.02s)
--- FAIL: TestModesBenchmarkScorecard (0.00s)
--- FAIL: TestModesCommandColsFlag (0.00s)
--- FAIL: TestModesCommandColsFlag/cols_0_flag_accepted_as_auto (0.00s)
--- FAIL: TestModesCommandColsFlag/cols_1_single_column_output (0.00s)
--- FAIL: TestModesCommandColsFlag/cols_2_side-by-side_grid_output (0.00s)
--- FAIL: TestModesCommandColsFlag/cols_3_side-by-side_grid_output (0.00s)
--- FAIL: TestModesCommandColsFlag/cols_auto_flag_accepted (0.00s)
--- FAIL: TestModesCommandSmartZeroWidthListsWithoutRendering (0.00s)
--- FAIL: TestModesCustomImageInput (0.00s)
--- FAIL: TestModesCustomImageInput/image_pair_mode (0.00s)
--- FAIL: TestModesCustomImageInput/single_image_standard_vs_smart (0.00s)
--- FAIL: TestModesInfoListsRegistryCompositionsAndPreservesCase (0.00s)
--- FAIL: TestModesSamplePresets (0.00s)
--- FAIL: TestModesSamplePresets/render_preset_shortcut (0.00s)
--- FAIL: TestModesSorting (0.05s)
--- FAIL: TestModesSorting/demo_sort_by_efficiency (0.00s)
--- FAIL: TestModesSorting/demo_sort_by_ssim_and_by-ssim_flag (0.00s)
--- FAIL: TestModesStatsInTitle (0.00s)
--- FAIL: TestRealVideoPlaybackIntegration (0.08s)
```

## Acceptance
- Tests use the current width flags (or the CLI restores the documented shorthand, per spec).
- mediabrowse playback test is deterministic.
- `go test ./...` green; CI ticket 097 can then gate on it.
