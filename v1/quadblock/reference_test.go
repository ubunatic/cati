package quadblock

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/cati/internal/imgutil"
	"ubunatic.com/cati/v1/core"
)

// renderReference samples each quadrant with At and walks the cells in order,
// keeping the same left/above context used by the production compiler.
func renderReference(img image.Image, opts Options) [][]core.Cell {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	cols, rows := (w+1)/2, (h+1)/2
	compiled := make([]quadCell, cols*rows)
	grid := make([][]core.Cell, rows)
	for y := range rows {
		grid[y] = make([]core.Cell, cols)
		for x := range cols {
			var pixels [4]color.RGBA
			for i, d := range [4]image.Point{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				px, py := b.Min.X+x*2+d.X, b.Min.Y+y*2+d.Y
				if px < b.Max.X && py < b.Max.Y {
					r, g, bl, a := img.At(px, py).RGBA()
					pixels[i] = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bl >> 8), A: uint8(a >> 8)}
				}
			}
			var left, above *quadCell
			if x > 0 {
				left = &compiled[y*cols+x-1]
			}
			if y > 0 {
				above = &compiled[(y-1)*cols+x]
			}
			c := compileCell(pixels, left, above, opts)
			compiled[y*cols+x] = c
			grid[y][x] = core.Cell{Ch: c.ch, Fg: c.fg, Bg: c.bg, HasFg: c.hasFG, HasBg: c.hasBG, Transparent: c.transparent}
		}
	}
	return grid
}

func renderToImageReference(img image.Image, opts Options) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for cy, row := range renderReference(img, opts) {
		for cx, c := range row {
			if c.Transparent {
				continue
			}
			mask := charToMask(c.Ch)
			for q, d := range [4]image.Point{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				x, y := cx*2+d.X, cy*2+d.Y
				if x >= b.Dx() || y >= b.Dy() {
					continue
				}
				src := color.RGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.RGBA)
				if src.A == 0 {
					continue
				}
				var pixel color.RGBA
				switch c.Ch {
				case '▌':
					pixel = c.Bg
					if d.X == 0 {
						pixel = c.Fg
					}
				case '▐':
					pixel = c.Bg
					if d.X == 1 {
						pixel = c.Fg
					}
				default:
					pixel = c.Bg
					if mask&(uint8(8)>>uint(q)) != 0 {
						pixel = c.Fg
					}
				}
				if !c.HasFg && !c.HasBg {
					continue
				}
				if !c.HasFg {
					pixel = c.Bg
				}
				if !c.HasBg && c.HasFg {
					pixel = c.Fg
				}
				out.SetRGBA(x, y, pixel)
			}
		}
	}
	return out
}

func TestReferenceGoldenParity(t *testing.T) {
	t.Parallel()
	type testCase struct {
		folder string
		chars  int
	}
	cases := []testCase{
		{"demo_horiz_20x20", 2}, {"demo_horiz_20x20", 4}, {"demo_horiz_20x20", 5},
		{"demo_verti_20x20", 2}, {"demo_verti_20x20", 4}, {"demo_verti_20x20", 5},
		{"demo_horiz_4x4", 2}, {"demo_horiz_4x4", 4},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("%s_%d", tc.folder, tc.chars), func(t *testing.T) {
			t.Parallel()
			sourcePath := filepath.Join("..", "..", "testdata", tc.folder, "source.png")
			f, err := os.Open(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			src, err := png.Decode(f)
			_ = f.Close()
			if err != nil {
				t.Fatal(err)
			}
			w, h, extH := imgutil.FitDims(src.Bounds().Dx(), src.Bounds().Dy(), 2, 2, 2, tc.chars, tc.chars)
			scaled := imgutil.ScaleNN(src, w, h)
			if extH > 0 {
				scaled = imgutil.AppendTransparentRows(scaled, extH)
			}
			opts := Options{SplitHalf: true, HalfblockThreshold: 2}
			gotNative := renderToImageReference(scaled, opts)
			got := image.NewRGBA(image.Rect(0, 0, gotNative.Bounds().Dx()*6, gotNative.Bounds().Dy()*12))
			for y := range got.Bounds().Dy() {
				for x := range got.Bounds().Dx() {
					got.Set(x, y, gotNative.At(x/6, y/12))
				}
			}
			goldenPath := filepath.Join("..", "..", "testdata", tc.folder, fmt.Sprintf("render_quad_%dch.png", tc.chars))
			gf, err := os.Open(goldenPath)
			if err != nil {
				t.Fatal(err)
			}
			want, err := png.Decode(gf)
			_ = gf.Close()
			if err != nil {
				t.Fatal(err)
			}
			if !imagesEqual(got, want) {
				t.Fatalf("reference image differs from %s", goldenPath)
			}
		})
	}
}

func TestReferenceParity(t *testing.T) {
	fixtures := map[string]image.Image{
		"rgba_odd":            referenceImage(image.NewRGBA(image.Rect(0, 0, 9, 7))),
		"nrgba_alpha":         referenceImage(image.NewNRGBA(image.Rect(0, 0, 9, 7))),
		"gray":                referenceImage(image.NewGray(image.Rect(0, 0, 9, 7))),
		"translated_subimage": referenceSubImage(),
	}
	for name, img := range fixtures {
		t.Run(name, func(t *testing.T) {
			for _, jobs := range []int{1, 3} {
				got, err := RenderToGrid(img, 0, Options{Jobs: jobs})
				if err != nil {
					t.Fatal(err)
				}
				want := renderReference(img, Options{})
				for y := range want {
					for x := range want[y] {
						if got.Cells[y][x] != want[y][x] {
							t.Fatalf("jobs=%d cell (%d,%d) = %+v, reference %+v", jobs, x, y, got.Cells[y][x], want[y][x])
						}
					}
				}
			}
		})
	}
}

func referenceImage(img image.Image) image.Image {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBA{R: uint8(x*31 + 9), G: uint8(y*27 + 5), B: uint8((x+y)*19 + 1), A: []uint8{0, 1, 128, 255}[(x+y+40)%4]}
			switch m := img.(type) {
			case *image.RGBA:
				m.SetRGBA(x, y, toRGBA(c))
			case *image.NRGBA:
				m.SetNRGBA(x, y, c)
			case *image.Gray:
				m.SetGray(x, y, color.Gray{Y: c.R})
			}
		}
	}
	return img
}

func referenceSubImage() image.Image {
	parent := referenceImage(image.NewNRGBA(image.Rect(-4, 3, 15, 20))).(*image.NRGBA)
	return parent.SubImage(image.Rect(2, 8, 12, 15))
}
