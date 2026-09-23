package cmd

import (
	"fmt"
	"image"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

type benchmarkItemResult struct {
	entry      renderModeEntry
	geoSSIM    float64
	photoSSIM  float64
	totalSSIM  float64
	avgLatency time.Duration
	efficiency float64
}

func runModesBenchmarkScorecard(out io.Writer, width int, smart bool, entries []renderModeEntry, sortKey string) error {
	// 1. Prepare benchmark corpus
	corpusNames := []string{
		"circle", "checker", "cross", "diag", "horiz", "verti",
		"soldering", "summer", "darth",
		"emojig", "logo",
	}
	type corpusSample struct {
		name     string
		category string
		img      image.Image
	}
	corpus := make([]corpusSample, 0, len(corpusNames))
	for _, name := range corpusNames {
		loaded, err := loadNamedImage(name)
		if err != nil {
			return fmt.Errorf("load benchmark sample %q: %w", name, err)
		}
		cat := "geometric"
		if name == "soldering" || name == "summer" || name == "darth" {
			cat = "photo"
		} else if name == "emojig" || name == "logo" {
			cat = "icon"
		}
		corpus = append(corpus, corpusSample{
			name:     loaded.name,
			category: cat,
			img:      loaded.img,
		})
	}

	type modeFittedSample struct {
		sample corpusSample
		fitted image.Image
	}
	type modeRunData struct {
		entry      renderModeEntry
		totalDur   time.Duration
		samples    []modeFittedSample
		geoSSIMs   []float64
		photoSSIMs []float64
		allSSIMs   []float64
	}

	// Phase 1: Isolated sequential rendering and execution latency measurement
	runDataList := make([]*modeRunData, 0, len(entries))
	for _, entry := range entries {
		cfg := entry.cfg
		cfg.smart = smart
		rd := &modeRunData{
			entry:   entry,
			samples: make([]modeFittedSample, 0, len(corpus)),
		}
		for _, s := range corpus {
			start := time.Now()
			fitted, _, err := smartPrepareSelected(s.img, width, 0, cfg)
			if err != nil {
				return fmt.Errorf("prepare %s for mode %s: %w", s.name, entry.name, err)
			}
			_, err = renderDemoLines(fitted, cfg)
			dur := time.Since(start)
			if err != nil {
				return fmt.Errorf("render %s for mode %s: %w", s.name, entry.name, err)
			}
			rd.totalDur += dur
			rd.samples = append(rd.samples, modeFittedSample{
				sample: s,
				fitted: fitted,
			})
		}
		runDataList = append(runDataList, rd)
	}

	// Phase 2: Parallel SSIM computation across all cores
	var wg sync.WaitGroup
	for _, rd := range runDataList {
		wg.Add(1)
		go func(data *modeRunData) {
			defer wg.Done()
			cfg := data.entry.cfg
			cfg.smart = smart
			for _, item := range data.samples {
				score := calcSingleImageSSIM(item.sample.img, item.fitted, cfg, width)
				data.allSSIMs = append(data.allSSIMs, score)
				if item.sample.category == "geometric" {
					data.geoSSIMs = append(data.geoSSIMs, score)
				} else if item.sample.category == "photo" {
					data.photoSSIMs = append(data.photoSSIMs, score)
				}
			}
		}(rd)
	}
	wg.Wait()

	// Aggregate metrics
	results := make([]benchmarkItemResult, 0, len(runDataList))
	for _, rd := range runDataList {
		avgLat := rd.totalDur / time.Duration(len(corpus))
		geoAvg := average(rd.geoSSIMs)
		photoAvg := average(rd.photoSSIMs)
		totalAvg := average(rd.allSSIMs)
		eff := (1.0 - totalAvg) * float64(max(1, avgLat.Microseconds()))

		results = append(results, benchmarkItemResult{
			entry:      rd.entry,
			geoSSIM:    geoAvg,
			photoSSIM:  photoAvg,
			totalSSIM:  totalAvg,
			avgLatency: avgLat,
			efficiency: eff,
		})
	}

	hasUnoptimized := false
	for _, r := range results {
		if !isModeOptimized(r.entry) {
			hasUnoptimized = true
			break
		}
	}

	// Print Scorecard
	fmt.Fprintf(out, "Dataset Benchmark Scorecard (%d modes across %d test assets, width=%d):\n\n", len(results), len(corpus), width)
	fmt.Fprintf(out, "  Rank  %-18s  %8s  %10s  %10s  %11s  %10s\n", "Mode", "Geo SSIM", "Photo SSIM", "Total SSIM", "Avg Latency", "Efficiency")
	fmt.Fprintf(out, "  ----  %-18s  %8s  %10s  %10s  %11s  %10s\n", "------------------", "--------", "----------", "----------", "-----------", "----------")
	for i, r := range results {
		latStr := formatBenchmarkLatency(r.avgLatency)
		displayName := r.entry.name
		if !isModeOptimized(r.entry) {
			displayName += "*"
		}
		fmt.Fprintf(out, "  %4d  %-18s  %8.3f  %10.3f  %10.3f  %11s  %10.2f\n",
			i+1, displayName, r.geoSSIM, r.photoSSIM, r.totalSSIM, latStr, r.efficiency)
	}
	if hasUnoptimized {
		fmt.Fprintln(out, "\n  * Mode is not yet bitwise optimized (cell geometry > 128px; runs via scalar fallback)")
	}
	return nil
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func formatBenchmarkLatency(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < 10*time.Millisecond {
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000.0)
}

func sortBenchmarkResults(results []benchmarkItemResult, sortKey string) {
	key := strings.ToLower(strings.TrimSpace(sortKey))
	if key == "" {
		key = "ssim"
	}
	switch key {
	case "ssim", "psnr", "best", "quality":
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].totalSSIM != results[j].totalSSIM {
				return results[i].totalSSIM > results[j].totalSSIM
			}
			return results[i].entry.name < results[j].entry.name
		})
	case "-ssim", "-psnr", "worst":
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].totalSSIM != results[j].totalSSIM {
				return results[i].totalSSIM < results[j].totalSSIM
			}
			return results[i].entry.name < results[j].entry.name
		})
	case "time", "dur", "fastest":
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].avgLatency != results[j].avgLatency {
				return results[i].avgLatency < results[j].avgLatency
			}
			return results[i].entry.name < results[j].entry.name
		})
	case "-time", "-dur", "slowest":
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].avgLatency != results[j].avgLatency {
				return results[i].avgLatency > results[j].avgLatency
			}
			return results[i].entry.name < results[j].entry.name
		})
	case "eff", "efficiency":
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].efficiency != results[j].efficiency {
				return results[i].efficiency < results[j].efficiency
			}
			return results[i].entry.name < results[j].entry.name
		})
	case "-eff", "-efficiency":
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].efficiency != results[j].efficiency {
				return results[i].efficiency > results[j].efficiency
			}
			return results[i].entry.name < results[j].entry.name
		})
	case "name":
		sort.SliceStable(results, func(i, j int) bool {
			return results[i].entry.name < results[j].entry.name
		})
	case "-name":
		sort.SliceStable(results, func(i, j int) bool {
			return results[i].entry.name > results[j].entry.name
		})
	default:
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].totalSSIM != results[j].totalSSIM {
				return results[i].totalSSIM > results[j].totalSSIM
			}
			return results[i].entry.name < results[j].entry.name
		})
	}
}
