package halfblock

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ubunatic.com/cati/internal/imgutil"
	"ubunatic.com/cati/v1/core"
)

// renderReference reads every source pixel through image.Image.At and assembles
// the grid independently of the production slice readers.
func renderReference(img image.Image) *core.Grid {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	rows := (h + 1) / 2
	cells := make([][]core.Cell, rows)
	for row := range rows {
		cells[row] = make([]core.Cell, w)
		for x := 0; x < w; x++ {
			read := func(y int) color.RGBA {
				if y >= b.Max.Y {
					return color.RGBA{}
				}
				r, g, bl, a := img.At(b.Min.X+x, y).RGBA()
				return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bl >> 8), A: uint8(a >> 8)}
			}
			top, bot := read(b.Min.Y+row*2), read(b.Min.Y+row*2+1)
			t, bt := top.A == 0, bot.A == 0
			cell := core.Cell{Ch: ' '}
			switch {
			case t && bt:
				cell.Transparent = true
			case t:
				cell.Ch, cell.Fg, cell.HasFg = '▄', bot, true
			case bt:
				cell.Ch, cell.Fg, cell.HasFg = '▀', top, true
			case top.R == bot.R && top.G == bot.G && top.B == bot.B:
				cell.Ch, cell.Fg, cell.HasFg = '█', top, true
			default:
				cell.Ch, cell.Fg, cell.Bg, cell.HasFg, cell.HasBg = '▀', top, bot, true, true
			}
			cells[row][x] = cell
		}
	}
	return &core.Grid{Width: w, Height: rows, Cells: cells}
}

func renderToImageReference(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y, row := range renderReference(img).Cells {
		for x, c := range row {
			if c.Transparent {
				continue
			}
			top, bottom := c.Bg, c.Bg
			switch c.Ch {
			case '▀':
				top = c.Fg
			case '▄':
				bottom = c.Fg
			case '█':
				top, bottom = c.Fg, c.Fg
			default:
				if c.HasFg {
					top = c.Fg
				}
				if c.HasBg {
					bottom = c.Bg
				}
			}
			if c.HasFg || c.HasBg {
				out.SetRGBA(x, y*2, top)
				if y*2+1 < b.Dy() {
					out.SetRGBA(x, y*2+1, bottom)
				}
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
		t.Run(filepath.Base(tc.folder)+"_"+string(rune('0'+tc.chars)), func(t *testing.T) {
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
			w, h, extH := imgutil.FitDims(src.Bounds().Dx(), src.Bounds().Dy(), 1, 2, 1, tc.chars, tc.chars)
			scaled := imgutil.ScaleNN(src, w, h)
			if extH > 0 {
				scaled = imgutil.AppendTransparentRows(scaled, extH)
			}
			gotNative := renderToImageReference(scaled)
			got := image.NewRGBA(image.Rect(0, 0, gotNative.Bounds().Dx()*12, gotNative.Bounds().Dy()*12))
			for y := range got.Bounds().Dy() {
				for x := range got.Bounds().Dx() {
					got.Set(x, y, gotNative.At(x/12, y/12))
				}
			}
			goldenPath := filepath.Join("..", "..", "testdata", tc.folder, "render_halfblock_"+string(rune('0'+tc.chars))+"ch.png")
			gf, err := os.Open(goldenPath)
			if err != nil {
				t.Fatal(err)
			}
			want, err := png.Decode(gf)
			_ = gf.Close()
			if err != nil {
				t.Fatal(err)
			}
			if !referenceImagesEqual(got, want) {
				t.Fatalf("reference image differs from %s", goldenPath)
			}
		})
	}
}

func referenceImagesEqual(a, b image.Image) bool {
	if a.Bounds().Dx() != b.Bounds().Dx() || a.Bounds().Dy() != b.Bounds().Dy() {
		return false
	}
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			ar, ag, ab, aa := a.At(a.Bounds().Min.X+x, a.Bounds().Min.Y+y).RGBA()
			br, bg, bb, ba := b.At(b.Bounds().Min.X+x, b.Bounds().Min.Y+y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}
	return true
}

func TestReferenceParity(t *testing.T) {
	fixtures := []struct {
		name string
		img  image.Image
	}{
		{"rgba", referenceImage(image.NewRGBA(image.Rect(0, 0, 9, 7)))},
		{"nrgba_partial_alpha", referenceImage(image.NewNRGBA(image.Rect(0, 0, 9, 7)))},
		{"gray", referenceImage(image.NewGray(image.Rect(0, 0, 9, 7)))},
		{"subimage_nonzero_bounds", referenceSubImage()},
	}
	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RenderToGrid(tc.img, 0, Options{})
			if err != nil {
				t.Fatal(err)
			}
			want := renderReference(tc.img)
			if !reflect.DeepEqual(got, want) {
				if got.Width != want.Width || got.Height != want.Height {
					t.Fatalf("grid dimensions %dx%d, reference %dx%d", got.Width, got.Height, want.Width, want.Height)
				}
				for y := range want.Cells {
					for x := range want.Cells[y] {
						if got.Cells[y][x] != want.Cells[y][x] {
							t.Fatalf("cell (%d,%d) = %+v, reference %+v", x, y, got.Cells[y][x], want.Cells[y][x])
						}
					}
				}
				t.Fatalf("grid differs from At-based reference")
			}
			var ansi bytes.Buffer
			if err := Render(&ansi, tc.img, 0, Options{NoLinePrefix: true}); err != nil {
				t.Fatal(err)
			}
			var expected bytes.Buffer
			for _, row := range want.Cells {
				for _, c := range row {
					if c.Transparent {
						expected.WriteByte(' ')
					} else {
						expected.WriteString(cellEscape(c))
						expected.WriteRune(c.Ch)
						expected.WriteString(ansiReset)
					}
				}
				expected.WriteByte('\n')
			}
			if !bytes.Equal(ansi.Bytes(), expected.Bytes()) {
				t.Fatal("ANSI output differs from At-based reference")
			}
		})
	}
}

func referenceImage(img image.Image) image.Image {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBA{R: uint8(x*37 + 11), G: uint8(y*29 + 7), B: uint8((x+y)*17 + 3), A: []uint8{0, 1, 128, 255}[(x+y-b.Min.X-b.Min.Y+40)%4]}
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
	parent := referenceImage(image.NewNRGBA(image.Rect(-3, 4, 12, 17))).(*image.NRGBA)
	return parent.SubImage(image.Rect(1, 7, 10, 14))
}
