//go:build ignore

// demo_widths renders demo assets across all three render modes and prints the
// results as a side-by-side table, one column per image. Run via:
// go run scripts/demo_widths.go
package main

import (
	"flag"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
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

type imageFlags []image

func (f *imageFlags) String() string {
	parts := make([]string, 0, len(*f))
	for _, img := range *f {
		parts = append(parts, img.path)
	}
	return strings.Join(parts, ",")
}

func (f *imageFlags) Set(value string) error {
	name, path := parseImageArg(value)
	*f = append(*f, image{name: name, path: path})
	return nil
}

func parseImageArg(value string) (name, path string) {
	if before, after, ok := strings.Cut(value, "="); ok && before != "" && after != "" {
		return before, after
	}
	return imageName(value), value
}

func imageName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" || base == "." {
		return path
	}
	return base
}

func render(bin, path, modeFlag string, w int) []string {
	out, err := exec.Command(bin, path, "-m", modeFlag, fmt.Sprintf("-w=%d", w)).Output()
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

func scaledWidths(maxWidth, steps int) []int {
	if maxWidth < 1 {
		maxWidth = 1
	}
	if steps < 1 {
		steps = 1
	}
	widths := make([]int, 0, steps)
	for w := maxWidth; len(widths) < steps; {
		widths = append(widths, w)
		next := int(math.Round(float64(w) * 0.8))
		if next >= w {
			next = w - 1
		}
		if next < 1 {
			next = 1
		}
		w = next
	}
	return widths
}

func defaultImages() []image {
	return []image{
		{"horiz", "testdata/demo_horiz_20x20/source.png"},
		{"verti", "testdata/demo_verti_20x20/source.png"},
		{"split", "testdata/demo_vert_split_8x8/source.png"},
		{"red", "testdata/solid_red_4x4.png"},
		{"diag", "testdata/demo_diag_20x20/source.png"},
		{"circ", "testdata/demo_circle_20x20/source.png"},
		{"chkr", "testdata/demo_checker_20x20/source.png"},
		{"cross", "testdata/demo_cross_20x20/source.png"},
	}
}

func printTable(widths []int, columns []string, cells [][][]string) {
	colW := make([]int, len(columns))
	for ci, name := range columns {
		colW[ci] = len(name)
		for wi := range widths {
			for _, line := range cells[ci][wi] {
				if vl := visLen(line); vl > colW[ci] {
					colW[ci] = vl
				}
			}
		}
	}

	const gap = "  " // column gap (no pipe)

	fmt.Printf("w  ")
	for ci, name := range columns {
		fmt.Printf(" %-*s", colW[ci], name)
		if ci < len(columns)-1 {
			fmt.Print(gap)
		}
	}
	fmt.Println()

	fmt.Printf("---")
	for ci := range columns {
		fmt.Printf(" %s", strings.Repeat("-", colW[ci]))
		if ci < len(columns)-1 {
			fmt.Print(gap)
		}
	}
	fmt.Println()

	for wi, w := range widths {
		if wi > 0 {
			fmt.Println() // blank line between width groups
		}
		maxLines := 1
		for ci := range columns {
			if n := len(cells[ci][wi]); n > maxLines {
				maxLines = n
			}
		}
		for li := range maxLines {
			if li == 0 {
				fmt.Printf("%-2d ", w)
			} else {
				fmt.Printf("   ")
			}
			for ci := range columns {
				var line string
				if li < len(cells[ci][wi]) {
					line = cells[ci][wi][li]
				}
				pad := colW[ci] - visLen(line)
				fmt.Printf(" %s%s", line, strings.Repeat(" ", pad))
				if ci < len(columns)-1 {
					fmt.Print(gap)
				}
			}
			fmt.Println()
		}
	}
}

func main() {
	var selected imageFlags
	maxWidth := 6
	steps := 2
	bin := "cati"
	flag.Var(&selected, "image", "image to render; repeatable; accepts path or name=path")
	flag.Var(&selected, "i", "shorthand for -image")
	flag.IntVar(&maxWidth, "w", maxWidth, "maximum render width")
	flag.IntVar(&steps, "n", steps, "number of 80% scale steps to render")
	flag.StringVar(&bin, "bin", bin, "cati executable to run")
	flag.Parse()

	images := []image(selected)
	if len(images) == 0 {
		images = defaultImages()
	}

	modes := []mode{
		{"halfblock", "halfblock"},
		{"quad/splithalf", "quad/splithalf"},
		{"spark/quad", "spark/quad"},
	}
	widths := scaledWidths(maxWidth, steps)

	if len(images) == 1 {
		img := images[0]
		fmt.Printf("\n=== Image: %s ===\n", img.name)
		columns := make([]string, len(modes))
		cells := make([][][]string, len(modes))
		for mi, m := range modes {
			columns[mi] = m.name
			cells[mi] = make([][]string, len(widths))
			for wi, w := range widths {
				cells[mi][wi] = render(bin, img.path, m.flag, w)
			}
		}
		printTable(widths, columns, cells)
		return
	}

	for _, m := range modes {
		fmt.Printf("\n=== Mode: %s ===\n", m.name)

		// Collect output: cells[imageIdx][widthIdx] = lines.
		columns := make([]string, len(images))
		cells := make([][][]string, len(images))
		for ii, img := range images {
			columns[ii] = img.name
			cells[ii] = make([][]string, len(widths))
			for wi, w := range widths {
				cells[ii][wi] = render(bin, img.path, m.flag, w)
			}
		}

		printTable(widths, columns, cells)
	}
}
