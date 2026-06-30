package cmd

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/quadblock"
	"codeberg.org/ubunatic/cati/internal/sextant"
	"codeberg.org/ubunatic/cati/internal/sparkline"
	"codeberg.org/ubunatic/cati/internal/sparkline/testhelper"
)

// ── isImageFile ───────────────────────────────────────────────────────────────

func TestIsImageFile(t *testing.T) {
	t.Helper()
	cases := []struct {
		path string
		want bool
	}{
		{"photo.png", true},
		{"photo.PNG", true},
		{"photo.jpg", true},
		{"photo.JPEG", true},
		{"photo.jpeg", true},
		{"readme.txt", false},
		{"noext", false},
		{"archive.tar.gz", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isImageFile(tc.path); got != tc.want {
				t.Errorf("isImageFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// ── expandArgs ────────────────────────────────────────────────────────────────

func TestExpandArgs_Files(t *testing.T) {
	// Explicit file paths are returned as-is (stat must succeed → use real files).
	files := []string{
		"testdata/solid_red_4x4.png",
		"testdata/checkerboard_4x4.png",
	}
	got, err := expandArgs(files, false)
	if err != nil {
		t.Fatalf("expandArgs: %v", err)
	}
	if len(got) != len(files) {
		t.Errorf("got %d paths, want %d: %v", len(got), len(files), got)
	}
}

func TestExpandArgs_Directory_Flat(t *testing.T) {
	got, err := expandArgs([]string{"testdata"}, false)
	if err != nil {
		t.Fatalf("expandArgs: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one image from testdata/")
	}
	for _, p := range got {
		if !isImageFile(p) {
			t.Errorf("non-image file in result: %s", p)
		}
	}
}

func TestExpandArgs_Directory_Recursive(t *testing.T) {
	// Create a temp tree: root/sub/deep.png
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// Copy a real PNG into both levels.
	copyFile := func(src, dst string) {
		t.Helper()
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", dst, err)
		}
	}
	copyFile("testdata/solid_red_4x4.png", filepath.Join(root, "top.png"))
	copyFile("testdata/solid_red_4x4.png", filepath.Join(sub, "deep.png"))

	// Flat: should only return root-level file.
	flat, err := expandArgs([]string{root}, false)
	if err != nil {
		t.Fatalf("flat expandArgs: %v", err)
	}
	if len(flat) != 1 {
		t.Errorf("flat: expected 1 file, got %d: %v", len(flat), flat)
	}

	// Recursive: should return both.
	rec, err := expandArgs([]string{root}, true)
	if err != nil {
		t.Fatalf("recursive expandArgs: %v", err)
	}
	if len(rec) != 2 {
		t.Errorf("recursive: expected 2 files, got %d: %v", len(rec), rec)
	}
}

func TestExpandArgs_Deduplication(t *testing.T) {
	path := "testdata/solid_red_4x4.png"
	got, err := expandArgs([]string{path, path}, false)
	if err != nil {
		t.Fatalf("expandArgs: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected deduplication to 1 entry, got %d: %v", len(got), got)
	}
}

func TestExpandArgs_MissingFile(t *testing.T) {
	_, err := expandArgs([]string{"nonexistent.png"}, false)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestZoomFlagShorthand(t *testing.T) {
	cmd := New()
	// Test --zoom
	err := cmd.ParseFlags([]string{"--zoom", "w"})
	if err != nil {
		t.Fatalf("failed to parse --zoom: %v", err)
	}
	zoomVal, err := cmd.Flags().GetString("zoom")
	if err != nil {
		t.Fatalf("failed to get zoom flag: %v", err)
	}
	if zoomVal != "w" {
		t.Errorf("expected zoom flag to be 'w', got '%s'", zoomVal)
	}

	// Reset flags and test -z
	cmd = New()
	err = cmd.ParseFlags([]string{"-z", "h"})
	if err != nil {
		t.Fatalf("failed to parse -z: %v", err)
	}
	zoomVal, err = cmd.Flags().GetString("zoom")
	if err != nil {
		t.Fatalf("failed to get zoom flag: %v", err)
	}
	if zoomVal != "h" {
		t.Errorf("expected zoom flag to be 'h', got '%s'", zoomVal)
	}
}

func TestParseRenderMode(t *testing.T) {
	tests := []struct {
		name  string
		mode  string
		want  string
		check func(t *testing.T, opts quadblock.Options)
	}{
		{"default empty is halfblock", "", "halfblock", nil},
		{"halfblock short", "h", "halfblock", nil},
		{"halfblock alias", "half", "halfblock", nil},
		{"quad split-half", "qs", "quad/splithalf", func(t *testing.T, opts quadblock.Options) {
			if !opts.SplitHalf {
				t.Fatal("qs mode should enable SplitHalf")
			}
		}},
		{"quad alias", "quad", "quad/splithalf", func(t *testing.T, opts quadblock.Options) {
			if !opts.SplitHalf {
				t.Fatal("quad alias should enable SplitHalf")
			}
		}},
		{"quad edge snap", "qe", "quad/edge-snap", func(t *testing.T, opts quadblock.Options) {
			if !opts.EdgeSnap {
				t.Fatal("qe mode should enable EdgeSnap")
			}
		}},
		{"spark quad", "sq", "spark/quad", func(t *testing.T, opts quadblock.Options) {
			if opts != (quadblock.Options{}) {
				t.Fatalf("sq mode should not set quad options, got %#v", opts)
			}
		}},
		{"spark alias", "spark", "spark/quad", func(t *testing.T, opts quadblock.Options) {
			if opts != (quadblock.Options{}) {
				t.Fatalf("spark alias should not set quad options, got %#v", opts)
			}
		}},
		{"spark geom", "sg", "spark/geom", func(t *testing.T, opts quadblock.Options) {
			if opts != (quadblock.Options{}) {
				t.Fatalf("sg mode should not set quad options, got %#v", opts)
			}
		}},
		{"spark best", "sb", "spark/best", func(t *testing.T, opts quadblock.Options) {
			if opts != (quadblock.Options{}) {
				t.Fatalf("sb mode should not set quad options, got %#v", opts)
			}
		}},
		{"sextant 2x3", "xs", "sextant/2x3", func(t *testing.T, opts quadblock.Options) {
			if opts != (quadblock.Options{}) {
				t.Fatalf("xs mode should not set quad options, got %#v", opts)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rc, err := parseRenderMode(tc.mode)
			if err != nil {
				t.Fatalf("parseRenderMode(%q): %v", tc.mode, err)
			}
			if got := rcModeName(rc); got != tc.want {
				t.Fatalf("rcModeName(parseRenderMode(%q)) = %q, want %q", tc.mode, got, tc.want)
			}
			if tc.check != nil {
				tc.check(t, rc.quadOpts)
			}
		})
	}
}

func TestRemovedRenderModesAreRejected(t *testing.T) {
	for _, mode := range []string{"xg", "xb", "geom", "best", "sextant/geom", "sextant/best", "sh", "shg", "shb", "geomshape", "geomshape/2x2", "geomshape/geom", "geomshape/best"} {
		t.Run(mode, func(t *testing.T) {
			if _, err := parseRenderMode(mode); err == nil {
				t.Fatalf("parseRenderMode(%q) succeeded, want error", mode)
			}
		})
	}
}

func TestModeFlagShorthand(t *testing.T) {
	cmd := New()
	if err := cmd.ParseFlags([]string{"-m", "qe"}); err != nil {
		t.Fatalf("failed to parse -m: %v", err)
	}
	got, err := cmd.Flags().GetString("mode")
	if err != nil {
		t.Fatalf("failed to get mode flag: %v", err)
	}
	if got != "qe" {
		t.Fatalf("expected mode flag to be 'qe', got %q", got)
	}

	cmd = New()
	if err := cmd.ParseFlags([]string{"--mode", "spark"}); err != nil {
		t.Fatalf("failed to parse --mode: %v", err)
	}
	got, err = cmd.Flags().GetString("mode")
	if err != nil {
		t.Fatalf("failed to get mode flag: %v", err)
	}
	if got != "spark" {
		t.Fatalf("expected mode flag to be 'spark', got %q", got)
	}

	cmd = New()
	if err := cmd.ParseFlags([]string{"--mode", "sextant"}); err != nil {
		t.Fatalf("failed to parse --mode: %v", err)
	}
	got, err = cmd.Flags().GetString("mode")
	if err != nil {
		t.Fatalf("failed to get mode flag: %v", err)
	}
	if got != "sextant" {
		t.Fatalf("expected mode flag to be 'sextant', got %q", got)
	}

	cmd = New()
	if err := cmd.ParseFlags([]string{"-S", "pyramid"}); err != nil {
		t.Fatalf("failed to parse -S: %v", err)
	}
	got, err = cmd.Flags().GetString("prescaler")
	if err != nil {
		t.Fatalf("failed to get prescaler flag: %v", err)
	}
	if got != "pyramid" {
		t.Fatalf("expected prescaler flag to be 'pyramid', got %q", got)
	}
}

func TestJobsFlagShorthand(t *testing.T) {
	cmd := New()
	if err := cmd.ParseFlags([]string{"-j", "7"}); err != nil {
		t.Fatalf("failed to parse -j: %v", err)
	}
	got, err := cmd.Flags().GetInt("jobs")
	if err != nil {
		t.Fatalf("failed to get jobs flag: %v", err)
	}
	if got != 7 {
		t.Fatalf("expected jobs flag to be 7, got %d", got)
	}

	cmd = New()
	if err := cmd.ParseFlags([]string{"--jobs", "11"}); err != nil {
		t.Fatalf("failed to parse --jobs: %v", err)
	}
	got, err = cmd.Flags().GetInt("jobs")
	if err != nil {
		t.Fatalf("failed to get jobs flag: %v", err)
	}
	if got != 11 {
		t.Fatalf("expected jobs flag to be 11, got %d", got)
	}
}

func TestResolveWorkerCount(t *testing.T) {
	if got := resolveWorkerCount(9, 4); got != 9 {
		t.Fatalf("resolveWorkerCount(9, 4) = %d, want 9", got)
	}
	if got := resolveWorkerCount(0, 6); got != 6 {
		t.Fatalf("resolveWorkerCount(0, 6) = %d, want 6", got)
	}
	if got := resolveWorkerCount(0, 0); got < 1 {
		t.Fatalf("resolveWorkerCount(0, 0) = %d, want >= 1", got)
	}
}

func TestParsePrescaleMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want prescaleMode
	}{
		{"default", "", prescaleNearestNeighbor},
		{"nn", "nn", prescaleNearestNeighbor},
		{"nearest", "nearest-neighbor", prescaleNearestNeighbor},
		{"pyramid", "pyramid", prescalePyramid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePrescaleMode(tc.mode)
			if err != nil {
				t.Fatalf("parsePrescaleMode(%q): %v", tc.mode, err)
			}
			if got != tc.want {
				t.Fatalf("parsePrescaleMode(%q) = %v, want %v", tc.mode, got, tc.want)
			}
		})
	}
}

func TestAlignScaledSize(t *testing.T) {
	tests := []struct {
		name  string
		rc    renderCfg
		w     int
		h     int
		wantW int
		wantH int
	}{
		{"halfblock keeps width, trims height", renderCfg{}, 11, 13, 11, 12},
		{"quad trims both axes", renderCfg{mode: modeQuad}, 11, 13, 10, 12},
		{"spark trims to cell quantum", renderCfg{mode: modeSpark}, 11, 13, 8, 8},
		{"sextant keeps odd height", renderCfg{mode: modeSextant, sextantMode: 0}, 11, 13, 10, 13},
		{"small quad image stays visible", renderCfg{mode: modeQuad}, 1, 1, 1, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotW, gotH := alignRenderedCellSize(tc.w, tc.h, tc.rc)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Fatalf("alignRenderedCellSize(%d,%d,%#v) = %dx%d, want %dx%d", tc.w, tc.h, tc.rc, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestResizeRenderedImageDownscaleNNCentered(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 1))
	cols := []color.RGBA{
		{R: 10, A: 255},
		{R: 20, A: 255},
		{R: 30, A: 255},
		{R: 40, A: 255},
	}
	for x, c := range cols {
		src.SetRGBA(x, 0, c)
	}

	got := resizeRenderedImage(src, 2, 1, renderCfg{})
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 1 {
		t.Fatalf("resizeRenderedImage size = %dx%d, want 2x1", got.Bounds().Dx(), got.Bounds().Dy())
	}

	left := got.At(0, 0).(color.RGBA)
	right := got.At(1, 0).(color.RGBA)
	if left != cols[0] {
		t.Fatalf("left pixel = %#v, want %#v", left, cols[0])
	}
	if right != cols[2] {
		t.Fatalf("right pixel = %#v, want %#v", right, cols[2])
	}
}

func TestResizeRenderedImagePyramid(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 1))
	cols := []color.RGBA{
		{R: 10, A: 255},
		{R: 20, A: 255},
		{R: 30, A: 255},
		{R: 40, A: 255},
	}
	for x, c := range cols {
		src.SetRGBA(x, 0, c)
	}

	got := resizeRenderedImage(src, 2, 1, renderCfg{prescaler: prescalePyramid})
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 1 {
		t.Fatalf("resizeRenderedImage size = %dx%d, want 2x1", got.Bounds().Dx(), got.Bounds().Dy())
	}

	left := got.At(0, 0).(color.RGBA)
	right := got.At(1, 0).(color.RGBA)
	if left == cols[0] && right == cols[2] {
		t.Fatalf("pyramid mode should not behave like centered NN")
	}
}

func TestSampleDarthDaughterNoDuplicateRenderedPixelRows(t *testing.T) {
	src, err := halfblock.LoadImage("assets/samples/sample-003-darth-daughter.jpg")
	if err != nil {
		t.Fatalf("load sample: %v", err)
	}

	modes := []string{"h", "qs", "qe", "sq"}
	widths := []int{20, 30, 40, 50}
	for _, mode := range modes {
		rc, err := parseRenderMode(mode)
		if err != nil {
			t.Fatalf("parseRenderMode(%q): %v", mode, err)
		}
		for _, width := range widths {
			t.Run(mode+"-w"+itoa(width), func(t *testing.T) {
				vp := staticViewportForTest(src, width, rc)
				if vp.Bounds().Dx() <= 0 || vp.Bounds().Dy() <= 0 {
					t.Fatalf("empty viewport for mode=%s width=%d", mode, width)
				}
				rendered := rowDuplicateSignalImageForTest(vp, rc)
				if row := firstDuplicatePixelRow(rendered); row >= 0 {
					vb := vp.Bounds()
					rb := rendered.Bounds()
					vpDup := row+1 < vb.Dy() && equalPixelRows(vp, vb.Min.Y+row, vb.Min.Y+row+1)
					t.Fatalf("mode=%s width=%d duplicated rendered pixel rows %d and %d viewport=%dx%d rendered=%dx%d viewport-dup=%v",
						mode, width, row, row+1, vb.Dx(), vb.Dy(), rb.Dx(), rb.Dy(), vpDup)
				}
			})
		}
	}
}

func TestAllRenderModesWidthOneThroughTwentyKeepAspectAndNoGaps(t *testing.T) {
	if err := testhelper.GenerateGradients("testdata"); err != nil {
		t.Fatalf("GenerateGradients: %v", err)
	}
	if err := testhelper.GenerateFixtures("testdata"); err != nil {
		t.Fatalf("GenerateFixtures: %v", err)
	}
	if err := testhelper.GenerateGeometrics("testdata"); err != nil {
		t.Fatalf("GenerateGeometrics: %v", err)
	}

	sources := []struct {
		name string
		img  image.Image
	}{
		{"generated_gradient_32x32", opaqueGradientForTest(32, 32)},
	}
	for _, tc := range []struct {
		name string
		path string
	}{
		{"gradient_32x32", "testdata/gradient_32x32.png"},
		{"solid_red_4x4", "testdata/solid_red_4x4/source.png"},
		{"horiz_gradient_20x20", "testdata/demo_horiz_20x20/source.png"},
		{"vert_gradient_20x20", "testdata/demo_verti_20x20/source.png"},
		{"vert_split_8x8", "testdata/demo_vert_split_8x8/source.png"},
		{"diag_20x20", "testdata/demo_diag_20x20/source.png"},
		{"circle_20x20", "testdata/demo_circle_20x20/source.png"},
		{"checker_20x20", "testdata/demo_checker_20x20/source.png"},
		{"cross_20x20", "testdata/demo_cross_20x20/source.png"},
	} {
		src := goldenLoad(t, tc.path)
		if src == nil {
			continue
		}
		sources = append(sources, struct {
			name string
			img  image.Image
		}{tc.name, src})
	}

	modes := []string{"h", "qs", "qe", "sq", "sg", "sb", "xs"}

	for _, source := range sources {
		for _, mode := range modes {
			rc, err := parseRenderMode(mode)
			if err != nil {
				t.Fatalf("parseRenderMode(%q): %v", mode, err)
			}
			for width := 1; width <= 20; width++ {
				t.Run(source.name+"/"+mode+"-w"+itoa(width), func(t *testing.T) {
					vp, err := prepareRenderedImageChecked(source.img, nil, width, 0, rc, "")
					if err != nil {
						t.Fatalf("prepareRenderedImageChecked: %v", err)
					}
					vb := vp.Bounds()
					if vb.Dx() <= 0 || vb.Dy() <= 0 {
						t.Fatalf("empty viewport: %dx%d", vb.Dx(), vb.Dy())
					}

					contentH := trimTransparentTailHeight(vp)
					if contentH <= 0 {
						t.Fatalf("viewport has no opaque content: %dx%d", vb.Dx(), vb.Dy())
					}
					if err := validateStaticViewportAspectForTest(source.img.Bounds(), vb.Dx(), contentH, rc); err != nil {
						t.Fatalf("viewport aspect: %v", err)
					}

					var ansi strings.Builder
					if err := rc.render(&ansi, vp); err != nil {
						t.Fatalf("render ANSI: %v", err)
					}
					if gap := firstDefaultBackgroundCellForOpaqueSource(ansi.String(), rc); gap.found {
						t.Fatalf("terminal-default gap at row=%d col=%d glyph=%q viewport=%dx%d",
							gap.row, gap.col, gap.ch, vb.Dx(), vb.Dy())
					}

					rendered := renderNativeImageForTest(vp, rc)
					if gap := firstTransparentGap(rendered); gap >= 0 {
						rb := rendered.Bounds()
						t.Fatalf("transparent gap at rendered pixel %d,%d for viewport=%dx%d rendered=%dx%d",
							gap%rb.Dx(), gap/rb.Dx(), vb.Dx(), vb.Dy(), rb.Dx(), rb.Dy())
					}
				})
			}
		}
	}
}

type ansiGap struct {
	found bool
	row   int
	col   int
	ch    rune
}

func firstDefaultBackgroundCellForOpaqueSource(out string, rc renderCfg) ansiGap {
	if !rc.mode.useSextant() {
		return ansiGap{}
	}
	row, col := 0, 0
	bgActive := false
	for i := 0; i < len(out); {
		r := rune(out[i])
		size := 1
		if r >= 0x80 {
			r, size = utf8.DecodeRuneInString(out[i:])
		}
		if r == '\x1b' {
			next, bg := scanANSIForBackground(out, i, bgActive)
			i = next
			bgActive = bg
			continue
		}
		switch r {
		case '\r':
			col = 0
		case '\n':
			row++
			col = 0
		default:
			if !bgActive {
				return ansiGap{found: true, row: row, col: col, ch: r}
			}
			col++
		}
		i += size
	}
	return ansiGap{}
}

func scanANSIForBackground(out string, i int, bgActive bool) (int, bool) {
	if i+1 >= len(out) || out[i+1] != '[' {
		return i + 1, bgActive
	}
	j := i + 2
	for j < len(out) && (out[j] < '@' || out[j] > '~') {
		j++
	}
	if j >= len(out) {
		return len(out), bgActive
	}
	if out[j] != 'm' {
		return j + 1, bgActive
	}
	params := splitANSIParams(out[i+2 : j])
	if len(params) == 0 || params[0] == "" {
		return j + 1, false
	}
	for k := 0; k < len(params); k++ {
		switch params[k] {
		case "0":
			bgActive = false
		case "38":
			if k+4 < len(params) && params[k+1] == "2" {
				k += 4
			}
		case "48":
			bgActive = true
			if k+4 < len(params) && params[k+1] == "2" {
				k += 4
			}
		}
	}
	return j + 1, bgActive
}

func splitANSIParams(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ';' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}

func staticViewportForTest(src image.Image, width int, rc renderCfg) image.Image {
	return prepareRenderedImage(src, nil, width, 0, rc, "")
}

func opaqueGradientForTest(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / max(1, w-1)),
				G: uint8(y * 255 / max(1, h-1)),
				B: uint8((x + y) * 255 / max(1, w+h-2)),
				A: 255,
			})
		}
	}
	return img
}

