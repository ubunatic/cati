package quadblock_test

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"ubunatic.com/cati/v1/core"
	"ubunatic.com/cati/v1/quadblock"
)

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

	nrgbaPartialAlphaImg := image.NewNRGBA(rect)
	for y := range 32 {
		for x := range 32 {
			nrgbaPartialAlphaImg.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
		}
	}

	subRect := image.Rect(5, 7, 37, 39)
	nrgbaSubImg := image.NewNRGBA(subRect)
	for y := 7; y < 39; y++ {
		for x := 5; x < 37; x++ {
			nrgbaSubImg.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 5), G: uint8(y * 8), B: uint8((x + y) * 4), A: 255})
		}
	}

	testCases := []struct {
		name string
		img  image.Image
	}{
		{"RGBA Image", rgbaImg},
		{"NRGBA Image", nrgbaImg},
		{"Gray Image", grayImg},
		{"NRGBA Partial Alpha Image", nrgbaPartialAlphaImg},
		{"NRGBA Nonzero Bounds Image", nrgbaSubImg},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fastpath output
			core.Fastpath = true
			var bufFast bytes.Buffer
			opts := quadblock.Options{SplitHalf: true}
			if err := quadblock.Render(&bufFast, tc.img, 16, opts); err != nil {
				t.Fatalf("Fastpath Render error: %v", err)
			}
			imgFast := quadblock.RenderToImage(tc.img, opts)

			// Fallback output
			core.Fastpath = false
			var bufSimple bytes.Buffer
			if err := quadblock.Render(&bufSimple, tc.img, 16, opts); err != nil {
				t.Fatalf("Simple Render error: %v", err)
			}
			imgSimple := quadblock.RenderToImage(tc.img, opts)

			// Reset
			core.Fastpath = true

			// Check ANSI string rendering parity
			if !bytes.Equal(bufFast.Bytes(), bufSimple.Bytes()) {
				t.Fatalf("Render ANSI output mismatch between Fastpath and Fallback for %s:\nFast:\n%q\nSimple:\n%q", tc.name, bufFast.String(), bufSimple.String())
			}

			// Check RenderToImage pixel parity
			bFast := imgFast.Bounds()
			bSimple := imgSimple.Bounds()
			if bFast != bSimple {
				t.Fatalf("RenderToImage bounds mismatch: %v vs %v", bFast, bSimple)
			}

			for y := bFast.Min.Y; y < bFast.Max.Y; y++ {
				for x := bFast.Min.X; x < bFast.Max.X; x++ {
					if imgFast.At(x, y) != imgSimple.At(x, y) {
						t.Fatalf("RenderToImage pixel mismatch at (%d,%d) for %s: %v vs %v", x, y, tc.name, imgFast.At(x, y), imgSimple.At(x, y))
					}
				}
			}
		})
	}
}
