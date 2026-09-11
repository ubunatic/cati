package cmd

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/sparkline/testhelper"
)

type samplePresetInfo struct {
	name        string
	category    string // "geometric", "photo", "icon"
	description string
	path        string
}

var samplePresets = []samplePresetInfo{
	// Geometric synthetic patterns
	{name: "circle", category: "geometric", description: "Synthetic 20x20 circle pattern", path: filepath.Join("testdata", "demo_circle_20x20", "source.png")},
	{name: "checker", category: "geometric", description: "Synthetic 20x20 checkerboard pattern", path: filepath.Join("testdata", "demo_checker_20x20", "source.png")},
	{name: "cross", category: "geometric", description: "Synthetic 20x20 cross pattern", path: filepath.Join("testdata", "demo_cross_20x20", "source.png")},
	{name: "diag", category: "geometric", description: "Synthetic 20x20 diagonal pattern", path: filepath.Join("testdata", "demo_diag_20x20", "source.png")},
	{name: "horiz", category: "geometric", description: "Synthetic 20x20 horizontal gradient", path: filepath.Join("testdata", "demo_horiz_20x20", "source.png")},
	{name: "verti", category: "geometric", description: "Synthetic 20x20 vertical gradient", path: filepath.Join("testdata", "demo_verti_20x20", "source.png")},
	{name: "gradient", category: "geometric", description: "Synthetic horizontal gradient", path: filepath.Join("testdata", "demo_horiz_20x20", "source.png")},

	// Photographic test assets
	{name: "soldering", category: "photo", description: "Photograph: Soldering practice 2025", path: filepath.Join("assets", "samples", "sample-001-soldering-practice-2025.jpg")},
	{name: "summer", category: "photo", description: "Photograph: Summer vacation", path: filepath.Join("assets", "samples", "sample-002-summer-vacation.jpg")},
	{name: "darth", category: "photo", description: "Photograph: Darth daughter", path: filepath.Join("assets", "samples", "sample-003-darth-daughter.jpg")},

	// Icons / Logos
	{name: "emojig", category: "icon", description: "Emojig SVG logo", path: filepath.Join("testdata", "emojig-icon.svg")},
	{name: "logo", category: "icon", description: "Embedded Cati PNG logo", path: ""},
	{name: "cati", category: "icon", description: "Embedded Cati PNG logo", path: ""},
}

func listSamplePresets(out io.Writer) error {
	fmt.Fprintln(out, "Available test asset sample presets:")
	categories := []struct {
		key   string
		label string
	}{
		{"geometric", "Geometric patterns"},
		{"photo", "Photographic samples"},
		{"icon", "Icons and logos"},
	}
	for _, cat := range categories {
		fmt.Fprintf(out, "  %s:\n", cat.label)
		for _, p := range samplePresets {
			if p.category == cat.key && p.name != "cati" {
				fmt.Fprintf(out, "    %-10s %s\n", p.name, p.description)
			}
		}
	}
	return nil
}

func ensurePresetFixture(path string) {
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = testhelper.GenerateGradients("testdata")
	_ = testhelper.GenerateFixtures("testdata")
	_ = testhelper.GenerateGeometrics("testdata")
}

type loadedImage struct {
	name string
	img  image.Image
}

func loadNamedImage(ref string) (loadedImage, error) {
	refClean := strings.TrimSpace(ref)
	if refClean == "" {
		return loadedImage{}, fmt.Errorf("empty image reference")
	}
	lower := strings.ToLower(refClean)
	for _, p := range samplePresets {
		if p.name == lower {
			if p.path == "" {
				img, err := decodeEmbeddedLogo()
				if err != nil {
					return loadedImage{}, err
				}
				return loadedImage{name: p.name, img: img}, nil
			}
			ensurePresetFixture(p.path)
			img, err := halfblock.LoadImage(p.path)
			if err != nil {
				return loadedImage{}, fmt.Errorf("load preset %q (%s): %w", p.name, p.path, err)
			}
			return loadedImage{name: p.name, img: img}, nil
		}
	}
	img, err := halfblock.LoadImage(refClean)
	if err != nil {
		return loadedImage{}, fmt.Errorf("load image %q: %w", refClean, err)
	}
	name := filepath.Base(refClean)
	return loadedImage{name: name, img: img}, nil
}