func validateStaticViewportAspectForTest(src image.Rectangle, renderW, renderH int, rc renderCfg) error {
	if spec, ok := rc.mode.v2FitSpec(); ok {
		return validateSourceAspectWith(rc, src, renderW, renderH, spec.AspectNum, spec.AspectDen, spec.CellW, spec.CellH)
	}
	return validateSourceAspect(rc, src, renderW, renderH)
}

func trimTransparentTailHeight(img image.Image) int {
	b := img.Bounds()
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		if !transparentPixelRow(img, y) {
			return y - b.Min.Y + 1
		}
	}
	return 0
}

func renderNativeImageForTest(vp image.Image, rc renderCfg) image.Image {
	b := vp.Bounds()
	switch rc.mode {
	case modeSextant:
		return sextant.RenderToImage(vp, rc.sextantMode)
	case modeSpark, modeSparkGeom, modeSparkBest:
		outCols := max(1, b.Dx()/4)
		outRows := max(1, b.Dy()/8)
		return sparkline.RenderToImage(vp, outCols, outRows, rc.sparkMode)
	case modeQuad:
		return quadblock.RenderToImage(vp, rc.quadOpts)
	default:
		return halfblock.RenderToImage(vp)
	}
}

func firstTransparentGap(img image.Image) int {
	b := img.Bounds()
	tailStart := b.Max.Y
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		if !transparentPixelRow(img, y) {
			break
		}
		tailStart = y
	}
	for y := b.Min.Y; y < tailStart; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				return (y-b.Min.Y)*b.Dx() + (x - b.Min.X)
			}
		}
	}
	return -1
}

