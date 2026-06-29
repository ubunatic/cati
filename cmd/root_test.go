package cmd

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"codeberg.org/ubunatic/cati/internal/imgutil"
	"codeberg.org/ubunatic/cati/internal/quadblock"
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
		{"small quad image stays visible", renderCfg{mode: modeQuad}, 1, 1, 1, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.rc.mode.viewSpec()
			gotW, gotH := imgutil.AlignCellSize(tc.w, tc.h, spec.CellW, spec.CellH)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Fatalf("AlignCellSize(%d,%d,cw=%d,ch=%d) = %dx%d, want %dx%d", tc.w, tc.h, spec.CellW, spec.CellH, gotW, gotH, tc.wantW, tc.wantH)
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

func staticViewportForTest(src image.Image, width int, rc renderCfg) image.Image {
	return prepareRenderedImage(src, nil, width, 0, rc, "")
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
