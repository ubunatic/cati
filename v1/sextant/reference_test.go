package sextant

import (
	"image"
	"image/color"
	"image/png"
	"math/bits"
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/cati/internal/imgutil"
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
			grid[row][col] = chooseCellReference(pixels)
		}
	}
	return grid
}

func chooseCellReference(pixels [6]color.RGBA) cellResult {
	allTransparent := true
	for _, pixel := range pixels {
		if pixel.A != 0 {
			allTransparent = false
			break
		}
	}
	if allTransparent {
		return cellResult{ch: ' ', transparent: true}
	}

	preferred := directMask(pixels)
	best := cellResult{ch: ' ', transparent: true}
	bestScore, bestOverlap, bestPopcount := int(^uint(0)>>1), -1, -1
	for mask := uint8(0); mask < 64; mask++ {
		cell, score := scoreMask(pixels, mask)
		overlap := bits.OnesCount8(mask & preferred)
		pop := bits.OnesCount8(mask)
		if score < bestScore || score == bestScore && (overlap > bestOverlap || overlap == bestOverlap && pop > bestPopcount) {
			best, bestScore, bestOverlap, bestPopcount = cell, score, overlap, pop
		}
	}
	return best
}

func renderToImageReference(img image.Image, mode Mode) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for cy, row := range renderReference(img, mode) {
		for cx, cell := range row {
			for i := 0; i < blockCols*blockRows; i++ {
				if !emittedCoverage(cell, i) {
					continue
				}
				pixel := cell.bg
				if maskContains(cell.mask, i) {
					pixel = cell.fg
				}
				x, y := cx*blockCols+i%blockCols, cy*blockRows+i/blockCols
				if x < b.Dx() && y < b.Dy() {
					out.SetRGBA(x, y, pixel)
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
			source := referenceGoldenLoad(t, sourcePath)
			w, h, extH := imgutil.FitDimsRatio(source.Bounds().Dx(), source.Bounds().Dy(), blockCols, blockRows, 4, 3, tc.chars, tc.chars)
			scaled := imgutil.ScaleNN(source, w, h)
			if extH > 0 {
				scaled = imgutil.AppendTransparentRows(scaled, extH)
			}
			native := renderToImageReference(scaled, ModeSextant)
			got := upscaleReferenceImage(native, 6, 8)
			goldenPath := filepath.Join("..", "..", "testdata", tc.folder, "render_sextant_"+string(rune('0'+tc.chars))+"ch.png")
			want := referenceGoldenLoad(t, goldenPath)
			if !referenceImagesEqual(got, want) {
				t.Fatalf("reference image differs from %s: got %v, want %v", goldenPath, got.Bounds(), want.Bounds())
			}
		})
	}
}

func referenceGoldenLoad(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func upscaleReferenceImage(img image.Image, scaleX, scaleY int) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx()*scaleX, b.Dy()*scaleY))
	for y := range out.Bounds().Dy() {
		for x := range out.Bounds().Dx() {
			out.Set(x, y, img.At(b.Min.X+x/scaleX, b.Min.Y+y/scaleY))
		}
	}
	return out
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
