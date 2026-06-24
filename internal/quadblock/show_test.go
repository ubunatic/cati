package quadblock

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/ubunatic/cati/internal/halfblock"
)

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

// renderLines renders img via fn and returns one string per terminal row,
// with the ansiLinePrefix (\x1b[2K\r) stripped so lines can be composed.
func renderLines(t *testing.T, img image.Image, fn func(io.Writer, image.Image) error) []string {
	t.Helper()
	var sb strings.Builder
	if err := fn(&sb, img); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := strings.ReplaceAll(sb.String(), ansiLinePrefix, "")
	return strings.Split(strings.TrimRight(out, "\n"), "\n")
}

// maxVisLen returns the longest visible display width across lines.
func maxVisLen(lines []string) int {
	w := 0
	for _, l := range lines {
		if v := visLen(l); v > w {
			w = v
		}
	}
	return w
}

// TestShowImages prints quad and half-block renders of each image side by side.
// Run with: go test ./internal/quadblock/ -v -run TestShowImages
func TestShowImages(t *testing.T) {
	if !testing.Verbose() {
		t.Skip("only runs with -v")
	}

	dirs := []struct {
		dir  string
		cols int
		rows int
	}{
		{"../../testdata", 20, 10},
		{"../../assets/samples", 60, 20},
	}

	const sep = "   "

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

			quadImg := ScaleToFit(orig, d.cols, d.rows)
			halfImg := halfblock.ScaleToFit(orig, d.cols, d.rows)

			quadLines := renderLines(t, quadImg, func(w io.Writer, img image.Image) error {
				return Render(w, img)
			})
			halfLines := renderLines(t, halfImg, func(w io.Writer, img image.Image) error {
				return halfblock.Render(w, img)
			})

			quadW := maxVisLen(quadLines)
			nRows := max(len(quadLines), len(halfLines))

			qLabel := fmt.Sprintf("quad %dx%d px → %d cols",
				quadImg.Bounds().Dx(), quadImg.Bounds().Dy(), quadImg.Bounds().Dx()/2)
			hLabel := fmt.Sprintf("half %dx%d px → %d cols",
				halfImg.Bounds().Dx(), halfImg.Bounds().Dy(), halfImg.Bounds().Dx())

			fmt.Printf("\n── %s ──\n", path)
			fmt.Printf("%s%s%s\n", padRight(qLabel, quadW), sep, hLabel)

			for i := range nRows {
				var ql, hl string
				if i < len(quadLines) {
					ql = quadLines[i]
				}
				if i < len(halfLines) {
					hl = halfLines[i]
				}
				fmt.Printf("%s%s%s\n", padRight(ql, quadW), sep, hl)
			}
		}
	}
}
