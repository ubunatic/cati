package cmd

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/cati/internal/metrics"
	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/sparkline"
)

func executeProbe(args ...string) (string, error) {
	command := modesCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs(append([]string{"test"}, args...))
	err := command.Execute()
	return out.String(), err
}

func TestModesProbeDefaultsAndSelectors(t *testing.T) {
	out, err := executeProbe()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "GLYPH OUTPUT") != 3 || !strings.Contains(out, "Mode: vec") || !strings.Contains(out, "SSIM (luma): N/A (<8x8)") {
		t.Fatal(out)
	}
	for _, pattern := range []string{"probe", "asymmetric", "diag", "vector", "grid", "multi"} {
		for _, mode := range []string{"oct", "vec", "half", "d10,50"} {
			t.Run(pattern+"/"+mode, func(t *testing.T) {
				out, err := executeProbe(pattern, "-m", mode)
				if err != nil {
					t.Fatal(err)
				}
				for _, token := range []string{"Mode: " + mode, "SOURCE PIXELS (1:1)", "RECONSTRUCTED PIXELS (1:1)", "MSE (RGBA8)", "PSNR:", "SSIM (luma):"} {
					if !strings.Contains(out, token) {
						t.Fatalf("missing %s", token)
					}
				}
				if strings.Contains(out, "\x1b[2K") || strings.Contains(out, "\r") {
					t.Fatal("line prefix in composed output")
				}
			})
		}
	}
	out, err = executeProbe("-m", "half")
	if err != nil || strings.Count(out, "Mode: half") != 3 {
		t.Fatalf("override: %s %v", out, err)
	}
}

func TestModesProbeSourcePatterns(t *testing.T) {
	for _, tc := range []struct {
		name, alias string
		w, h        int
	}{{"probe", "asymmetric", 2, 4}, {"diag", "vector", 4, 4}, {"grid", "multi", 12, 8}} {
		a, _, err := probeSource(tc.name)
		if err != nil {
			t.Fatal(err)
		}
		b, _, _ := probeSource(tc.alias)
		if a.Bounds().Dx() != tc.w || a.Bounds().Dy() != tc.h || !bytes.Equal(a.Pix, b.Pix) {
			t.Fatal(tc.name)
		}
	}
	probe, _, _ := probeSource("probe")
	if probe.RGBAAt(0, 1) != (color.RGBA{R: 255, A: 255}) || probe.RGBAAt(1, 0) != (color.RGBA{G: 50, B: 200, A: 255}) {
		t.Fatal("asymmetric probe moved")
	}
	if _, _, err := probeSource("bad"); err == nil {
		t.Fatal("unknown accepted")
	}
}

func TestModesProbeMetricsAndGlyphMasks(t *testing.T) {
	for _, name := range []string{"probe", "grid"} {
		src, _, _ := probeSource(name)
		result, err := prepareProbe(src, "oct")
		if err != nil {
			t.Fatal(err)
		}
		if result.mse != 0 || !math.IsInf(result.psnr, 1) || !bytes.Equal(src.Pix, result.reconstructed.Pix) {
			t.Fatalf("%s reconstruction MSE %g", name, result.mse)
		}
		res, _ := spec.ResolveGlyphSetExpression("oct")
		shapes := make([]sparkline.Shape, len(res.Shapes))
		byRune := map[rune][]bool{}
		for i, s := range res.Shapes {
			shapes[i] = sparkline.Shape{Ch: s.Glyph, Width: 2, Height: 4, Mask: s.Mask}
			byRune[s.Glyph] = s.Mask
		}
		grid, err := sparkline.RenderToGrid(src, result.cols, sparkline.Options{CellW: 2, CellH: 4, Rows: result.rows, Shapes: shapes, AspectX: 1})
		if err != nil {
			t.Fatal(err)
		}
		for cy, row := range grid.Cells {
			for cx, cell := range row {
				mask := byRune[cell.Ch]
				if len(mask) != 8 {
					t.Fatal(cell.Ch)
				}
				for y := 0; y < 4; y++ {
					for x := 0; x < 2; x++ {
						want := cell.Bg
						if mask[y*2+x] {
							want = cell.Fg
						}
						got := result.reconstructed.RGBAAt(cx*2+x, cy*4+y)
						if got != want {
							t.Fatalf("selected glyph mask differs at %d,%d", x, y)
						}
					}
				}
			}
		}
	}
	src, _, _ := probeSource("probe")
	// A cell with three colors cannot be reconstructed exactly by two colors.
	src.SetRGBA(1, 0, color.RGBA{G: 255, A: 255})
	imperfect, err := prepareProbe(src, "oct")
	if err != nil {
		t.Fatal(err)
	}
	if imperfect.mse <= 0 || math.IsInf(imperfect.psnr, 0) || imperfect.ssim != "N/A (<8x8)" {
		t.Fatalf("imperfect metrics %+v", imperfect)
	}
	grid, _, _ := probeSource("grid")
	r, err := prepareProbe(grid, "oct")
	if err != nil || r.ssim != "1.000000" {
		t.Fatalf("grid SSIM %s %v", r.ssim, err)
	}
}

