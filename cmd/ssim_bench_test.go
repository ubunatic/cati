package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/pixelart"
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

	// ── pixelart pre-scalers ─────────────────────────────────────────────────────
	// EPX / Scale2x: edge-corner-aware 2× upscale before NN downscale.
	// Helps for high-contrast sharp-edge images (PCB, line art); near-no-op for
	// smooth photos (exact pixel-equality condition rarely fires).
	{"halfblock+epx2x", renderCfg{preScale: pixelart.Scale2x}},
	{"quad/splithalf+epx2x", renderCfg{useQuad: true, quadOpts: quadblock.Options{SplitHalf: true}, preScale: pixelart.Scale2x}},
	{"quad/pca2+epx2x", renderCfg{useQuad: true, quadOpts: quadblock.Options{PCA2: true}, preScale: pixelart.Scale2x}},

	// Scale3x: tripling for higher-quality intermediate before downscale.
	{"halfblock+epx3x", renderCfg{preScale: pixelart.Scale3x}},
	{"quad/splithalf+epx3x", renderCfg{useQuad: true, quadOpts: quadblock.Options{SplitHalf: true}, preScale: pixelart.Scale3x}},

	// Unsharp mask: sharpens edges before downscale — useful for all image types.
	// Brings soft gradients closer to hard transitions → cleaner 2-colour splits.
	{"halfblock+sharp0.5", renderCfg{preScale: pixelart.Sharpen05}},
	{"halfblock+sharp1.0", renderCfg{preScale: pixelart.Sharpen10}},
	{"quad/splithalf+sharp0.5", renderCfg{useQuad: true, quadOpts: quadblock.Options{SplitHalf: true}, preScale: pixelart.Sharpen05}},
	{"quad/splithalf+sharp1.0", renderCfg{useQuad: true, quadOpts: quadblock.Options{SplitHalf: true}, preScale: pixelart.Sharpen10}},
	{"quad/pca2+sharp0.5", renderCfg{useQuad: true, quadOpts: quadblock.Options{PCA2: true}, preScale: pixelart.Sharpen05}},

	// ── edge-snap cell encoder ───────────────────────────────────────────────────
	// Splits each 2×2 cell by the dominant luminance gradient direction within the
	// cell. Operates at the right scale (cell level, not source image level).
	{"quad/edge-snap", renderCfg{useQuad: true, quadOpts: quadblock.Options{EdgeSnap: true}}},
	{"quad/edge-snap+ambig", renderCfg{useQuad: true, quadOpts: quadblock.Options{EdgeSnap: true, Blend: quadblock.BlendAmbiguous}}},
	{"quad/edge-snap+hb3", renderCfg{useQuad: true, quadOpts: quadblock.Options{EdgeSnap: true, HalfblockThreshold: 3}}},
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
		q    RenderQuality
	}

	type cumQ struct{ ssim, blk, edge float64 }
	cum := make(map[string]*cumQ, len(allVariants))
	for _, v := range allVariants {
		cum[v.name] = &cumQ{}
	}

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
			ref := pyramidDownscale(orig, b.Dx(), b.Dy())
			q := computeQuality(ref, vp, v.rc)
			results = append(results, result{v.name, q})
			c := cum[v.name]
			c.ssim += q.SSIM
			c.blk += q.Blockiness
			c.edge += q.EdgeCont
		}

		sort.Slice(results, func(i, j int) bool { return results[i].q.SSIM > results[j].q.SSIM })

		fmt.Printf("%-50s\n", filepath.Base(path))
		fmt.Printf("  %-26s  SSIM   Blk    Edge\n", "variant")
		fmt.Printf("  %-26s  ─────  ─────  ─────\n", "──────────────────────────")
		for i, r := range results {
			marker := "   "
			if i < 3 {
				marker = fmt.Sprintf("#%d ", i+1)
			}
			fmt.Printf("  %s %-26s %.3f  %.3f  %.3f\n",
				marker, r.name, r.q.SSIM, r.q.Blockiness, r.q.EdgeCont)
			if !testing.Verbose() && i == 2 {
				break
			}
		}
		fmt.Println()
	}

	// Cross-image summary: average over all images, sorted by SSIM.
	type avg struct {
		name string
		q    cumQ
	}
	n := float64(len(samples))
	var avgs []avg
	for _, v := range allVariants {
		c := cum[v.name]
		avgs = append(avgs, avg{v.name, cumQ{c.ssim / n, c.blk / n, c.edge / n}})
	}
	sort.Slice(avgs, func(i, j int) bool { return avgs[i].q.ssim > avgs[j].q.ssim })

	fmt.Println("── Overall average (pyramid ref) ────────────────────────────────")
	fmt.Printf("  %-26s  SSIM   Blk    Edge\n", "variant")
	fmt.Printf("  %-26s  ─────  ─────  ─────\n", "──────────────────────────")
	for i, a := range avgs {
		marker := "   "
		if i < 3 {
			marker = fmt.Sprintf("#%d ", i+1)
		}
		fmt.Printf("  %s %-26s %.3f  %.3f  %.3f\n",
			marker, a.name, a.q.ssim, a.q.blk, a.q.edge)
		if !testing.Verbose() && i == 4 {
			break
		}
	}
	fmt.Println()
}
