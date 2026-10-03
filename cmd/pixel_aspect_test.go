package cmd

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/cati/internal/viewgeom"
	"ubunatic.com/cati/v1/sparkline"
	"ubunatic.com/cati/v1/sparkline/testhelper"
)

func TestPixelAspectGolden(t *testing.T) {
	// Nonzero bounds and alternating rows make interpolation, wrong sampling
	// origins and forced 1x vertical repeats observable independently of geometry.
	src := image.NewNRGBA(image.Rect(5, 7, 17, 19))
	palette := []color.NRGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}}
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			src.SetNRGBA(5+x, 7+y, palette[(x+y)%len(palette)])
		}
	}
	rc, err := findRenderModeByName("3x3")
	if err != nil {
		t.Fatal(err)
	}
	for _, aspect := range []string{"pixel", "raw", "1:1"} {
		for _, prescaler := range []prescaleMode{prescaleNearestNeighbor, prescalePyramid} {
			rc.prescaler = prescaler
			prepared, err := prepareRenderPlanImage(src, viewgeom.TargetConstraints{ExplicitCols: 3, AspectMode: aspect}, rc)
			if err != nil {
				t.Fatal(err)
			}
			if got := prepared.Bounds().Size(); got != image.Pt(18, 6) {
				t.Fatalf("%s %s: dimensions %v, want 18x6", aspect, prescaler, got)
			}
			for y := 0; y < 6; y++ {
				for x := 0; x < 18; x++ {
					want := src.At(5+x*12/18, 7+y*2)
					if color.NRGBAModel.Convert(prepared.At(x, y)) != want {
						t.Fatalf("%s %s: pixel (%d,%d) = %v, want source color %v", aspect, prescaler, x, y, prepared.At(x, y), want)
					}
				}
			}
			rendered, err := sparkline.RenderToImageWithOptions(prepared, 3, 2, rc.glyphOptions())
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", "aspect-pixel", "render_3x3_3x2.png")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := testhelper.SavePNG(path, rendered, map[string]string{
					"Algorithm": "3x3", "Aspect": "pixel", "Source": "12x12 alternating RGB",
					"Cols": "3", "Rows": "2", "Sampling": "nearest-neighbor",
					"SnapTolerance": "less than one mode cell",
				}); err != nil {
					t.Fatal(err)
				}
			} else if golden := goldenLoad(t, path); golden != nil && !goldenEqual(rendered, golden) {
				t.Fatalf("%s %s: rendered pixels differ from %s", aspect, prescaler, path)
			}
		}
	}
}