func rowDuplicateSignalImageForTest(vp image.Image, rc renderCfg) image.Image {
	switch {
	case rc.mode.useQuad():
		return quadblock.RenderToImage(vp, rc.quadOpts)
	case rc.mode.useSpark():
		return vp
	default:
		return halfblock.RenderToImage(vp)
	}
}

func firstDuplicatePixelRow(img image.Image) int {
	b := img.Bounds()
	for y := b.Min.Y; y+1 < b.Max.Y; y++ {
		if transparentPixelRow(img, y) && transparentPixelRow(img, y+1) {
			continue
		}
		if equalPixelRows(img, y, y+1) {
			return y - b.Min.Y
		}
	}
	return -1
}

func transparentPixelRow(img image.Image, y int) bool {
	bounds := img.Bounds()
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		_, _, _, a := img.At(x, y).RGBA()
		if a != 0 {
			return false
		}
	}
	return true
}

func equalPixelRows(img image.Image, y0, y1 int) bool {
	bounds := img.Bounds()
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		r0, g0, b0, a0 := img.At(x, y0).RGBA()
		r1, g1, b1, a1 := img.At(x, y1).RGBA()
		if r0 != r1 || g0 != g1 || b0 != b1 || a0 != a1 {
			return false
		}
	}
	return true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
