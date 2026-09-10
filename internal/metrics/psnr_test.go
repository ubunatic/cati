package metrics

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestPSNR(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 2, 1))
	b := image.NewRGBA(image.Rect(0, 0, 2, 1))
	a.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	b.SetRGBA(0, 0, color.RGBA{0, 0, 0, 255})
	if got := PSNR(a, a); !math.IsInf(got, 1) {
		t.Fatalf("PSNR identical = %v, want +Inf", got)
	}
	if got := PSNR(a, b); !(got > 0 && !math.IsInf(got, 0)) {
		t.Fatalf("PSNR different = %v, want finite positive value", got)
	}
	if got := PSNR(a, image.NewRGBA(image.Rect(0, 0, 1, 1))); got != 0 {
		t.Fatalf("PSNR dimension mismatch = %v, want 0", got)
	}
}
