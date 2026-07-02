package cmd

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/quadblock"
	"ubunatic.com/cati/v1/sextant"
	"ubunatic.com/cati/v1/sparkline"
	"ubunatic.com/cati/v1/sparkline/testhelper"
)

var updateGolden = flag.Bool("update", false, "overwrite golden render images in testdata")

// TestGoldenRenders runs three render modes through the full pipeline and compares
// against golden PNGs stored at the algorithm's native pixel resolution.
//
// Each test case is a square char request (cols = rows = N), named "Nch".
// For a square source the golden is square (or close to it) with no transparent
// pixels — the exception is when math demands a partial char row (e.g. 5ch
// halfblock from a 20×20 source: 2.5 halfblock char rows → bottom row transparent).
//
// Run with -update to regenerate all goldens.
func TestGoldenRenders(t *testing.T) {
	testdataDir := "testdata"

	if err := testhelper.GenerateGradients(testdataDir); err != nil {
		t.Fatalf("GenerateGradients: %v", err)
	}
	if err := testhelper.GenerateFixtures(testdataDir); err != nil {
		t.Fatalf("GenerateFixtures: %v", err)
	}
	if err := testhelper.GenerateGeometrics(testdataDir); err != nil {
		t.Fatalf("GenerateGeometrics: %v", err)
	}

	type tc struct {
		folder     string
		sourcePath string
		n          int // square char request: cols = rows = n
	}
	cases := []tc{
		{"demo_horiz_20x20", "", 2},
		{"demo_horiz_20x20", "", 4},
		{"demo_horiz_20x20", "", 5},
		{"demo_horiz_20x20", "", 10},
		{"demo_horiz_20x20", "", 20},
		{"demo_verti_20x20", "", 2},
		{"demo_verti_20x20", "", 4},
		{"demo_verti_20x20", "", 5},
		{"demo_verti_20x20", "", 10},
		{"demo_verti_20x20", "", 20},
		// Small gradient edge cases.
		{"demo_horiz_4x4", "", 1},
		{"demo_horiz_4x4", "", 2},
		{"demo_horiz_4x4", "", 4},
		{"demo_verti_4x4", "", 1},
		{"demo_verti_4x4", "", 2},
		{"demo_verti_4x4", "", 4},
		{"demo_horiz_2x2", "", 1},
		{"demo_horiz_2x2", "", 2},
		{"demo_verti_2x2", "", 1},
		{"demo_verti_2x2", "", 2},
		{"demo_horiz_1x1", "", 1},
		{"demo_verti_1x1", "", 1},
		// Vertical split: test boundary char, partial rows, and nose regression.
		{"demo_vert_split_8x8", "", 1},
		{"demo_vert_split_8x8", "", 2},
		{"demo_vert_split_8x8", "", 3},
		{"demo_vert_split_8x8", "", 4},
		{"demo_vert_split_8x8", "", 5},
		// Solid-colour regression: w=1 must produce ▀ (half-height), not █ (full).
		{"solid_red_4x4", "", 1},
		{"solid_red_4x4", "", 2},
		// Geometric shapes: diagonal edge, curved edge, high-frequency, straight bars.
		{"demo_diag_20x20", "", 2},
		{"demo_diag_20x20", "", 4},
		{"demo_diag_20x20", "", 5},
		{"demo_diag_20x20", "", 10},
		{"demo_diag_20x20", "", 20},
		{"demo_circle_20x20", "", 2},
		{"demo_circle_20x20", "", 4},
		{"demo_circle_20x20", "", 5},
		{"demo_circle_20x20", "", 10},
		{"demo_circle_20x20", "", 20},
		{"demo_checker_20x20", "", 2},
		{"demo_checker_20x20", "", 4},
		{"demo_checker_20x20", "", 5},
		{"demo_checker_20x20", "", 10},
		{"demo_checker_20x20", "", 20},
		{"demo_cross_20x20", "", 2},
		{"demo_cross_20x20", "", 4},
		{"demo_cross_20x20", "", 5},
		{"demo_cross_20x20", "", 10},
		{"demo_cross_20x20", "", 20},
		// Photo samples used by the demo make targets.
		{"sample_darth_daughter", "assets/samples/sample-003-darth-daughter.jpg", 24},
		{"sample_darth_daughter", "assets/samples/sample-003-darth-daughter.jpg", 30},
		{"sample_darth_daughter", "assets/samples/sample-003-darth-daughter.jpg", 50},
		{"sample_soldering_practice", "assets/samples/sample-001-soldering-practice-2025.jpg", 24},
		{"sample_soldering_practice", "assets/samples/sample-001-soldering-practice-2025.jpg", 30},
		{"sample_soldering_practice", "assets/samples/sample-001-soldering-practice-2025.jpg", 50},
		{"sample_soldering_practice", "assets/samples/sample-001-soldering-practice-2025.jpg", 80},
		{"sample_summer_vacation", "assets/samples/sample-002-summer-vacation.jpg", 24},
		{"sample_summer_vacation", "assets/samples/sample-002-summer-vacation.jpg", 30},
		{"sample_summer_vacation", "assets/samples/sample-002-summer-vacation.jpg", 50},
	}

	type algoSpec struct {
		name string
		rc   renderCfg
	}
	var algos []algoSpec
	for _, pair := range []struct{ name, mode string }{
		{"halfblock", "half"},
		{"quad", "quad"},
		{"spark", "spark+quad"},
		{"spark_best", "spark+six"},
		{"sextant", "six"},
	} {
		rc, err := findRenderModeByName(pair.mode)
		if err != nil {
			t.Fatalf("findRenderModeByName(%q): %v", pair.mode, err)
		}
		algos = append(algos, algoSpec{pair.name, rc})
	}

	for _, c := range cases {
		folderPath := filepath.Join(testdataDir, c.folder)
		sourcePath := c.sourcePath
		if sourcePath == "" {
			sourcePath = filepath.Join(folderPath, "source.png")
		}
		orig := goldenSourceLoad(t, sourcePath)
		if orig == nil {
			continue
		}
		baseRC, err := findRenderModeByName("half")
		if err != nil {
			t.Fatalf("findRenderModeByName(%q): %v", "half", err)
		}
		baseScaled := prepareRenderedImage(orig, nil, c.n, c.n, baseRC, "")
		baseRender := goldenRenderToImage(baseScaled, baseRC, 0, 0)
		baseBounds := baseRender.Bounds()
		if err := os.MkdirAll(folderPath, 0o755); err != nil {
			t.Fatalf("create %s: %v", folderPath, err)
		}
		for _, a := range algos {
			renderName := fmt.Sprintf("render_%s_%dch.png", a.name, c.n)
			renderPath := filepath.Join(folderPath, renderName)
			meta := map[string]string{
				"Algorithm": a.name,
				"Chars":     fmt.Sprintf("%d", c.n),
			}

			scaled := prepareRenderedImage(orig, nil, c.n, c.n, a.rc, "")
			rendered := goldenRenderToImage(scaled, a.rc, baseBounds.Dx(), baseBounds.Dy())

			if *updateGolden {
				if err := testhelper.SavePNG(renderPath, rendered, meta); err != nil {
					t.Errorf("save golden %s: %v", renderPath, err)
				}
				continue
			}

			if _, err := os.Stat(renderPath); os.IsNotExist(err) {
				t.Logf("creating golden %s", renderName)
				if err := testhelper.SavePNG(renderPath, rendered, meta); err != nil {
					t.Errorf("save golden %s: %v", renderPath, err)
				}
				continue
			}

			golden := goldenLoad(t, renderPath)
			if golden == nil {
				continue
			}
			if !goldenEqual(rendered, golden) {
				t.Errorf("%s/%s: rendered image differs from golden", c.folder, renderName)
			}
		}
	}
}

