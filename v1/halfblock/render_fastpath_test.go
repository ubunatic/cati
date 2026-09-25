package halfblock_test

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"ubunatic.com/cati/v1/core"
	"ubunatic.com/cati/v1/halfblock"
)

func TestFastpathDifferentialParity(t *testing.T) {
	rect := image.Rect(0, 0, 32, 32)

	rgbaImg := image.NewRGBA(rect)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			rgbaImg.SetRGBA(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 7), B: uint8((x + y) * 3), A: 255})
		}
	}

	nrgbaImg := image.NewNRGBA(rect)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			nrgbaImg.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 5), G: uint8(y * 8), B: uint8((x + y) * 4), A: 255})
		}
	}

	grayImg := image.NewGray(rect)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
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
			opts := halfblock.Options{Rows: 16}

			// Fastpath output
			core.Fastpath = true
			var bufFast bytes.Buffer
			if err := halfblock.Render(&bufFast, tc.img, 16, opts); err != nil {
				t.Fatalf("Fastpath Render error: %v", err)
			}
			imgFast := halfblock.RenderToImage(tc.img)

			// Scaled images fastpath
			scaledFastNN := halfblock.ScaleNN(tc.img, 16, 16)
			scaledFastFit := halfblock.ScaleToFit(tc.img, 16, 16)

			// Fallback output
			core.Fastpath = false
			var bufSimple bytes.Buffer
			if err := halfblock.Render(&bufSimple, tc.img, 16, opts); err != nil {
				t.Fatalf("Simple Render error: %v", err)
			}
			imgSimple := halfblock.RenderToImage(tc.img)

			scaledSimpleNN := halfblock.ScaleNN(tc.img, 16, 16)
			scaledSimpleFit := halfblock.ScaleToFit(tc.img, 16, 16)

			// Reset
			core.Fastpath = true

			// Check ANSI string rendering parity
			if !bytes.Equal(bufFast.Bytes(), bufSimple.Bytes()) {
				t.Fatalf("Render ANSI output mismatch between Fastpath and Fallback for %s", tc.name)
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

			// Check ScaleNN parity
			for y := 0; y < 16; y++ {
				for x := 0; x < 16; x++ {
					if scaledFastNN.At(x, y) != scaledSimpleNN.At(x, y) {
						t.Fatalf("ScaleNN pixel mismatch at (%d,%d) for %s: %v vs %v", x, y, tc.name, scaledFastNN.At(x, y), scaledSimpleNN.At(x, y))
					}
				}
			}

			// Check ScaleToFit parity
			for y := 0; y < 16; y++ {
				for x := 0; x < 16; x++ {
					if scaledFastFit.At(x, y) != scaledSimpleFit.At(x, y) {
						t.Fatalf("ScaleToFit pixel mismatch at (%d,%d) for %s: %v vs %v", x, y, tc.name, scaledFastFit.At(x, y), scaledSimpleFit.At(x, y))
					}
				}
			}
		})
	}
}
