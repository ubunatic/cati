package halfblock

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsSVG(t *testing.T) {
	cases := map[string]bool{
		"image.svg":  true,
		"image.SVG":  true,
		"image.png":  false,
		"image.jpeg": false,
		"video.mp4":  false,
	}
	for path, want := range cases {
		if got := IsSVG(path); got != want {
			t.Errorf("IsSVG(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestProbeSVGDimensions(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantW     int
		wantH     int
		wantErrOK bool // true if an error is expected
	}{
		{
			name:    "width_height",
			content: `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="64"></svg>`,
			wantW:   128,
			wantH:   64,
		},
		{
			name:    "width_height_px",
			content: `<svg xmlns="http://www.w3.org/2000/svg" width="128px" height="64px"></svg>`,
			wantW:   128,
			wantH:   64,
		},
		{
			name:    "absolute_cm_units_are_css_pixels",
			content: `<svg xmlns="http://www.w3.org/2000/svg" width="5cm" height="5cm"></svg>`,
			wantW:   189,
			wantH:   189,
		},
		{
			name:    "absolute_in_units_are_css_pixels",
			content: `<svg xmlns="http://www.w3.org/2000/svg" width="2in" height="1in"></svg>`,
			wantW:   192,
			wantH:   96,
		},
		{
			name:    "one_absolute_dimension_uses_viewbox_aspect",
			content: `<svg xmlns="http://www.w3.org/2000/svg" width="5cm" viewBox="0 0 2 1"></svg>`,
			wantW:   189,
			wantH:   95,
		},
		{
			name:    "viewbox_fallback",
			content: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 96"></svg>`,
			wantW:   256,
			wantH:   96,
		},
		{
			name:    "percentage_falls_back_to_viewbox",
			content: `<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%" viewBox="0 0 40 20"></svg>`,
			wantW:   40,
			wantH:   20,
		},
		{
			name:      "no_dimensions",
			content:   `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
			wantErrOK: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.svg")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			w, h, err := ProbeSVGDimensions(path)
			if tc.wantErrOK {
				if err == nil {
					t.Fatalf("ProbeSVGDimensions(%q) = (%d, %d, nil), want error", tc.name, w, h)
				}
				return
			}
			if err != nil {
				t.Fatalf("ProbeSVGDimensions(%q): %v", tc.name, err)
			}
			if w != tc.wantW || h != tc.wantH {
				t.Errorf("ProbeSVGDimensions(%q) = (%d, %d), want (%d, %d)", tc.name, w, h, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestFitSVGTarget(t *testing.T) {
	cases := []struct {
		name       string
		srcW, srcH int
		maxW, maxH int
		wantW      int
		wantH      int
	}{
		{
			name:  "fallback_uses_long_edge_cap_for_wide_svg",
			srcW:  120,
			srcH:  60,
			wantW: SVGMaxDim,
			wantH: 1024,
		},
		{
			name:  "target_width_preserves_aspect",
			srcW:  120,
			srcH:  60,
			maxW:  40,
			wantW: 40,
			wantH: 20,
		},
		{
			name:  "target_height_preserves_aspect",
			srcW:  120,
			srcH:  60,
			maxH:  10,
			wantW: 20,
			wantH: 10,
		},
		{
			name:  "target_box_uses_tighter_axis",
			srcW:  120,
			srcH:  60,
			maxW:  80,
			maxH:  20,
			wantW: 40,
			wantH: 20,
		},
		{
			name:  "no_upscale_when_target_larger_than_declared_size",
			srcW:  120,
			srcH:  60,
			maxW:  240,
			maxH:  120,
			wantW: 120,
			wantH: 60,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotW, gotH := fitSVGTarget(tc.srcW, tc.srcH, tc.maxW, tc.maxH)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Fatalf("fitSVGTarget(%d,%d,%d,%d) = (%d,%d), want (%d,%d)",
					tc.srcW, tc.srcH, tc.maxW, tc.maxH, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestLoadImageSVG(t *testing.T) {
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		t.Skip("rsvg-convert not found on $PATH — skipping SVG rasterization test")
	}
	// sample.svg is a flat single-shape fixture; emojig-icon.svg is a
	// real-world multi-path/gradient/group icon, exercising more of
	// rsvg-convert's rendering surface.
	for _, name := range []string{"sample.svg", "emojig-icon.svg"} {
		t.Run(name, func(t *testing.T) {
			img, err := LoadImage("../../testdata/" + name)
			if err != nil {
				t.Fatalf("LoadImage(%s): %v", name, err)
			}
			b := img.Bounds()
			if b.Dx() <= 0 || b.Dy() <= 0 {
				t.Fatalf("LoadImage(%s) bounds = %v, want positive dimensions", name, b)
			}
		})
	}
}

func TestLoadImageWithTargetSVG(t *testing.T) {
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		t.Skip("rsvg-convert not found on $PATH — skipping SVG rasterization test")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "target.svg")
	if err := os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="60"><rect width="120" height="60" fill="red"/></svg>`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	img, err := LoadImageWithTarget(path, 40, 20)
	if err != nil {
		t.Fatalf("LoadImageWithTarget: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 40 || b.Dy() != 20 {
		t.Fatalf("LoadImageWithTarget bounds = %dx%d, want 40x20", b.Dx(), b.Dy())
	}
}