// goldenCharBlock returns the minimal aspect-correct per-character pixel block
// W×H such that every registered render mode's cell geometry divides it exactly
// (integer kX = W/CellW, kY = H/CellH with no remainder) and H = 2·W
// (≈ 1:2 terminal-cell aspect ratio).
//
// For the current four modes (halfblock 1×2, quad 2×2, spark 4×8, sextant 2×3):
//
//	lcm(CellW) = 4,  lcm(CellH) = 24  →  W=12, H=24
//
//	halfblock: kX=12, kY=12  →  12×24 ✓
//	quad:      kX= 6, kY=12  →  12×24 ✓
//	spark:     kX= 3, kY= 3  →  12×24 ✓
//	sextant:   kX= 6, kY= 8  →  12×24 ✓
//
// Adding a mode with a new cell geometry automatically adjusts the block: the
// caller should not cache the result across test runs if the registry changes.
func goldenCharBlock() (W, H int) {
	lcmW, lcmH := 1, 1
	for _, m := range []renderMode{modeHalfblock, modeQuad, modeSpark, modeSparkBest, modeSextant} {
		spec := m.viewSpec()
		lcmW = lcm(lcmW, spec.CellW)
		lcmH = lcm(lcmH, spec.CellH)
	}
	for k := 1; k <= 1000; k++ {
		W = k * lcmW
		H = 2 * W
		if H%lcmH == 0 {
			return W, H
		}
	}
	panic("goldenCharBlock: no valid block found within k=1000")
}

