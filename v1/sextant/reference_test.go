package sextant

import (
	"image"
	"image/color"
	"testing"

	"ubunatic.com/cati/v1/core"
)

// renderReference samples each 2x3 cell through img.At before selecting its
// sextant, independently checking the direct RGBA/NRGBA readers.
func renderReference(img image.Image, mode Mode) [][]cellResult {
	b := img.Bounds()
	cols := (b.Dx() + blockCols - 1) / blockCols
	rows := (b.Dy() + blockRows - 1) / blockRows
	grid := make([][]cellResult, rows)
	for row := range rows {
		grid[row] = make([]cellResult, cols)
		for col := range cols {
			var pixels [6]color.RGBA
			x0, x1 := b.Min.X+col*blockCols, min(b.Min.X+(col+1)*blockCols, b.Max.X)
			y0, y1 := b.Min.Y+row*blockRows, min(b.Min.Y+(row+1)*blockRows, b.Max.Y)
			if x1-x0 == blockCols && y1-y0 == blockRows {
				for i := range pixels {
					x, y := x0+i%blockCols, y0+i/blockCols
					r, g, bl, a := img.At(x, y).RGBA()
					pixels[i] = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bl >> 8), A: uint8(a >> 8)}
				}
			} else {
				for ry := 0; ry < blockRows; ry++ {
					sy0, sy1 := splitAxis(y0, y1, blockRows, ry)
					for cx := 0; cx < blockCols; cx++ {
						sx0, sx1 := splitAxis(x0, x1, blockCols, cx)
						var rr, gg, bb, n int
						for y := sy0; y < sy1; y++ {
							for x := sx0; x < sx1; x++ {
								r, g, bl, a := img.At(x, y).RGBA()
								if a != 0 {
									rr += int(uint8(r >> 8))
									gg += int(uint8(g >> 8))
									bb += int(uint8(bl >> 8))
									n++
								}
							}
						}
						if n > 0 {
							pixels[ry*blockCols+cx] = color.RGBA{R: uint8(rr / n), G: uint8(gg / n), B: uint8(bb / n), A: 255}
						}
					}
				}
			}
			grid[row][col] = chooseCell(pixels, mode)
		}
	}
	return grid
}

func TestReferenceParity(t *testing.T) {
	fixtures := map[string]image.Image{
		"rgba_odd":                referenceImage(image.NewRGBA(image.Rect(0, 0, 7, 8))),
		"nrgba_partial_alpha":     referenceImage(image.NewNRGBA(image.Rect(0, 0, 7, 8))),
		"gray":                    referenceImage(image.NewGray(image.Rect(0, 0, 7, 8))),
		"subimage_nonzero_bounds": referenceSubImage(),
	}
	for name, img := range fixtures {
		t.Run(name, func(t *testing.T) {
			want := renderReference(img, ModeSextant)
			grid, err := RenderToGrid(img, 0, Options{Mode: ModeSextant})
			if err != nil {
				t.Fatal(err)
			}
			for y := range want {
				for x, ref := range want[y] {
					got := grid.Cells[y][x]
					if got.Ch != ref.ch || got.Fg != ref.fg || got.Bg != ref.bg || got.HasFg != ref.hasFG || got.HasBg != ref.hasBG || got.Transparent != ref.transparent {
						t.Fatalf("cell (%d,%d) differs: got %+v, reference %+v", x, y, got, ref)
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
			c := color.NRGBA{R: uint8(x*43 + 13), G: uint8(y*29 + 17), B: uint8((x+y)*23 + 3), A: []uint8{0, 1, 128, 254, 255}[(x+y+40)%5]}
			switch m := img.(type) {
			case *image.RGBA:
				m.SetRGBA(x, y, core.NRGBAToRGBA(c.R, c.G, c.B, c.A))
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
	parent := referenceImage(image.NewNRGBA(image.Rect(-4, 5, 13, 19))).(*image.NRGBA)
	return parent.SubImage(image.Rect(1, 8, 10, 17))
}
