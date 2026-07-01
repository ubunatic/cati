package quadblock

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestRenderJMatchesSerial(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 9, 7))
	for y := 0; y < 7; y++ {
		for x := 0; x < 9; x++ {
			img.Set(x, y, color.RGBA{R: uint8(17 * x), G: uint8(31 * y), B: uint8(x*3 + y), A: 255})
		}
	}

	var serial, parallel strings.Builder
	if err := RenderOpts(&serial, img, Options{}); err != nil {
		t.Fatalf("Render serial: %v", err)
	}
	if err := RenderJ(&parallel, img, Options{}, 4); err != nil {
		t.Fatalf("RenderJ parallel: %v", err)
	}
	if serial.String() != parallel.String() {
		t.Fatalf("RenderJ output differs\nserial:   %q\nparallel: %q", serial.String(), parallel.String())
	}
}

func TestRenderToImageJMatchesSerial(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 9, 7))
	for y := 0; y < 7; y++ {
		for x := 0; x < 9; x++ {
			img.Set(x, y, color.RGBA{R: uint8(17 * x), G: uint8(31 * y), B: uint8(x*3 + y), A: 255})
		}
	}

	serial := RenderToImage(img, Options{})
	parallel := RenderToImageJ(img, Options{}, 4)
	if !imagesEqual(serial, parallel) {
		t.Fatal("RenderToImageJ output differs from serial RenderToImage")
	}
}