func lcm(a, b int) int { return a / gcd(a, b) * b }
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// goldenRenderToImage renders scaled through the native algorithm and upscales
// the result to the shared comparison canvas via pure integer pixel replication.
//
// Every render mode reaches exactly W×H pixels per character (goldenCharBlock)
// with no NN-smear: kX = W/CellW and kY = H/CellH are always integers.
//
// Transparent extension rows (appended by FitDims when the image ends mid-cell)
// are preserved as transparent pixels by each renderer: halfblock.RenderToImage
// uses transparent for terminal-default BG, sparkline.RenderToImage preserves
// source alpha. No post-processing is needed.
//
// refW/refH are no longer used (all modes produce identical canvas dimensions)
// and are kept only for call-site compatibility; pass 0 to make that explicit.
func goldenRenderToImage(scaled image.Image, rc renderCfg, refW, refH int) image.Image {
	b := scaled.Bounds()
	var rendered image.Image
	switch rc.mode {
	case modeSextant:
		rendered = sextant.RenderToImage(scaled, rc.sextantMode)
	case modeHalfSplit, modeSpark, modeSparkQuad, modeSixHalf, modeSparkSix:
		spec := rc.mode.viewSpec()
		outCols := max(1, b.Dx()/spec.CellW)
		outRows := max(1, b.Dy()/spec.CellH)
		rendered = sparkline.RenderToImage(scaled, outCols, outRows, rc.sparkMode)
	case modeQuad:
		rendered = quadblock.RenderToImage(scaled, rc.quadOpts)
	default:
		rendered = halfblock.RenderToImage(scaled)
	}
	return upscaleToCharRes(rendered, rc)
}

// upscaleToCharRes upscales rendered to the shared character-grid resolution so
// that all algorithm outputs can be normalized onto the same comparison canvas.
// Transparent pixels (BG extension rows) remain transparent.
//
// The target block is computed by goldenCharBlock() — currently 12×24 px/char,
// covering halfblock (1×2), quad (2×2), spark (4×8), and sextant (2×3) exactly:
//
//	halfblock: 1×2px/char → kX=12, kY=12 → 12×24
//	quad:      2×2px/char → kX= 6, kY=12 → 12×24
//	spark:     4×8px/char → kX= 3, kY= 3 → 12×24
//	sextant:   2×3px/char → kX= 6, kY= 8 → 12×24
func upscaleToCharRes(rendered image.Image, rc renderCfg) image.Image {
	blockW, blockH := goldenCharBlock()
	spec := rc.mode.viewSpec()
	kX := blockW / spec.CellW
	kY := blockH / spec.CellH
	if kX <= 1 && kY <= 1 {
		return rendered
	}
	b := rendered.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx()*kX, b.Dy()*kY))
	for y := range out.Bounds().Dy() {
		srcY := b.Min.Y + y/kY
		for x := range out.Bounds().Dx() {
			out.Set(x, y, rendered.At(b.Min.X+x/kX, srcY))
		}
	}
	return out
}

