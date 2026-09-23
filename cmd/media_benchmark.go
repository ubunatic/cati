package cmd

import (
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"time"

	"ubunatic.com/cati/v1/halfblock"
	catiterm "ubunatic.com/cati/v1/term"
)

type mediaBenchmarkModeResult struct {
	mode   string
	frames int
	decode time.Duration
	render time.Duration
	total  time.Duration
}

func runMediaBenchmark(out io.Writer, path string, width, height, jobs, iterations int) error {
	if iterations < 1 {
		return fmt.Errorf("--bench-iterations must be at least 1")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat media file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("media file %q is not a regular file", path)
	}
	if width <= 0 {
		width = max(1, catiterm.TermWidth())
	}
	if height <= 0 {
		height = max(1, catiterm.TermHeight())
	}
	entries, err := listableRenderModes()
	if err != nil {
		return fmt.Errorf("load render modes: %w", err)
	}
	results := make([]mediaBenchmarkModeResult, 0, len(entries))
	if halfblock.IsVideo(path) {
		for _, entry := range entries {
			cfg := entry.cfg
			cfg.jobs = jobs
			res, err := benchmarkVideoMode(path, width, height, cfg)
			if err != nil {
				return fmt.Errorf("benchmark mode %s: %w", entry.name, err)
			}
			res.mode = entry.name
			res.decode = res.total - res.render
			results = append(results, res)
		}
		fmt.Fprintf(out, "Video benchmark: %s (%dx%d)\n", filepath.Base(path), width, height)
		fmt.Fprintln(out, "Mode                 Frames   Stream overhead   Render time   Total fps")
		for _, r := range results {
			total := r.total
			fps := 0.0
			if total > 0 {
				fps = float64(r.frames) / total.Seconds()
			}
			fmt.Fprintf(out, "%-20s %6d   %11s   %11s   %9.1f\n", r.mode, r.frames, r.decode.Round(time.Millisecond), r.render.Round(time.Millisecond), fps)
		}
		return nil
	}

	img, err := halfblock.LoadImage(path)
	if err != nil {
		return fmt.Errorf("load image %q: %w", path, err)
	}
	for _, entry := range entries {
		cfg := entry.cfg
		cfg.jobs = jobs
		fitted, err := fitBenchmarkImage(img, width, height, cfg)
		if err != nil {
			return fmt.Errorf("fit image for mode %s: %w", entry.name, err)
		}
		start := time.Now()
		for i := 0; i < iterations; i++ {
			if _, err := renderDemoLines(fitted, cfg); err != nil {
				return fmt.Errorf("render image for mode %s: %w", entry.name, err)
			}
		}
		d := time.Since(start)
		results = append(results, mediaBenchmarkModeResult{mode: entry.name, frames: iterations, render: d})
	}
	fmt.Fprintf(out, "Image benchmark: %s (%dx%d), %d renders per mode\n", filepath.Base(path), img.Bounds().Dx(), img.Bounds().Dy(), iterations)
	fmt.Fprintln(out, "Mode                 Avg/render    Renders/sec")
	for _, r := range results {
		avg := r.render / time.Duration(r.frames)
		fps := 0.0
		if avg > 0 {
			fps = float64(time.Second) / float64(avg)
		}
		fmt.Fprintf(out, "%-20s %11s   %11.1f\n", r.mode, avg, fps)
	}
	return nil
}

func fitBenchmarkImage(img image.Image, width, height int, cfg renderCfg) (image.Image, error) {
	fitted, _, err := smartPrepareSelected(img, width, height, cfg)
	return fitted, err
}

func benchmarkVideoMode(path string, width, height int, cfg renderCfg) (mediaBenchmarkModeResult, error) {
	start := time.Now()
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
		renderStart := time.Now()
		if _, err := renderDemoLines(fitted, cfg); err != nil {
			return result, err
		}
		result.render += time.Since(renderStart)
		result.frames++
	}
	result.total = time.Since(start)
	return result, nil
}
