package cmd

import (
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
	catiterm "ubunatic.com/cati/v1/term"
)

type mediaBenchmarkModeResult struct {
	mode   string
	frames int
	decode time.Duration
	render time.Duration
	total  time.Duration
	// skipped is true when the first render returned an error.
	skipped bool
	// slow is true when the single, mandatory render exceeded the budget.
	slow bool
}

func runMediaBenchmark(out io.Writer, path string, width, height, jobs int, mode string, budget time.Duration) error {
	return runMediaBenchmarkWithRunner(out, path, width, height, jobs, mode, budget, time.Now, renderDemoLines)
}

func runMediaBenchmarkWithRunner(out io.Writer, path string, width, height, jobs int, mode string, budget time.Duration, now func() time.Time, render func(image.Image, renderCfg) ([]string, error)) error {
	if budget <= 0 {
		return fmt.Errorf("--bench-budget must be greater than 0")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat media file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("media file %q is not a regular file", path)
	}
	// Use a sensible fixed default when stdout is not a terminal.
	// TermWidth/TermHeight return 0 when stdout is not a real TTY.
	if width <= 0 {
		width = 80
		if tw := catiterm.TermWidth(); tw > 0 {
			width = tw
		}
	}
	if height <= 0 {
		height = 24
		if th := catiterm.TermHeight(); th > 0 {
			height = th
		}
	}
	entries, err := listableRenderModes()
	if err != nil {
		return fmt.Errorf("load render modes: %w", err)
	}
	if mode == "" {
		entries, err = withoutExperimentalModes(entries)
		if err != nil {
			return err
		}
	} else {
		entries, err = selectedRenderModes([]string{mode})
		if err != nil {
			return fmt.Errorf("select render mode: %w", err)
		}
	}
	if len(entries) == 0 {
		return fmt.Errorf("no render modes selected")
	}
	isVideo := halfblock.IsVideo(path)
	var img image.Image
	if !isVideo {
		img, err = halfblock.LoadImage(path)
		if err != nil {
			return fmt.Errorf("load image %q: %w", path, err)
		}
	}
	if isVideo {
		fmt.Fprintf(out, "Video benchmark: %s (%dx%d)\n", filepath.Base(path), width, height)
		fmt.Fprintln(out, "Mode                 Frames       FPS   Render time")
	} else {
		budget = max(time.Millisecond, budget/time.Duration(len(entries)))
		fmt.Fprintf(out, "Image benchmark: %s (%dx%d), budget %s per mode\n", filepath.Base(path), width, height, budget.Round(time.Millisecond))
		fmt.Fprintln(out, "Mode                 Avg/render   Renders")
	}
	// Each mode row is printed immediately after its benchmark completes (streaming).
	for _, entry := range entries {
		cfg := entry.cfg
		cfg.jobs = jobs
		if isVideo {
			res, err := benchmarkVideoMode(path, width, height, cfg, now, render)
			if err != nil {
				return fmt.Errorf("benchmark mode %s: %w", entry.name, err)
			}
			fps := 0.0
			if res.total > 0 {
				fps = float64(res.frames) / res.total.Seconds()
			}
			fmt.Fprintf(out, "%-20s %6d   %9.1f   %s\n", entry.name, res.frames, fps, res.render.Round(time.Microsecond))
			continue
		}
		fitted, err := fitBenchmarkImage(img, width, height, cfg)
		if err != nil {
			return fmt.Errorf("fit image for mode %s: %w", entry.name, err)
		}
		res := benchmarkImageMode(fitted, cfg, budget, now, render)
		cell := benchmarkCell(res)
		fmt.Fprintf(out, "%-20s %10s   %7d\n", entry.name, cell, res.frames)
	}
	return nil
}

// benchmarkCell formats one image benchmark result.
func benchmarkCell(res mediaBenchmarkModeResult) string {
	switch {
	case res.skipped:
		return "error"
	}
	avg := (res.render / time.Duration(res.frames)).Round(time.Microsecond)
	if res.slow {
		return avg.String() + " (slow)"
	}
	return avg.String()
}

// benchmarkImageMode renders once unconditionally, so every mode gets a
// measurement, then keeps rendering until the budget is used up.
func benchmarkImageMode(img image.Image, cfg renderCfg, budget time.Duration, now func() time.Time, render func(image.Image, renderCfg) ([]string, error)) mediaBenchmarkModeResult {
	var result mediaBenchmarkModeResult
	start := now()
	for result.frames == 0 || now().Sub(start) < budget {
		before := now()
		_, err := render(img, cfg)
		if err != nil {
			result.skipped = result.frames == 0
			break
		}
		result.render += now().Sub(before)
		result.frames++
	}
	result.slow = result.frames == 1 && result.render > budget
	return result
}

func fitBenchmarkImage(img image.Image, width, height int, cfg renderCfg) (image.Image, error) {
	fitted, _, err := smartPrepareSelected(img, width, height, cfg)
	return fitted, err
}

func benchmarkVideoMode(path string, width, height int, cfg renderCfg, now func() time.Time, render func(image.Image, renderCfg) ([]string, error)) (mediaBenchmarkModeResult, error) {
	start := now()
	frames, cleanup, err := halfblock.OpenVideoStream(context.Background(), path, 0, 0, 0)
	if err != nil {
		return mediaBenchmarkModeResult{}, err
	}
	defer cleanup()
	var result mediaBenchmarkModeResult
	for frame := range frames {
		fitted, err := fitBenchmarkImage(frame, width, height, cfg)
		if err != nil {
			return result, err
		}
		renderStart := now()
		_, err = render(fitted, cfg)
		if err != nil {
			return result, err
		}
		result.render += now().Sub(renderStart)
		result.frames++
	}
	result.total = now().Sub(start)
	return result, nil
}

// withoutExperimentalModes drops the spec's experimental compositions.
func withoutExperimentalModes(entries []renderModeEntry) ([]renderModeEntry, error) {
	rm, err := spec.LoadRenderModes()
	if err != nil {
		return nil, fmt.Errorf("load render modes: %w", err)
	}
	kept := entries[:0:0]
	for _, e := range entries {
		if !slices.Contains(rm.Experimental, e.name) {
			kept = append(kept, e)
		}
	}
	return kept, nil
}
