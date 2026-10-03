package cmd

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/cati/internal/viewgeom"
	"ubunatic.com/cati/spec"
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

func TestPixelAspectPolicyGoldens(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 12, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 20), G: uint8(y * 20), A: 255})
		}
	}
	rc, err := findRenderModeByName("3x3")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := spec.LoadPixelAspectPolicy()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                           string
		cols, rows, contentW, contentH int
	}{
		{"small", 5, 4, 30, 10},
		{"padded", 11, 7, 60, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepared, err := prepareRenderPlanImage(src, viewgeom.TargetConstraints{ExplicitCols: tc.cols, AspectMode: "pixel"}, rc)
			if err != nil {
				t.Fatal(err)
			}
			if prepared.Bounds().Size() != image.Pt(tc.cols*6, tc.rows*3) {
				t.Fatalf("unexpected canvas: %v", prepared.Bounds())
			}
			for y := 0; y < prepared.Bounds().Dy(); y++ {
				for x := 0; x < prepared.Bounds().Dx(); x++ {
					_, _, _, alpha := prepared.At(x, y).RGBA()
					padding := x >= tc.contentW || y >= tc.contentH
					if (alpha == 0) != padding {
						t.Fatalf("(%d,%d): alpha %d, padding=%v", x, y, alpha, padding)
					}
				}
			}
			if tc.name == "padded" {
				for x := 0; x < 60; x++ {
					r, _, _, _ := prepared.At(x, 0).RGBA()
					wantR, _, _, _ := src.At(x/5, 0).RGBA()
					if r != wantR {
						t.Fatalf("column %d does not preserve uniform 5x repeat", x)
					}
				}
			}
			rendered, err := sparkline.RenderToImageWithOptions(prepared, tc.cols, tc.rows, rc.glyphOptions())
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", "aspect-pixel", "render_3x3_"+tc.name+".png")
			if *updateGolden {
				if err := testhelper.SavePNG(path, rendered, map[string]string{
					"Algorithm": "3x3", "Aspect": "pixel", "Source": "12x12 RGB ramps",
					"Cols": fmt.Sprint(tc.cols), "Rows": fmt.Sprint(tc.rows),
					"Content":       fmt.Sprintf("%dx%d", tc.contentW, tc.contentH),
					"MaxDistortion": fmt.Sprint(policy.MaxDistortion), "MaxPadding": fmt.Sprint(policy.MaxPadding),
					"Sampling": "nearest-neighbor",
				}); err != nil {
					t.Fatal(err)
				}
			} else if golden := goldenLoad(t, path); golden != nil && !goldenEqual(rendered, golden) {
				t.Fatalf("render differs from %s", path)
			}
		})
	}
}