// unrepeat downsamples an upscaled image by taking every kX-th column and
// kY-th row. It is the inverse of the integer replication in upscaleToCharRes
// and lets tests verify that the canvas adds no information and loses none.
func unrepeat(img image.Image, kX, kY int) image.Image {
	b := img.Bounds()
	outW := b.Dx() / kX
	outH := b.Dy() / kY
	out := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for y := range outH {
		for x := range outW {
			out.Set(x, y, img.At(b.Min.X+x*kX, b.Min.Y+y*kY))
		}
	}
	return out
}

// imagesEqual returns true iff a and b have identical bounds and pixel values.
func imagesEqual(a, b image.Image) bool {
	ab, bb := a.Bounds(), b.Bounds()
	if ab != bb {
		return false
	}
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return false
			}
		}
	}
	return true
}

// TestGoldenBlockIntegerFactors asserts that goldenCharBlock() returns a W×H
// such that every registered render mode's cell width divides W and cell height
// divides H exactly (no remainder), guaranteeing pure integer replication.
func TestGoldenBlockIntegerFactors(t *testing.T) {
	blockW, blockH := goldenCharBlock()
	// Expect 12×24 for current mode set; document it explicitly.
	if blockW != 12 || blockH != 24 {
		t.Logf("goldenCharBlock = %d×%d (expected 12×24 for current modes)", blockW, blockH)
	}
	if blockH != 2*blockW {
		t.Errorf("H=%d is not 2·W=%d (aspect invariant violated)", blockH, 2*blockW)
	}
	for _, tc := range []struct {
		name string
		mode renderMode
	}{
		{"halfblock", modeHalfblock},
		{"quad", modeQuad},
		{"spark", modeSpark},
		{"spark_best", modeSparkBest},
		{"sextant", modeSextant},
	} {
		spec := tc.mode.viewSpec()
		if blockW%spec.CellW != 0 {
			t.Errorf("%s: blockW=%d is not divisible by CellW=%d", tc.name, blockW, spec.CellW)
		}
		if blockH%spec.CellH != 0 {
			t.Errorf("%s: blockH=%d is not divisible by CellH=%d", tc.name, blockH, spec.CellH)
		}
		kX := blockW / spec.CellW
		kY := blockH / spec.CellH
		t.Logf("%s: cell %d×%d → kX=%d kY=%d → %d×%d", tc.name, spec.CellW, spec.CellH, kX, kY, spec.CellW*kX, spec.CellH*kY)
	}
}

