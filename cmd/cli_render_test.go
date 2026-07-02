package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestCLIRender exercises the same code path as:
//
//	cati -m MODE -w N <source>
//
// The ANSI output is compared against goldens stored under testdata/<folder>/.
// Run with -update to regenerate all goldens.
func TestCLIRender(t *testing.T) {
	type imageCase struct {
		folder string
		widths []int
	}
	imageCases := []imageCase{
		{"demo_verti_20x20", []int{2, 4, 5, 10, 20}},
		// Vertical-split: boundary char, partial rows, and nose regression.
		{"demo_vert_split_8x8", []int{1, 2, 3, 4, 5}},
	}

	modes := []struct{ name, mode string }{
		{"halfblock", "half"},
		{"quad", "quad"},
		{"spark", "spark+quad"},
		{"spark_best", "spark+six"},
		{"sextant", "six"},
	}

	for _, ic := range imageCases {
		sourcePath := filepath.Join("testdata", ic.folder, "source.png")
		source := goldenLoad(t, sourcePath)
		if source == nil {
			t.Errorf("source not found: %s", sourcePath)
			continue
		}
		for _, pair := range modes {
			rc, err := findRenderModeByName(pair.mode)
			if err != nil {
				t.Fatalf("findRenderModeByName(%q): %v", pair.mode, err)
			}
			for _, w := range ic.widths {
				name := fmt.Sprintf("%s_w%d", pair.name, w)
				t.Run(ic.folder+"/"+name, func(t *testing.T) {
					img := prepareRenderedImage(source, nil, w, 0, rc, "")

					var buf bytes.Buffer
					if err := rc.render(&buf, img); err != nil {
						t.Fatalf("render: %v", err)
					}

					goldenPath := filepath.Join("testdata", ic.folder, "cli_"+name+".ansi")

					if *updateGolden {
						if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
							t.Errorf("write golden %s: %v", goldenPath, err)
						}
						return
					}

					if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
						t.Logf("creating golden %s", goldenPath)
						if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
							t.Errorf("write golden %s: %v", goldenPath, err)
						}
						return
					}

					expected, err := os.ReadFile(goldenPath)
					if err != nil {
						t.Fatalf("read golden %s: %v", goldenPath, err)
					}
					if !bytes.Equal(buf.Bytes(), expected) {
						t.Errorf("%s: ANSI output differs from golden", name)
					}
				})
			}
		}
	}
}
