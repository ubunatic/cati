package quadblock

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/cati/v1/halfblock"
)

// ── side-by-side rendering helpers ───────────────────────────────────────────

// visLen returns the visible display width of s, ignoring ANSI escape sequences.
func visLen(s string) int {
	inEsc := false
	n := 0
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc = true
		case inEsc && r == 'm':
			inEsc = false
		case !inEsc:
			n++
		}
	}
	return n
}

// padRight pads s with spaces to reach visible display width w.
func padRight(s string, w int) string {
	if pad := w - visLen(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// maxVisLen returns the longest visible display width across all lines.
func maxVisLen(lines []string) int {
	w := 0
	for _, l := range lines {
		if v := visLen(l); v > w {
			w = v
		}
	}
	return w
}

// renderLines renders img via fn, strips the ansiLinePrefix from every line,
// and returns one string per terminal row.
func renderLines(t *testing.T, img image.Image, fn func(io.Writer, image.Image) error) []string {
	t.Helper()
	var sb strings.Builder
	if err := fn(&sb, img); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := strings.ReplaceAll(sb.String(), ansiLinePrefix, "")
	return strings.Split(strings.TrimRight(out, "\n"), "\n")
}

// ── variant definitions ───────────────────────────────────────────────────────

type variant struct {
	label    string         // shown in the column header (truncated to showCols)
	opts     Options        // quad options; ignored when useHalf=true
	useHalf  bool           // render with halfblock instead of quad
	colorRed ColorReduction // palette applied before rendering (ColorFull = none)
}

// showVariants lists all rendering modes shown in TestShowImages.
// Variants are printed in groups of maxVariantsPerRow columns.
//
// Row 1: algorithm variants
// Row 2: colour-space reduction (all use default quad Options)
var showVariants = []variant{
	// ── Row 1: algorithm variants ─────────────────────────────────────────────
	{label: "quad/default", opts: Options{}},
	{label: "quad/hb≥2", opts: Options{HalfblockThreshold: 2}},
	{label: "quad/split½", opts: Options{SplitHalf: true}},
	{label: "split½+nb", opts: Options{SplitHalf: true, SplitHalfNeighbors: true}},
	{label: "lum-split", opts: Options{LumSplit: true}},
	{label: "halfblock", useHalf: true},

	// ── Row 2: colour-space reduction ─────────────────────────────────────────
	{label: "ansi256", colorRed: ColorANSI256},
	{label: "ansi16", colorRed: ColorANSI16},
	{label: "gray8", colorRed: ColorGray8},
	{label: "gray16", colorRed: ColorGray16},
	{label: "gray64", colorRed: ColorGray64},
}

const (
	showCols          = 24 // fixed terminal columns per variant column
	showSep           = "  "
	maxVariantsPerRow = 6 // wrap to a new row of columns after this many variants
)

// TestShowImages renders each image in all quality variants side by side.
// Variants are grouped into rows of maxVariantsPerRow columns.
// Run with: go test ./internal/quadblock/ -v -run TestShowImages
func TestShowImages(t *testing.T) {
	if !testing.Verbose() {
		t.Skip("only runs with -v")
	}

	dirs := []struct {
		dir  string
		rows int // 0 = unconstrained (aspect-ratio derived)
	}{
		{"../../testdata", 0},
		{"../../assets/samples", 0},
	}

	for _, d := range dirs {
		entries, err := os.ReadDir(d.dir)
		if err != nil {
			t.Logf("skip dir %s: %v", d.dir, err)
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(d.dir, e.Name())
			orig, err := halfblock.LoadImage(path)
			if err != nil {
				t.Logf("skip %s: %v", path, err)
				continue
			}

			// Pre-scale once per renderer type; all quad variants share one scaled image.
			quadImg := ScaleToFit(orig, showCols, d.rows)
			halfImg := halfblock.ScaleToFit(orig, showCols, d.rows)

			// Render all variants to line slices.
			type rendered struct {
				v     variant
				lines []string
				cols  int
			}
			var columns []rendered
			for _, v := range showVariants {
				var src image.Image
				if v.useHalf {
					src = halfImg
				} else {
					src = quadImg
				}
				if v.colorRed != ColorFull {
					src = ReduceColors(src, v.colorRed)
				}

				var lines []string
				if v.useHalf {
					lines = renderLines(t, src, func(w io.Writer, img image.Image) error {
						return halfblock.Render(w, img, img.Bounds().Dx(), halfblock.Options{})
					})
				} else {
					o := v.opts
					lines = renderLines(t, src, func(w io.Writer, img image.Image) error {
						return Render(w, img, img.Bounds().Dx(), o)
					})
				}
				cols := maxVisLen(lines)
				if cols == 0 {
					cols = showCols
				}
				columns = append(columns, rendered{v: v, lines: lines, cols: cols})
			}

			// Print header (file path) once per image.
			fmt.Printf("\n── %s ──\n", path)

			// Print variant groups (up to maxVariantsPerRow columns per group).
			for start := 0; start < len(columns); start += maxVariantsPerRow {
				end := start + maxVariantsPerRow
				if end > len(columns) {
					end = len(columns)
				}
				group := columns[start:end]

				// Variant labels.
				for i, c := range group {
					if i > 0 {
						fmt.Print(showSep)
					}
					label := c.v.label
					if visLen(label) > c.cols {
						label = label[:c.cols]
					}
					fmt.Print(padRight(label, c.cols))
				}
				fmt.Println()

				// Image rows.
				nRows := 0
				for _, c := range group {
					if len(c.lines) > nRows {
						nRows = len(c.lines)
					}
				}
				for row := range nRows {
					for i, c := range group {
						if i > 0 {
							fmt.Print(showSep)
						}
						line := ""
						if row < len(c.lines) {
							line = c.lines[row]
						}
						fmt.Print(padRight(line, c.cols))
					}
					fmt.Println()
				}
			}
		}
	}
}
