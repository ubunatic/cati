//go:build ignore

// demo_widths renders demo assets across all three render modes and prints the
// results as a side-by-side table, one column per image. Run via:
//   go run scripts/demo_widths.go
//
// Video mode: compare frames from one or more video files at specific timestamps.
//   go run scripts/demo_widths.go -video assets/baby-360p.mp4 -at 1s,5s,10s -w 20
package main

import (
	"flag"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
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

// renderAt renders one frame of a video at offsetSec using --range Xs:.
func renderAt(bin, path, modeFlag string, offsetSec float64, w int) []string {
	rangeArg := fmt.Sprintf("%.3fs:", offsetSec)
	out, err := exec.Command(bin, path, "-m", modeFlag,
		fmt.Sprintf("-w=%d", w), "--range", rangeArg).Output()
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

// ── video flags ───────────────────────────────────────────────────────────────

type videoFlags []image // reuse image struct: name=label, path=file

func (f *videoFlags) String() string {
	parts := make([]string, 0, len(*f))
	for _, v := range *f {
		parts = append(parts, v.path)
	}
	return strings.Join(parts, ",")
}

func (f *videoFlags) Set(value string) error {
	name, path := parseImageArg(value)
	*f = append(*f, image{name: name, path: path})
	return nil
}

// parseAtList parses a comma-separated list of time tokens into seconds.
// Each token is parsed the same way as parseTimeSec in the main cati package:
// bare float ("5"), Go duration ("5s", "1m30s"), or clock ("1:30").
func parseAtList(s string) ([]float64, error) {
	if s == "" {
		return nil, nil
	}
	tokens := strings.Split(s, ",")
	out := make([]float64, 0, len(tokens))
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		sec, err := parseTimeSec(tok)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp %q: %w", tok, err)
		}
		out = append(out, sec)
	}
	return out, nil
}

// parseTimeSec mirrors cmd.parseTimeSec: bare float, Go duration, or mm:ss / hh:mm:ss.
func parseTimeSec(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d.Seconds(), nil
	}
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		switch len(parts) {
		case 2:
			m, e1 := strconv.ParseFloat(parts[0], 64)
			sec, e2 := strconv.ParseFloat(parts[1], 64)
			if e1 != nil || e2 != nil {
				return 0, fmt.Errorf("invalid time %q", s)
			}
			return m*60 + sec, nil
		case 3:
			h, e1 := strconv.ParseFloat(parts[0], 64)
			m, e2 := strconv.ParseFloat(parts[1], 64)
			sec, e3 := strconv.ParseFloat(parts[2], 64)
			if e1 != nil || e2 != nil || e3 != nil {
				return 0, fmt.Errorf("invalid time %q", s)
			}
			return h*3600 + m*60 + sec, nil
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid time %q (use e.g. 5s, 1m30s, 1:30)", s)
	}
	return f, nil
}

// probeDuration runs ffprobe to get the duration of a video in seconds.
func probeDuration(path string) float64 {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		path).Output()
	if err != nil {
		return 0
	}
	f, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return f
}

// autoTimestamps returns a useful default set of timestamps for a video:
// 1 s, then 25 / 50 / 75 % of duration (skipping values < 1 s).
func autoTimestamps(path string) []float64 {
	dur := probeDuration(path)
	if dur <= 0 {
		return []float64{1}
	}
	candidates := []float64{1, dur * 0.25, dur * 0.5, dur * 0.75}
	out := make([]float64, 0, len(candidates))
	for _, t := range candidates {
		if t >= 0.5 && t < dur {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		out = []float64{0}
	}
	return out
}

// fmtSec formats seconds as a short human-readable label.
func fmtSec(sec float64) string {
	if sec < 60 {
		return fmt.Sprintf("%.1fs", sec)
	}
	m := int(sec) / 60
	s := sec - float64(m*60)
	return fmt.Sprintf("%dm%04.1fs", m, s)
}

// runVideoMode renders video frames from one or more video files at the given
// timestamps and prints the results as a side-by-side table.
//
// Single-frame case (exactly one video × one timestamp): flips the table so
// columns = render modes and rows = width steps, matching the single-image layout.
func runVideoMode(bin string, videos []image, timestamps []float64, widths []int) {
	modes := []mode{
		{"halfblock", "halfblock"},
		{"quad/splithalf", "quad/splithalf"},
		{"spark/quad", "spark/quad"},
	}

	// Build columns: one per (video × timestamp) combination.
	type col struct {
		video image
		at    float64
		label string
	}
	var cols []col
	for _, v := range videos {
		ats := timestamps
		if len(ats) == 0 {
			ats = autoTimestamps(v.path)
		}
		for _, t := range ats {
			label := v.name + "@" + fmtSec(t)
			cols = append(cols, col{video: v, at: t, label: label})
		}
	}

	// ── Single-frame: columns = modes ────────────────────────────────────────
	if len(cols) == 1 {
		c := cols[0]
		fmt.Printf("\n=== Frame: %s ===\n", c.label)
		colNames := make([]string, len(modes))
		cells := make([][][]string, len(modes))
		for mi, m := range modes {
			colNames[mi] = m.name
			cells[mi] = make([][]string, len(widths))
			for wi, w := range widths {
				cells[mi][wi] = renderAt(bin, c.video.path, m.flag, c.at, w)
			}
		}
		printTable(widths, colNames, cells)
		return
	}

	// ── Multi-frame: one section per mode, columns = frames ──────────────────
	for _, m := range modes {
		fmt.Printf("\n=== Mode: %s ===\n", m.name)
		colNames := make([]string, len(cols))
		cells := make([][][]string, len(cols))
		for ci, c := range cols {
			colNames[ci] = c.label
			cells[ci] = make([][]string, len(widths))
			for wi, w := range widths {
				cells[ci][wi] = renderAt(bin, c.video.path, m.flag, c.at, w)
			}
		}
		printTable(widths, colNames, cells)
	}
}

func main() {
	var selected imageFlags
	var videos videoFlags
	var atStr string
	maxWidth := 6
	steps := 2
	bin := "cati"
	flag.Var(&selected, "image", "image to render; repeatable; accepts path or name=path")
	flag.Var(&selected, "i", "shorthand for -image")
	flag.Var(&videos, "video", "video file to sample frames from; repeatable; accepts path or name=path")
	flag.Var(&videos, "v", "shorthand for -video")
	flag.StringVar(&atStr, "at", "", "comma-separated timestamps for video mode (e.g. 1s,5s,10s,1:30); default: auto")
	flag.IntVar(&maxWidth, "w", maxWidth, "maximum render width")
	flag.IntVar(&steps, "n", steps, "number of 80% scale steps to render")
	flag.StringVar(&bin, "bin", bin, "cati executable to run")
	flag.Parse()

	widths := scaledWidths(maxWidth, steps)

	// ── Video mode ────────────────────────────────────────────────────────────
	if len(videos) > 0 {
		timestamps, err := parseAtList(atStr)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		runVideoMode(bin, []image(videos), timestamps, widths)
		return
	}

	// ── Image mode (existing behaviour) ───────────────────────────────────────
	images := []image(selected)
	if len(images) == 0 {
		images = defaultImages()
	}

	modes := []mode{
		{"halfblock", "halfblock"},
		{"quad/splithalf", "quad/splithalf"},
		{"spark/quad", "spark/quad"},
	}

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
