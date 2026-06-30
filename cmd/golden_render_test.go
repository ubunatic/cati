package cmd

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/imgutil"
	"codeberg.org/ubunatic/cati/internal/quadblock"
	"codeberg.org/ubunatic/cati/internal/sextant"
	"codeberg.org/ubunatic/cati/internal/sparkline"
	"codeberg.org/ubunatic/cati/internal/sparkline/testhelper"
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
		{"halfblock", "halfblock"},
		{"quad", "quad/splithalf"},
		{"spark", "spark/quad"},
		{"spark_best", "spark/best"},
		{"sextant", "sextant/2x3"},
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
		baseRC, err := findRenderModeByName("halfblock")
		if err != nil {
			t.Fatalf("findRenderModeByName(%q): %v", "halfblock", err)
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

// goldenRenderToImage renders scaled through the native algorithm, normalizes
// the result onto the shared comparison canvas, and then upscales to a common
// per-cell resolution for golden comparison.
//
// Transparent extension rows (appended by FitDims when the image ends mid-cell)
// are preserved as transparent pixels by each renderer: halfblock.RenderToImage
// uses transparent for terminal-default BG, sparkline.RenderToImage preserves
// source alpha. No post-processing is needed.
func goldenRenderToImage(scaled image.Image, rc renderCfg, refW, refH int) image.Image {
	b := scaled.Bounds()
	var rendered image.Image
	switch rc.mode {
	case modeSextant:
		rendered = sextant.RenderToImage(scaled, rc.sextantMode)
	case modeSpark, modeSparkBest:
		outCols := max(1, b.Dx()/4)
		outRows := max(1, b.Dy()/8)
		rendered = sparkline.RenderToImage(scaled, outCols, outRows, rc.sparkMode)
	case modeQuad:
		rendered = quadblock.RenderToImage(scaled, rc.quadOpts)
	default:
		rendered = halfblock.RenderToImage(scaled)
	}
	rendered = upscaleToCharRes(rendered, rc)
	if refW > 0 && refH > 0 {
		rendered = imgutil.ScaleNN(rendered, refW, refH)
	}
	return rendered
}

// upscaleToCharRes upscales rendered to the shared character-grid resolution so
// that all algorithm outputs can be normalized onto the same comparison canvas.
// Transparent pixels (BG extension rows) remain transparent.
//
//	halfblock: 1×2px/char → scale 4×4 → 4×8
//	quad:      2×2px/char → scale 2×4 → 4×8
//	spark:     4×8px/char → scale 1×1 → 4×8 (no-op)
func upscaleToCharRes(rendered image.Image, rc renderCfg) image.Image {
	spec := rc.mode.viewSpec()
	scaleX := 4 / spec.CellW
	scaleY := 8 / spec.CellH
	if scaleX <= 1 && scaleY <= 1 {
		return rendered
	}
	b := rendered.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx()*scaleX, b.Dy()*scaleY))
	for y := range out.Bounds().Dy() {
		srcY := b.Min.Y + y/scaleY
		for x := range out.Bounds().Dx() {
			out.Set(x, y, rendered.At(b.Min.X+x/scaleX, srcY))
		}
	}
	return out
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
// char of transparent BG pixels per column (= 4 golden pixel rows).
//
// Why the pipeline guarantees this: FitDims snaps the partial last row to the
// nearest half-cell boundary, so extH is either 0 (full content cell) or exactly
// CellH/2 (a top half-cell of content plus a transparent bottom half → ▀).
// Therefore extH ≤ CellH/2 rendered pixels, which upscales to
// (CellH/2)×(8/CellH) = 4 golden pixel rows — exactly half a char, regardless
// of algorithm.
func TestGoldenTransparentBound(t *testing.T) {
	// halfCharGoldenPx is the maximum transparent rows allowed in any golden:
	// CellH/2 rendered pixels × scaleY = (CellH/2)×(8/CellH) = 4, always.
	const halfCharGoldenPx = 4
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

func goldenEqual(a, b image.Image) bool {
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
