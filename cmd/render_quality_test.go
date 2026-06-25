package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/quadblock"
)

// TestRenderQuality loads the sample images, computes SSIM for every render
// variant, prints the top-3 per image and a cross-image summary.
// Run with: go test ./cmd/ -run TestRenderQuality -v
func TestRenderQuality(t *testing.T) {
	const (
		cols = 80 // terminal columns for the test render
		rows = 40 // terminal rows
	)

	sampleDir := "assets/samples"
	entries, err := os.ReadDir(sampleDir)
	if err != nil {
		t.Skipf("sample dir not found (%v) — skipping quality benchmark", err)
	}
	var samplePaths []string
	for _, e := range entries {
		if !e.IsDir() {
			samplePaths = append(samplePaths, filepath.Join(sampleDir, e.Name()))
		}
	}
	if len(samplePaths) == 0 {
		t.Skip("no sample images found")
	}

	type variant struct {
		name    string
		useQuad bool
		opts    quadblock.Options
	}
	variants := []variant{
		// ── Halfblock baseline ────────────────────────────────────────────────
		{"halfblock", false, quadblock.Options{}},

		// ── Quad: default (coverage+continuity colour pair) ───────────────────
		{"quad/default", true, quadblock.Options{}},
		{"quad/hb2", true, quadblock.Options{HalfblockThreshold: 2}},
		{"quad/hb3", true, quadblock.Options{HalfblockThreshold: 3}},

		// ── Quad: halfblock row-average colour pair ───────────────────────────
		{"quad/splithalf", true, quadblock.Options{SplitHalf: true}},
		{"quad/splithalf-nb", true, quadblock.Options{SplitHalf: true, SplitHalfNeighbors: true}},

		// ── Quad: luminance-split ─────────────────────────────────────────────
		{"quad/lum-split", true, quadblock.Options{LumSplit: true}},

		// ── Quad: PCA (power-iteration on 3×3 RGB covariance matrix) ─────────
		{"quad/pca2", true, quadblock.Options{PCA2: true}},

		// ── Quad: diameter split (max-distance pair → group averages) ─────────
		{"quad/diameter", true, quadblock.Options{Diameter: true}},
		{"quad/diameter+hb2", true, quadblock.Options{Diameter: true, HalfblockThreshold: 2}},

		// ── Quad: k-means (initialised from diameter) ─────────────────────────
		{"quad/kmeans2", true, quadblock.Options{KMeans: 2}},
		{"quad/kmeans3", true, quadblock.Options{KMeans: 3}},
		{"quad/kmeans5", true, quadblock.Options{KMeans: 5}},

		// ── Quad: neighbourhood blending ─────────────────────────────────────
		{"quad/blend-ambig", true, quadblock.Options{Blend: quadblock.BlendAmbiguous}},
		{"quad/blend-wide", true, quadblock.Options{Blend: quadblock.BlendAmbiguousWide}},

		// ── Combinations ─────────────────────────────────────────────────────
		{"quad/pca2+blend", true, quadblock.Options{PCA2: true, Blend: quadblock.BlendAmbiguous}},
		{"quad/kmeans3+blend", true, quadblock.Options{KMeans: 3, Blend: quadblock.BlendAmbiguous}},
		{"quad/kmeans3+hb2", true, quadblock.Options{KMeans: 3, HalfblockThreshold: 2}},
	}

	type result struct {
		variant string
		ssim    float64
	}

	// imageName extracts just the filename without directory.
	imageName := func(path string) string { return filepath.Base(path) }

	// rankLabel annotates position 0,1,2 with medal emoji.
	rankLabel := func(i int) string {
		switch i {
		case 0:
			return "1st"
		case 1:
			return "2nd"
		case 2:
			return "3rd"
		default:
			return fmt.Sprintf("%dth", i+1)
		}
	}

	// Per-variant cumulative SSIM for the summary.
	cumSSIM := make(map[string]float64, len(variants))

	fmt.Printf("\n╔═══════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║              Render Quality Benchmark (SSIM)                  ║\n")
	fmt.Printf("╚═══════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("  Terminal: %d×%d  |  %d variants  |  %d images\n\n",
		cols, rows, len(variants), len(samplePaths))

	for _, imgPath := range samplePaths {
		orig, err := halfblock.LoadImage(imgPath)
		if err != nil {
			t.Errorf("load %s: %v", imgPath, err)
			continue
		}

		var results []result
		for _, v := range variants {
			var ssim float64
			if v.useQuad {
				scaled := quadblock.ScaleToFit(orig, cols, rows)
				b := scaled.Bounds()
				// Pyramid reference from orig (not the NN-scaled viewport): sharper
				// than a single-pass box so blurry renders can't cheat, and different
				// from NN so halfblock != 1.0.
				ref := pyramidDownscale(orig, b.Dx(), b.Dy())
				rendered := quadblock.RenderToImage(scaled, v.opts)
				ssim = ssimLuminance(ref, rendered)
			} else {
				scaled := halfblock.ScaleToFit(orig, cols, rows)
				b := scaled.Bounds()
				ref := pyramidDownscale(orig, b.Dx(), b.Dy())
				ssim = ssimLuminance(ref, scaled)
			}
			results = append(results, result{v.name, ssim})
			cumSSIM[v.name] += ssim
		}

		sort.Slice(results, func(i, j int) bool {
			return results[i].ssim > results[j].ssim
		})

		fmt.Printf("  %-45s\n", imageName(imgPath))
		fmt.Printf("  %-30s  %s\n", "variant", "SSIM")
		fmt.Printf("  %-30s  %s\n", "───────────────────────────────", "──────")
		for i, r := range results {
			medal := ""
			if i < 3 {
				medal = " ← " + rankLabel(i)
			}
			fmt.Printf("  %-30s  %.4f%s\n", r.variant, r.ssim, medal)
		}
		fmt.Println()
	}

	// Summary: rank variants by average SSIM across all images.
	type avgResult struct {
		name string
		avg  float64
	}
	var summary []avgResult
	n := float64(len(samplePaths))
	for _, v := range variants {
		summary = append(summary, avgResult{v.name, cumSSIM[v.name] / n})
	}
	sort.Slice(summary, func(i, j int) bool {
		return summary[i].avg > summary[j].avg
	})

	fmt.Printf("  ╔══════════════════════════════════════════╗\n")
	fmt.Printf("  ║   Summary: average SSIM across images    ║\n")
	fmt.Printf("  ╚══════════════════════════════════════════╝\n")
	fmt.Printf("  %-30s  %s\n", "variant", "avg SSIM")
	fmt.Printf("  %-30s  %s\n", "───────────────────────────────", "────────")
	for i, r := range summary {
		medal := ""
		if i < 3 {
			medal = " ← " + rankLabel(i)
		}
		fmt.Printf("  %-30s  %.4f%s\n", r.name, r.avg, medal)
	}
	fmt.Println()
}