// TestUnrepeatLossless verifies that un-repeating a golden canvas produced by
// upscaleToCharRes recovers an image pixel-identical to the native render, and
// that every pixel in the upscaled canvas is an exact integer replica of its
// source pixel (no smear, no NN interpolation artefacts).
func TestUnrepeatLossless(t *testing.T) {
	blockW, blockH := goldenCharBlock()

	// Build a small synthetic source with distinct colours at each pixel so that
	// any smearing would be caught immediately.
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			src.SetRGBA(x, y, color.RGBA{R: uint8(x * 60), G: uint8(y * 60), B: uint8((x + y) * 30), A: 255})
		}
	}

	for _, tc := range []struct {
		name string
		rc   renderCfg
	}{
		{"halfblock", renderCfg{mode: modeHalfblock}},
		{"quad", renderCfg{mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true}}},
		{"sextant", renderCfg{mode: modeSextant, sextantMode: sextant.ModeSextant}},
		{"spark", renderCfg{mode: modeSpark, sparkMode: sparkline.Quad}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.rc.mode.viewSpec()
			kX := blockW / spec.CellW
			kY := blockH / spec.CellH

			// Get native render.
			b := src.Bounds()
			var native image.Image
			switch tc.rc.mode {
			case modeSextant:
				native = sextant.RenderToImage(src, tc.rc.sextantMode)
			case modeHalfSplit, modeSpark, modeSparkQuad, modeSixHalf, modeSparkSix:
				spec := tc.rc.mode.viewSpec()
				native = sparkline.RenderToImage(src, max(1, b.Dx()/spec.CellW), max(1, b.Dy()/spec.CellH), tc.rc.sparkMode)
			case modeQuad:
				native = quadblock.RenderToImage(src, tc.rc.quadOpts)
			default:
				native = halfblock.RenderToImage(src)
			}

			// Upscale then un-repeat.
			upscaled := upscaleToCharRes(native, tc.rc)
			recovered := unrepeat(upscaled, kX, kY)

			if !imagesEqual(native, recovered) {
				t.Errorf("%s: unrepeat(upscale(native)) != native — upscale introduced smear", tc.name)
			}

			// Also verify every pixel in upscaled is a pure replica of its source.
			ub := upscaled.Bounds()
			nb := native.Bounds()
			for y := range ub.Dy() {
				for x := range ub.Dx() {
					srcX := nb.Min.X + x/kX
					srcY := nb.Min.Y + y/kY
					want := native.At(srcX, srcY)
					got := upscaled.At(ub.Min.X+x, ub.Min.Y+y)
					wr, wg, wb, wa := want.RGBA()
					gr, gg, gb, ga := got.RGBA()
					if wr != gr || wg != gg || wb != gb || wa != ga {
						t.Errorf("%s: upscaled[%d,%d] = %v, want replica of native[%d,%d] = %v",
							tc.name, x, y, got, srcX-nb.Min.X, srcY-nb.Min.Y, want)
						return
					}
				}
			}
		})
	}
}

func goldenSourceLoad(t *testing.T, path string) image.Image {
	t.Helper()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		img, err := halfblock.LoadImage(path)
		if err != nil {
			t.Errorf("load %s: %v", path, err)
			return nil
		}
		return img
	default:
		return goldenLoad(t, path)
	}
}

func goldenLoad(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Errorf("open %s: %v", path, err)
		return nil
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Errorf("decode %s: %v", path, err)
		return nil
	}
	return img
}

// TestGoldenTransparentBound asserts that no golden PNG has more than half a
// char of transparent BG pixels per column.
//
// Why the pipeline guarantees this: FitDims snaps the partial last row to the
// nearest half-cell boundary, so extH is either 0 (full content cell) or exactly
// CellH/2 (a top half-cell of content plus a transparent bottom half → ▀).
// After integer upscaling by kY = blockH/CellH, the transparent tail is at most
// (CellH/2)×kY = blockH/2 golden pixel rows — exactly half a character block,
// uniform across all algorithms.
func TestGoldenTransparentBound(t *testing.T) {
	_, blockH := goldenCharBlock()
	// halfCharGoldenPx is the maximum transparent rows allowed in any golden:
	// CellH/2 rendered pixels × kY = blockH/2, constant across all modes.
	halfCharGoldenPx := blockH / 2
	paths, err := filepath.Glob("testdata/*/render_*.png")
	if err != nil || len(paths) == 0 {
		t.Skip("no goldens found")
	}
	for _, path := range paths {
		img := goldenLoad(t, path)
		if img == nil {
			continue
		}
		b := img.Bounds()
		var transp int
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				if a == 0 {
					transp++
				}
			}
		}
		if transp > b.Dx()*halfCharGoldenPx {
			t.Errorf("%s: %d transparent pixels > half-char bound (%d)", path, transp, b.Dx()*halfCharGoldenPx)
		}
	}
}

func goldenEqual(a, b image.Image) bool { return imagesEqual(a, b) }

// Ensure math import is used (gcd/lcm helpers use only stdlib arithmetic).
var _ = math.MaxInt
