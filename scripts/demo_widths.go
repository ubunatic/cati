//go:build ignore

// demo_widths renders the main testdata assets at widths 1..6 across all
// three render modes and prints the results as a side-by-side table, one
// column per demo image.  Run via: go run scripts/demo_widths.go
package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var (
	// controlSeq strips structural terminal sequences (erase-line, CR) while
	// preserving colour escape codes so the rendered glyphs stay coloured.
	controlSeq = regexp.MustCompile(`\x1b\[2K|\r`)
	// ansiSeq strips all ANSI escape sequences for visual-width measurement.
	ansiSeq = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
)

func stripControl(s string) string { return controlSeq.ReplaceAllString(s, "") }
func visLen(s string) int          { return len([]rune(ansiSeq.ReplaceAllString(s, ""))) }

type image struct{ name, path string }
type mode struct{ name, flag string }

func render(path, modeFlag string, w int) []string {
	out, err := exec.Command("cati", path, "-m", modeFlag, fmt.Sprintf("-w=%d", w)).Output()
	if err != nil {
		return []string{"(err)"}
	}
	raw := strings.TrimRight(string(out), "\n")
	parts := strings.Split(raw, "\n")
	lines := make([]string, 0, len(parts))
	for _, p := range parts {
		p = stripControl(p)
		if p != "" {
			lines = append(lines, p)
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func main() {
	images := []image{
		{"horiz", "testdata/demo_horiz_20x20/source.png"},
		{"verti", "testdata/demo_verti_20x20/source.png"},
		{"split", "testdata/demo_vert_split_8x8/source.png"},
		{"red", "testdata/solid_red_4x4.png"},
		{"diag", "testdata/demo_diag_20x20/source.png"},
		{"circ", "testdata/demo_circle_20x20/source.png"},
		{"chkr", "testdata/demo_checker_20x20/source.png"},
		{"cross", "testdata/demo_cross_20x20/source.png"},
	}
	modes := []mode{
		{"halfblock", "halfblock"},
		{"quad/splithalf", "quad/splithalf"},
		{"spark/quad", "spark/quad"},
	}
	widths := []int{1, 2, 3, 4, 5, 6}

	for _, m := range modes {
		fmt.Printf("\n=== Mode: %s ===\n", m.name)

		// Collect output: cells[imageIdx][widthIdx] = lines
		cells := make([][][]string, len(images))
		for ii, img := range images {
			cells[ii] = make([][]string, len(widths))
			for wi, w := range widths {
				cells[ii][wi] = render(img.path, m.flag, w)
			}
		}

		// Column widths: max visual line length in the column, at least header width.
		colW := make([]int, len(images))
		for ii, img := range images {
			colW[ii] = len(img.name)
			for wi := range widths {
				for _, line := range cells[ii][wi] {
					if vl := visLen(line); vl > colW[ii] {
						colW[ii] = vl
					}
				}
			}
		}

		const gap = "  " // column gap (no pipe)

		// Header row.
		fmt.Printf("w  ")
		for ii, img := range images {
			fmt.Printf(" %-*s", colW[ii], img.name)
			if ii < len(images)-1 {
				fmt.Print(gap)
			}
		}
		fmt.Println()

		// Separator.
		fmt.Printf("---")
		for ii := range images {
			fmt.Printf(" %s", strings.Repeat("-", colW[ii]))
			if ii < len(images)-1 {
				fmt.Print(gap)
			}
		}
		fmt.Println()

		// Data rows.
		for wi, w := range widths {
			if wi > 0 {
				fmt.Println() // blank line between width groups
			}
			maxLines := 1
			for ii := range images {
				if n := len(cells[ii][wi]); n > maxLines {
					maxLines = n
				}
			}
			for li := range maxLines {
				if li == 0 {
					fmt.Printf("%-2d ", w)
				} else {
					fmt.Printf("   ")
				}
				for ii := range images {
					var line string
					if li < len(cells[ii][wi]) {
						line = cells[ii][wi][li]
					}
					pad := colW[ii] - visLen(line)
					fmt.Printf(" %s%s", line, strings.Repeat(" ", pad))
					if ii < len(images)-1 {
						fmt.Print(gap)
					}
				}
				fmt.Println()
			}
		}
	}
}
