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
