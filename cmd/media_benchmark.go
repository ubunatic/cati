package cmd

import (
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"time"

	"ubunatic.com/cati/v1/core"
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
	// slow is true when the first render exceeded the budget.
	// The single render's timing is still reported.
	slow bool
	// timedOut is true when the first render was cancelled because it
	// exceeded the budget (render ran in a goroutine and was abandoned).
	timedOut bool
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
	if mode != "" {
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
		fmt.Fprintf(out, "Video benchmark: %s (%dx%d), fast vs simple paths\n", filepath.Base(path), width, height)
		fmt.Fprintln(out, "Mode                 Frames    Fast fps  Simple fps   Speedup")
	} else {
		budget = max(time.Millisecond, budget/time.Duration(2*len(entries)))
		fmt.Fprintf(out, "Image benchmark: %s (%dx%d), budget %s per mode and path\n", filepath.Base(path), width, height, budget.Round(time.Millisecond))
		fmt.Fprintln(out, "Mode                 Fast avg/render   Simple avg/render   Speedup")
	}
	defer func(prev bool) { core.Fastpath = prev }(core.Fastpath)
	// Each mode row is printed immediately after its benchmark completes (streaming).
	for _, entry := range entries {
		cfg := entry.cfg
		cfg.jobs = jobs
		if isVideo {
			var fps [2]float64
			var frames int
			for i, fast := range []bool{true, false} {
				core.Fastpath = fast
				res, err := benchmarkVideoMode(path, width, height, cfg, now, render)
				if err != nil {
					return fmt.Errorf("benchmark mode %s: %w", entry.name, err)
				}
				frames = res.frames
				if res.total > 0 {
					fps[i] = float64(res.frames) / res.total.Seconds()
				}
			}
			fmt.Fprintf(out, "%-20s %6d   %9.1f   %9.1f   %s\n", entry.name, frames, fps[0], fps[1], speedup(fps[0], fps[1]))
			continue
		}
		fitted, err := fitBenchmarkImage(img, width, height, cfg)
		if err != nil {
			return fmt.Errorf("fit image for mode %s: %w", entry.name, err)
		}
		var cells [2]string
		var avgs [2]time.Duration
		for i, fast := range []bool{true, false} {
			core.Fastpath = fast
			cells[i], avgs[i] = benchmarkCell(benchmarkImageMode(fitted, cfg, budget, now, render))
		}
		ratio := "—"
		if avgs[0] > 0 && avgs[1] > 0 {
			ratio = speedup(float64(avgs[1]), float64(avgs[0]))
		}
		fmt.Fprintf(out, "%-20s %15s   %17s   %s\n", entry.name, cells[0], cells[1], ratio)
	}
	return nil
}

// benchmarkCell formats one image benchmark result and returns its average
// render time (0 when no usable measurement exists).
func benchmarkCell(res mediaBenchmarkModeResult) (string, time.Duration) {
	switch {
	case res.skipped:
		return "error", 0
	case res.timedOut:
		return ">budget", 0
	}
	avg := res.render / time.Duration(res.frames)
	if res.slow {
		return avg.String() + " (slow)", avg
	}
	return avg.String(), avg
}

// speedup formats how many times faster a is than b.
func speedup(a, b float64) string {
	if a <= 0 || b <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.2fx", a/b)
}

// benchmarkImageMode runs as many renders as possible within the budget.
// The first render is run in a goroutine so it can be abandoned if it
// exceeds the budget, rather than blocking the whole benchmark.
// At least one render is always attempted; if the first render finishes
// within the budget the loop continues until the budget is exhausted.
func benchmarkImageMode(img image.Image, cfg renderCfg, budget time.Duration, now func() time.Time, render func(image.Image, renderCfg) ([]string, error)) mediaBenchmarkModeResult {
	type renderResult struct {
		dur time.Duration
		err error
	}
	runRender := func() (time.Duration, error) {
		ch := make(chan renderResult, 1)
		before := now()
		go func() {
			_, err := render(img, cfg)
			ch <- renderResult{dur: now().Sub(before), err: err}
		}()
		select {
		case r := <-ch:
			return r.dur, r.err
		case <-time.After(budget):
			// Goroutine leaks but render has no cancellation; acceptable for
			// a benchmark helper that runs once per mode.
			return budget, context.DeadlineExceeded
		}
	}

	var result mediaBenchmarkModeResult
	start := now()

	// First render — always run at least one.
	dur, err := runRender()
	if err == context.DeadlineExceeded {
		result.timedOut = true
		return result
	}
	if err != nil {
		result.skipped = true
		return result
	}
	result.render += dur
	result.frames++
	if dur > budget {
		// Render completed but took longer than the budget; mark slow.
		result.slow = true
		return result
	}

	// Continue rendering until the budget is exhausted.
	for now().Sub(start) < budget {
		dur, err = runRender()
		if err != nil {
			break
		}
		result.render += dur
		result.frames++
	}
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
		if _, err := render(fitted, cfg); err != nil {
			return result, err
		}
		result.render += now().Sub(renderStart)
		result.frames++
	}
	result.total = now().Sub(start)
	return result, nil
}
