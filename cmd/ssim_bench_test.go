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

// findModRoot walks up from the current working directory until it finds go.mod.
func findModRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// allVariants lists every render configuration to benchmark.
// Add new entries here when new algorithms are implemented.
var allVariants = []struct {
	name string
	rc   renderCfg
}{
	// ── halfblock (baseline) ────────────────────────────────────────────────────
	{"halfblock", renderCfg{}},

	// ── quad: colour-pair selection algorithms ──────────────────────────────────
	{"quad/default", renderCfg{useQuad: true}},
	{"quad/lum-split", renderCfg{useQuad: true, quadOpts: quadblock.Options{LumSplit: true}}},
	{"quad/splithalf", renderCfg{useQuad: true, quadOpts: quadblock.Options{SplitHalf: true}}},
	{"quad/splithalf-nb", renderCfg{useQuad: true, quadOpts: quadblock.Options{SplitHalf: true, SplitHalfNeighbors: true}}},
	{"quad/pca2", renderCfg{useQuad: true, quadOpts: quadblock.Options{PCA2: true}}},

	// ── quad: halfblock-threshold fallback (avoids noisy 3-colour cells) ────────
	{"quad/hb≥1", renderCfg{useQuad: true, quadOpts: quadblock.Options{HalfblockThreshold: 1}}},
	{"quad/hb≥2", renderCfg{useQuad: true, quadOpts: quadblock.Options{HalfblockThreshold: 2}}},
	{"quad/hb≥3", renderCfg{useQuad: true, quadOpts: quadblock.Options{HalfblockThreshold: 3}}},

	// ── quad: neighbourhood blending ────────────────────────────────────────────
	{"quad/blend-ambig", renderCfg{useQuad: true, quadOpts: quadblock.Options{Blend: quadblock.BlendAmbiguous}}},
	{"quad/blend-wide", renderCfg{useQuad: true, quadOpts: quadblock.Options{Blend: quadblock.BlendAmbiguousWide}}},
	{"quad/blend-always", renderCfg{useQuad: true, quadOpts: quadblock.Options{Blend: quadblock.BlendAlways}}},

	// ── quad: combinations ───────────────────────────────────────────────────────
	{"quad/splithalf+ambig", renderCfg{useQuad: true, quadOpts: quadblock.Options{SplitHalf: true, Blend: quadblock.BlendAmbiguous}}},
	{"quad/splithalf+wide", renderCfg{useQuad: true, quadOpts: quadblock.Options{SplitHalf: true, Blend: quadblock.BlendAmbiguousWide}}},
	{"quad/lum+ambig", renderCfg{useQuad: true, quadOpts: quadblock.Options{LumSplit: true, Blend: quadblock.BlendAmbiguous}}},
	{"quad/lum+wide", renderCfg{useQuad: true, quadOpts: quadblock.Options{LumSplit: true, Blend: quadblock.BlendAmbiguousWide}}},
	{"quad/pca2+ambig", renderCfg{useQuad: true, quadOpts: quadblock.Options{PCA2: true, Blend: quadblock.BlendAmbiguous}}},
	{"quad/pca2+wide", renderCfg{useQuad: true, quadOpts: quadblock.Options{PCA2: true, Blend: quadblock.BlendAmbiguousWide}}},
	{"quad/hb2+ambig", renderCfg{useQuad: true, quadOpts: quadblock.Options{HalfblockThreshold: 2, Blend: quadblock.BlendAmbiguous}}},
}

// TestSSIMBenchmark loads every sample image, computes SSIM for all render
// variants, and prints the top-3 per image plus a cross-image summary.
// Run with -v to see full rankings per image.
func TestSSIMBenchmark(t *testing.T) {
	const (
		cols = 80
		rows = 40
	)

	root, err := findModRoot()
	if err != nil {
		t.Skipf("skipping: cannot find module root: %v", err)
	}
	sampleDir := filepath.Join(root, "assets", "samples")
	entries, err := os.ReadDir(sampleDir)
	if err != nil {
		t.Skipf("skipping: cannot read %s: %v", sampleDir, err)
	}
	var samples []string
	for _, e := range entries {
		if !e.IsDir() {
			ext := filepath.Ext(e.Name())
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
				samples = append(samples, filepath.Join(sampleDir, e.Name()))
			}
		}
	}
	if len(samples) == 0 {
		t.Skip("no sample images found")
	}

	type result struct {
		name string
		ssim float64
	}

	sumSSIM := make(map[string]float64, len(allVariants))

	fmt.Println()
	for _, path := range samples {
		orig, err := halfblock.LoadImage(path)
		if err != nil {
			t.Logf("skip %s: %v", path, err)
			continue
		}

		var results []result
		for _, v := range allVariants {
			vp := v.rc.scaleToFit(orig, cols, rows)
			b := vp.Bounds()
			ref := boxDownscale(orig, b.Dx(), b.Dy())
			s := renderSSIM(ref, vp, v.rc)
			results = append(results, result{v.name, s})
			sumSSIM[v.name] += s
		}

		sort.Slice(results, func(i, j int) bool { return results[i].ssim > results[j].ssim })

		fmt.Printf("%-50s\n", filepath.Base(path))
		for i, r := range results {
			marker := "   "
			if i < 3 {
				marker = fmt.Sprintf("#%d ", i+1)
			}
			fmt.Printf("  %s %-26s SSIM:%.4f\n", marker, r.name, r.ssim)
			if !testing.Verbose() && i == 2 {
				break
			}
		}
		fmt.Println()
	}

	// Cross-image summary: average SSIM over all images.
	var avgs []result
	for _, v := range allVariants {
		avgs = append(avgs, result{v.name, sumSSIM[v.name] / float64(len(samples))})
	}
	sort.Slice(avgs, func(i, j int) bool { return avgs[i].ssim > avgs[j].ssim })

	fmt.Println("── Overall average SSIM ─────────────────────────────")
	for i, a := range avgs {
		marker := "   "
		if i < 3 {
			marker = fmt.Sprintf("#%d ", i+1)
		}
		fmt.Printf("  %s %-26s SSIM:%.4f\n", marker, a.name, a.ssim)
		if !testing.Verbose() && i == 4 {
			break
		}
	}
	fmt.Println()
}
