package quadblock

import (
	"image"
	"image/color"
	"testing"

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
