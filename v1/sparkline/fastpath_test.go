package sparkline_test

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"ubunatic.com/cati/v1/core"
	"ubunatic.com/cati/v1/sparkline"
)

// TestFastpathParity asserts the RGBA fast path matches the simple path byte for byte.
func TestFastpathParity(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := range 40 {
		for x := range 40 {
			a := uint8(255)
			if (x+y)%11 == 0 {
				a = 0
			}
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 6), G: uint8(y * 6), B: uint8(x * y), A: a})
		}
	}
	render := func(fast bool) []byte {
		core.Fastpath = fast
		var buf bytes.Buffer
		if err := sparkline.Render(&buf, img, 20, sparkline.Options{}); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	defer func(v bool) { core.Fastpath = v }(core.Fastpath)
	if fast, simple := render(true), render(false); !bytes.Equal(fast, simple) {
		t.Fatalf("fast path output differs from simple path")
	}
}
