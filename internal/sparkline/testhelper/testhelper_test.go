package testhelper

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/ubunatic/cati/internal/sparkline"
)

var updateGolden = flag.Bool("update", false, "update golden expected images in testdata")

type testCase struct {
	folderName string
	outCols    int
	outRows    int
}

func TestGradientRenders(t *testing.T) {
	// Locate testdata directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	testdataDir := filepath.Join(wd, "..", "..", "..", "testdata")

	// Ensure gradients are generated
	if err := GenerateGradients(testdataDir); err != nil {
		t.Fatalf("failed to generate gradients: %v", err)
	}

	testCases := []testCase{
		{"demo_horiz_20x20", 5, 5},
		{"demo_horiz_20x20", 2, 2},
		{"demo_verti_20x20", 5, 5},
		{"demo_verti_20x20", 2, 2},
		{"demo_horiz_4x4", 2, 2},
		{"demo_verti_4x4", 2, 2},
		{"demo_horiz_2x2", 1, 1},
		{"demo_verti_2x2", 1, 1},
		{"demo_horiz_1x1", 1, 1},
		{"demo_verti_1x1", 1, 1},
	}

	for _, tc := range testCases {
		folderPath := filepath.Join(testdataDir, tc.folderName)
		sourcePath := filepath.Join(folderPath, "source.png")
		f, err := os.Open(sourcePath)
		if err != nil {
			t.Errorf("failed to open source image in %s: %v", tc.folderName, err)
			continue
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Errorf("failed to decode source image in %s: %v", tc.folderName, err)
			continue
		}

		// Define all algorithms to test
		type algo struct {
			name     string
			render   func(image.Image) image.Image
			metadata map[string]string
		}

		algos := []algo{
			{"halfblock", func(src image.Image) image.Image {
				return RenderHalfblock(src, tc.outCols, tc.outRows)
			}, map[string]string{
				"Algorithm":  "halfblock",
				"Parameters": fmt.Sprintf("outCols=%d, outRows=%d", tc.outCols, tc.outRows),
			}},
			{"quadblock", func(src image.Image) image.Image {
				return RenderQuadblock(src, tc.outCols, tc.outRows)
			}, map[string]string{
				"Algorithm":  "quadblock",
				"Parameters": fmt.Sprintf("outCols=%d, outRows=%d, KMeans=3", tc.outCols, tc.outRows),
			}},
		}

		// Add sparkline algorithms
		for _, mode := range sparkline.Modes() {
			m := mode
			modeName := sparkline.ModeName(m)
			shortMode := filepath.Base(modeName) // e.g. "lower"
			algos = append(algos, algo{
				name: shortMode,
				render: func(src image.Image) image.Image {
					return RenderToImage(src, tc.outCols, tc.outRows, m)
				},
				metadata: map[string]string{
					"Algorithm":  fmt.Sprintf("sparkline (%s)", shortMode),
					"Parameters": fmt.Sprintf("outCols=%d, outRows=%d", tc.outCols, tc.outRows),
				},
			})
		}

		for _, a := range algos {
			renderName := fmt.Sprintf("render_%s_%dx%d.png", a.name, tc.outCols, tc.outRows)
			renderPath := filepath.Join(folderPath, renderName)

			// Perform render
			renderedImg := a.render(img)

			if *updateGolden {
				if err := savePNG(renderPath, renderedImg, a.metadata); err != nil {
					t.Errorf("failed to save golden image %s in %s: %v", renderName, tc.folderName, err)
				}
				continue
			}

			// If golden file doesn't exist, save it as new golden
			if _, err := os.Stat(renderPath); os.IsNotExist(err) {
				t.Logf("Golden file %s in %s does not exist, creating it", renderName, tc.folderName)
				if err := savePNG(renderPath, renderedImg, a.metadata); err != nil {
					t.Errorf("failed to save golden image %s in %s: %v", renderName, tc.folderName, err)
				}
				continue
			}

			// Load golden and compare
			gf, err := os.Open(renderPath)
			if err != nil {
				t.Errorf("failed to open golden image %s in %s: %v", renderName, tc.folderName, err)
				continue
			}
			goldenImg, err := png.Decode(gf)
			gf.Close()
			if err != nil {
				t.Errorf("failed to decode golden image %s in %s: %v", renderName, tc.folderName, err)
				continue
			}

			if !imagesEqual(renderedImg, goldenImg) {
				t.Errorf("Rendered output mismatch for %s under algo %s at %dx%d cells", tc.folderName, a.name, tc.outCols, tc.outRows)
			}
		}
	}
}

func imagesEqual(a, b image.Image) bool {
	ab := a.Bounds()
	bb := b.Bounds()
	if ab != bb {
		return false
	}
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			c1 := a.At(x, y)
			c2 := b.At(x, y)
			r1, g1, b1, a1 := c1.RGBA()
			r2, g2, b2, a2 := c2.RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return false
			}
		}
	}
	return true
}
