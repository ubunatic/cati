package halfblock

import (
	"bytes"
	"image"
	"image/color"
	"reflect"
	"testing"

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
