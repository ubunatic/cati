package sparkline

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

// TestSmallWidthModeMatrix is the reproducible smoke matrix used by #041.
// It checks geometry, ANSI serial/worker parity, and reconstruction parity
// across the user-facing sparkline family without depending on golden files.
func TestSmallWidthModeMatrix(t *testing.T) {
	fixtures := map[string]image.Image{
		"synthetic": syntheticAssessmentImage(),
	}
	for _, fixture := range []struct {
		name string
		path string
	}{
		{"cati", "../../assets/cati_0001.png"},
		{"demo_diag_20x20", "../../testdata/demo_diag_20x20/source.png"},
		{"demo_circle_20x20", "../../testdata/demo_circle_20x20/source.png"},
		{"demo_checker_20x20", "../../testdata/demo_checker_20x20/source.png"},
		{"demo_cross_20x20", "../../testdata/demo_cross_20x20/source.png"},
	} {
		f, err := os.Open(fixture.path)
		if err != nil {
			t.Fatalf("open %s: %v", fixture.path, err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatalf("decode %s: %v", fixture.path, err)
		}
		fixtures[fixture.name] = img
	}

	modes := []Mode{HalfSplit, Spark, Quad, SixHalf, Best}
	for name, img := range fixtures {
		for _, mode := range modes {
			for width := 8; width <= 20; width++ {
				t.Run(fmt.Sprintf("%s/%s/width-%d", name, ModeName(mode), width), func(t *testing.T) {
					grid, err := RenderToGrid(img, width, Options{Mode: mode})
					if err != nil {
						t.Fatalf("RenderToGrid: %v", err)
					}
					if grid.Width != width || grid.Height == 0 {
						t.Fatalf("grid = %dx%d, want width %d and non-zero height", grid.Width, grid.Height, width)
					}

					var serial, parallel strings.Builder
					if err := Render(&serial, img, width, Options{Mode: mode}); err != nil {
						t.Fatalf("Render serial: %v", err)
					}
					if err := RenderJ(&parallel, img, width, 0, mode, 4); err != nil {
						t.Fatalf("Render worker: %v", err)
					}
					if serial.String() != parallel.String() {
						t.Fatal("serial and worker ANSI output differ")
					}

					serialImage := RenderToImage(img, width, grid.Height, mode)
					parallelImage := RenderToImageJ(img, width, grid.Height, mode, 4)
					if !imagesEqual(serialImage, parallelImage) {
						t.Fatal("serial and worker reconstruction differ")
					}
				})
			}
		}
	}
}

func syntheticAssessmentImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if x < 20 && y < 20 {
				img.SetRGBA(x, y, color.RGBA{R: 240, G: 180, A: 255})
			} else if (x+y)%7 == 0 {
				img.SetRGBA(x, y, color.RGBA{B: 220, A: 255})
			}
		}
	}
	return img
}

func imagesEqual(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}
