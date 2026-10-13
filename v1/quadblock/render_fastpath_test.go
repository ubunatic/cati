package quadblock

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"testing"

	"ubunatic.com/cati/v1/core"
)

func TestNRGBAPremultiplicationExactMatch(t *testing.T) {
	for a := 0; a <= 255; a++ {
		for r := 0; r <= 255; r++ {
			want := toRGBA(color.NRGBA{R: uint8(r), G: uint8(r), B: uint8(r), A: uint8(a)})
			got := nrgbaToRGBA(uint8(r), uint8(r), uint8(r), uint8(a))
			if got != want {
				t.Fatalf("nrgbaToRGBA mismatch for R=%d, A=%d: got %v, want %v", r, a, got, want)
			}
		}
	}
}

func TestFastpathDifferentialParity(t *testing.T) {
	rect := image.Rect(0, 0, 32, 32)

	rgbaImg := image.NewRGBA(rect)
	for y := range 32 {
		for x := range 32 {
			rgbaImg.SetRGBA(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 7), B: uint8((x + y) * 3), A: 255})
		}
	}

	nrgbaImg := image.NewNRGBA(rect)
	for y := range 32 {
		for x := range 32 {
			nrgbaImg.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 5), G: uint8(y * 8), B: uint8((x + y) * 4), A: 255})
		}
	}

	grayImg := image.NewGray(rect)
	for y := range 32 {
		for x := range 32 {
			grayImg.SetGray(x, y, color.Gray{Y: uint8((x + y) * 4)})
		}
	}

	testCases := []struct {
		name string
		img  image.Image
	}{
		{"RGBA Image", rgbaImg},
		{"NRGBA Image", nrgbaImg},
		{"Gray Image", grayImg},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertParity(t, tc.img, Options{SplitHalf: true}, 16, 1)
		})
	}
}

func TestNRGBAPartialAlphaFastpathParity(t *testing.T) {
	alphas := []uint8{0, 1, 128, 254, 255}
	sizes := []struct{ w, h int }{
		{32, 32},
		{33, 9}, // odd dimensions
		{3, 2},
		{1, 1},
	}

	for _, sz := range sizes {
		for _, alpha := range alphas {
			for _, jobs := range []int{1, 4} {
				name := fmt.Sprintf("%dx%d_alpha%d_jobs%d", sz.w, sz.h, alpha, jobs)
				img := image.NewNRGBA(image.Rect(0, 0, sz.w, sz.h))
				for y := 0; y < sz.h; y++ {
					for x := 0; x < sz.w; x++ {
						img.SetNRGBA(x, y, color.NRGBA{
							R: uint8((x*37 + y*17) % 256),
							G: uint8((x*13 + y*29) % 256),
							B: uint8((x*41 + y*7) % 256),
							A: alpha,
						})
					}
				}

				opts := Options{Jobs: jobs, NoLinePrefix: true}
				t.Run(name, func(t *testing.T) {
					assertParity(t, img, opts, 0, jobs)
				})
			}
		}
	}
}

func TestNonZeroBoundsFastpathParity(t *testing.T) {
	rects := []image.Rectangle{
		image.Rect(5, 7, 17, 19),
		image.Rect(10, 10, 25, 25),
		image.Rect(3, 3, 6, 5),
	}

	for idx, r := range rects {
		for _, jobs := range []int{1, 4} {
			nrgba := image.NewNRGBA(r)
			rgba := image.NewRGBA(r)
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					c := color.NRGBA{R: uint8(x * 10), G: uint8(y * 15), B: uint8((x + y) * 5), A: 128}
					nrgba.SetNRGBA(x, y, c)
					rgba.SetRGBA(x, y, toRGBA(c))
				}
			}

			opts := Options{Jobs: jobs, NoLinePrefix: true}

			t.Run(fmt.Sprintf("NRGBA_rect%d_jobs%d", idx, jobs), func(t *testing.T) {
				assertParity(t, nrgba, opts, 0, jobs)
			})
			t.Run(fmt.Sprintf("RGBA_rect%d_jobs%d", idx, jobs), func(t *testing.T) {
				assertParity(t, rgba, opts, 0, jobs)
			})

			parent := image.NewNRGBA(image.Rect(0, 0, 50, 50))
			for y := 0; y < 50; y++ {
				for x := 0; x < 50; x++ {
					parent.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 3), G: uint8(y * 4), B: 100, A: 128})
				}
			}
			sub := parent.SubImage(r).(*image.NRGBA)
			t.Run(fmt.Sprintf("SubNRGBA_rect%d_jobs%d", idx, jobs), func(t *testing.T) {
				assertParity(t, sub, opts, 0, jobs)
			})
		}
	}
}

func assertParity(t *testing.T, img image.Image, opts Options, cols, jobs int) {
	t.Helper()
	defer func() { core.Fastpath = true }()

	core.Fastpath = true
	var bufFast bytes.Buffer
	if err := Render(&bufFast, img, cols, opts); err != nil {
		t.Fatalf("Fastpath Render error: %v", err)
	}
	imgFast := RenderToImageJ(img, opts, jobs)

	core.Fastpath = false
	var bufSimple bytes.Buffer
	if err := Render(&bufSimple, img, cols, opts); err != nil {
		t.Fatalf("Simple Render error: %v", err)
	}
	imgSimple := RenderToImageJ(img, opts, jobs)

	if !bytes.Equal(bufFast.Bytes(), bufSimple.Bytes()) {
		t.Fatalf("ANSI output mismatch between Fastpath and Fallback")
	}

	bFast := imgFast.Bounds()
	bSimple := imgSimple.Bounds()
	if bFast != bSimple {
		t.Fatalf("RenderToImage bounds mismatch: %v vs %v", bFast, bSimple)
	}

	for y := bFast.Min.Y; y < bFast.Max.Y; y++ {
		for x := bFast.Min.X; x < bFast.Max.X; x++ {
			if imgFast.At(x, y) != imgSimple.At(x, y) {
				t.Fatalf("RenderToImage pixel mismatch at (%d,%d): %v vs %v", x, y, imgFast.At(x, y), imgSimple.At(x, y))
			}
		}
	}
}
