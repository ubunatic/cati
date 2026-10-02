package braille

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestRenderJobsParity(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 16; x++ {
			if (x+y)%2 == 0 {
				img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{B: 255, A: 255})
			}
		}
	}

	var buf1, buf4 bytes.Buffer
	opts1 := Options{Mode: ModeForegroundBackground, Jobs: 1, NoLinePrefix: true}
	opts4 := Options{Mode: ModeForegroundBackground, Jobs: 4, NoLinePrefix: true}

	if err := Render(&buf1, img, 8, opts1); err != nil {
		t.Fatalf("Render jobs=1: %v", err)
	}
	if err := Render(&buf4, img, 8, opts4); err != nil {
		t.Fatalf("Render jobs=4: %v", err)
	}

	if buf1.String() != buf4.String() {
		t.Fatalf("Render output mismatch between jobs=1 and jobs=4")
	}
}
