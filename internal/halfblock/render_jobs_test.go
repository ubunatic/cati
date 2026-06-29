package halfblock

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestRenderJMatchesSerial(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 9, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 9; x++ {
			img.Set(x, y, color.RGBA{R: uint8(17 * x), G: uint8(31 * y), B: uint8(x + y), A: 255})
		}
	}

	var serial, parallel strings.Builder
	if err := Render(&serial, img); err != nil {
		t.Fatalf("Render serial: %v", err)
	}
	if err := RenderJ(&parallel, img, 4); err != nil {
		t.Fatalf("RenderJ parallel: %v", err)
	}
	if serial.String() != parallel.String() {
		t.Fatalf("RenderJ output differs\nserial:   %q\nparallel: %q", serial.String(), parallel.String())
	}
}

func TestRenderToImageJMatchesSerial(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 9, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 9; x++ {
			img.Set(x, y, color.RGBA{R: uint8(17 * x), G: uint8(31 * y), B: uint8(x + y), A: 255})
		}
	}

	serial := RenderToImage(img)
	parallel := RenderToImageJ(img, 4)
	if serial.Bounds() != parallel.Bounds() {
		t.Fatalf("RenderToImageJ bounds = %v, want %v", parallel.Bounds(), serial.Bounds())
	}
	for y := serial.Bounds().Min.Y; y < serial.Bounds().Max.Y; y++ {
		for x := serial.Bounds().Min.X; x < serial.Bounds().Max.X; x++ {
			if serial.RGBAAt(x, y) != parallel.RGBAAt(x, y) {
				t.Fatalf("RenderToImageJ pixel %d,%d differs: %v vs %v", x, y, serial.RGBAAt(x, y), parallel.RGBAAt(x, y))
			}
		}
	}
}