func TestModesProbePaddingBoundsAndAlpha(t *testing.T) {
	src := image.NewNRGBA(image.Rect(4, 7, 7, 12))
	src.SetNRGBA(4, 7, color.NRGBA{R: 200, G: 100, A: 128})
	src.SetNRGBA(6, 11, color.NRGBA{B: 255, A: 255})
	result, err := prepareProbe(src, "oct")
	if err != nil {
		t.Fatal(err)
	}
	if result.cols != 2 || result.rows != 2 || result.padW != 1 || result.padH != 3 || result.source.Bounds() != image.Rect(0, 0, 3, 5) {
		t.Fatalf("geometry %+v", result)
	}
	for y := 0; y < 5; y++ {
		for x := 0; x < 3; x++ {
			ar, ag, ab, aa := src.At(x+4, y+7).RGBA()
			br, bg, bb, ba := result.source.At(x, y).RGBA()
			if ar>>8 != br>>8 || ag>>8 != bg>>8 || ab>>8 != bb>>8 || aa>>8 != ba>>8 {
				t.Fatalf("source pixel %d,%d changed", x, y)
			}
		}
	}
	if result.mse != metrics.MSE(result.source, result.reconstructed) {
		t.Fatal("metrics include padding")
	}
	empty := image.NewRGBA(image.Rectangle{})
	if _, err := prepareProbe(empty, "oct"); err == nil {
		t.Fatal("empty accepted")
	}
	transparent := image.NewRGBA(image.Rect(0, 0, 3, 5))
	r, err := prepareProbe(transparent, "oct")
	if err != nil || r.mse != 0 {
		t.Fatalf("transparent MSE %g %v", r.mse, err)
	}
	var out bytes.Buffer
	if err := writeProbe(&out, "transparent", transparent, "oct"); err != nil {
		t.Fatal(err)
	}
	// Three display rows must survive even when their image pixels are transparent.
	section := strings.Split(out.String(), "RECONSTRUCTED PIXELS (1:1)\n")[1]
	if strings.Count(section, "  |  ") != 3 {
		t.Fatalf("blank rows lost: %q", section)
	}
}

func TestModesProbeImagesErrorsAndHelp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.png")
	src, _, _ := probeSource("probe")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, src); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	a, err := executeProbe(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := executeProbe("-i", path)
	if err != nil || a != b {
		t.Fatalf("image forms differ %v", err)
	}
	for _, args := range [][]string{{"probe", "grid"}, {"-i", path, "probe"}, {"-i", ""}, {"-m", "bad"}, {"-m", ""}, {"missing-file.png"}, {"probe", "-m", "d999"}, {"--list-patterns", "probe"}, {"--width", "10"}} {
		if _, err := executeProbe(args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, flag := range []string{"--help", "--list-patterns"} {
		out, err := executeProbe(flag)
		if err != nil || !strings.Contains(out, "probe") {
			t.Fatalf("%s %v", out, err)
		}
	}
	if err := writeProbe(glyphFailWriter{}, "probe", src, "oct"); !errors.Is(err, errGlyphWrite) {
		t.Fatal(err)
	}
}

func TestModesProbeANSIAlignment(t *testing.T) {
	out, err := executeProbe("grid")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(out, "SOURCE PIXELS (1:1)")
	for _, line := range strings.Split(out[start:], "\n") {
		if line == "" {
			continue
		}
		index := strings.Index(line, "  |  ")
		if index < 0 {
			t.Fatal(line)
		}
		if ansiLineWidth(line[:index]) != len("SOURCE PIXELS (1:1)") {
			t.Fatalf("misaligned: %q", line)
		}
	}
}

func TestModesProbeGolden(t *testing.T) {
	for _, name := range []string{"probe", "diag", "grid"} {
		src, mode, _ := probeSource(name)
		result, err := prepareProbe(src, mode)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile("testdata/reconstruction_probes/" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range []string{"Algorithm\x00glyph-probe", "Mode\x00" + mode, "Pattern\x00" + name} {
			if !bytes.Contains(data, []byte(token)) {
				t.Fatal("missing metadata", token)
			}
		}
		expected, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if expected.Bounds() != result.reconstructed.Bounds() {
			t.Fatal("golden bounds")
		}
		if metrics.MSE(expected, result.reconstructed) != 0 {
			t.Fatalf("%s golden pixels differ (MSE %g)", name, metrics.MSE(expected, result.reconstructed))
		}
	}
}
