package sparkline

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/cati/internal/imgutil"
	"ubunatic.com/cati/v1/core"
)

// searchReference is a straightforward candidate-by-candidate pixel scan. It
// intentionally avoids bit masks, precomputed candidates, and typed Pix reads.
func searchReference(img image.Image, x0, y0, w, h int, candidates []candidate) cellResult {
	best := cellResult{Ch: ' ', Err: math.MaxFloat64}
	bestTransparent, bestSplit := math.MaxInt, math.MaxInt
	for _, cand := range candidates {
		var fg, bg acc
		var fn, bn, ft, bt int
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, b, a := img.At(x0+x, y0+y).RGBA()
				p := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
				isFG := cand.mask(x, y, w, h)
				if p.A == 0 {
					if isFG {
						ft++
					} else {
						bt++
					}
					continue
				}
				if isFG {
					fg.r += float64(p.R)
					fg.g += float64(p.G)
					fg.b += float64(p.B)
					fn++
				} else {
					bg.r += float64(p.R)
					bg.g += float64(p.G)
					bg.b += float64(p.B)
					bn++
				}
			}
		}
		fc, bc := avgColorOpaque(fg, fn), avgColorOpaque(bg, bn)
		if fc.A == 0 && bc.A != 0 && cand.ch != ' ' {
			continue
		}
		var err float64
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, b, a := img.At(x0+x, y0+y).RGBA()
				if a == 0 {
					continue
				}
				p := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
				if cand.mask(x, y, w, h) {
					err += sqDist(p, fc)
				} else {
					err += sqDist(p, bc)
				}
			}
		}
		if bc.A != 0 {
			err += float64(bt * 195075)
		}
		if fc.A != 0 || bc.A != 0 {
			err += float64(ft * 195075)
		}
		split := 0
		if fc.A != 0 && bc.A != 0 {
			split = 1
		}
		if err < best.Err || (err == best.Err && ft < bestTransparent) || (err == best.Err && ft == bestTransparent && split < bestSplit) {
			best = cellResult{Ch: cand.ch, FG: fc, BG: bc, Err: err, MaskFn: cand.mask, HasMask: true}
			bestTransparent, bestSplit = ft, split
		}
	}
	return best
}

func renderToImageReference(img image.Image, outCols, outRows int, mode Mode) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(b)
	if outCols <= 0 || outRows <= 0 {
		return out
	}
	for row := 0; row < outRows; row++ {
		y0 := b.Min.Y + row*b.Dy()/outRows
		y1 := b.Min.Y + (row+1)*b.Dy()/outRows
		for col := 0; col < outCols; col++ {
			x0 := b.Min.X + col*b.Dx()/outCols
			x1 := b.Min.X + (col+1)*b.Dx()/outCols
			if x1 <= x0 || y1 <= y0 {
				continue
			}
			cell := searchReference(img, x0, y0, x1-x0, y1-y0, candidatesForMode(mode))
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					out.SetRGBA(x, y, reconstructedCellColor(cell, x-x0, y-y0, x1-x0, y1-y0))
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
		for _, modeCase := range []struct {
			name           string
			mode           Mode
			cellW, cellH   int
			scaleX, scaleY int
			goldenName     string
		}{
			{"spark", Quad, 4, 8, 3, 3, "spark"},
			{"spark_best", Best, 4, 24, 3, 1, "spark_best"},
		} {
			modeCase := modeCase
			t.Run(fmt.Sprintf("%s_%d_%s", tc.folder, tc.chars, modeCase.name), func(t *testing.T) {
				t.Parallel()
				sourcePath := filepath.Join("..", "..", "testdata", tc.folder, "source.png")
				source := referenceGoldenLoad(t, sourcePath)
				aspectNum, aspectDen := 1, 1
				if modeCase.mode == Best {
					aspectNum, aspectDen = 1, 3
				}
				w, h, extH := imgutil.FitDimsRatio(source.Bounds().Dx(), source.Bounds().Dy(), modeCase.cellW, modeCase.cellH, aspectNum, aspectDen, tc.chars, tc.chars)
				scaled := imgutil.ScaleNN(source, w, h)
				if extH > 0 {
					scaled = imgutil.AppendTransparentRows(scaled, extH)
				}
				outCols := max(1, scaled.Bounds().Dx()/modeCase.cellW)
				outRows := max(1, scaled.Bounds().Dy()/modeCase.cellH)
				got := upscaleReferenceImage(renderToImageReference(scaled, outCols, outRows, modeCase.mode), modeCase.scaleX, modeCase.scaleY)
				goldenPath := filepath.Join("..", "..", "testdata", tc.folder, fmt.Sprintf("render_%s_%dch.png", modeCase.goldenName, tc.chars))
				want := referenceGoldenLoad(t, goldenPath)
				if !referenceImagesEqual(got, want) {
					t.Fatalf("reference image differs from %s: got %v, want %v", goldenPath, got.Bounds(), want.Bounds())
				}
			})
		}
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
	candidates := candidatesForMode(Vertical)
	fixtures := map[string]image.Image{
		"rgba":                referenceImage(image.NewRGBA(image.Rect(0, 0, 4, 8))),
		"nrgba_partial_alpha": referenceImage(image.NewNRGBA(image.Rect(0, 0, 4, 8))),
		"gray":                referenceImage(image.NewGray(image.Rect(0, 0, 4, 8))),
		"odd_subimage":        referenceSubImage(),
	}
	for name, img := range fixtures {
		t.Run(name, func(t *testing.T) {
			b := img.Bounds()
			got := findBestCandidateFast(img, b.Min.X, b.Min.X+3, b.Min.Y, b.Min.Y+7, makeBitCandidates(candidates, 4, 8))
			want := searchReference(img, b.Min.X, b.Min.Y, 4, 8, candidates)
			if got.Ch != want.Ch || got.FG != want.FG || got.BG != want.BG || got.Err != want.Err {
				t.Fatalf("got %+v, reference %+v", got, want)
			}
		})
	}
}

func referenceImage(img image.Image) image.Image {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBA{R: uint8(x*41 + 3), G: uint8(y*27 + 7), B: uint8((x+y)*19 + 1), A: []uint8{0, 1, 128, 254, 255}[(x+y+40)%5]}
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
	parent := referenceImage(image.NewNRGBA(image.Rect(-4, 3, 11, 17))).(*image.NRGBA)
	return parent.SubImage(image.Rect(1, 6, 6, 15))
}
