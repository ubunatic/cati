package cmd

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestComposedModeRendersResolvedGlyph(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	fg := color.RGBA{R: 240, G: 20, B: 10, A: 255}
	bg := color.RGBA{B: 220, A: 255}
	for y := range 2 {
		for x := range 2 {
			img.Set(x, y, bg)
		}
	}
	img.Set(0, 0, fg)

	rc, err := parseRenderMode("d4")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := rc.render(&out, img); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "▘") {
		t.Fatalf("d4 render = %q, want ▘", out.String())
	}
}

func TestComposedModeSerialAndWorkerReconstructionAgree(t *testing.T) {
	img := horizontalGradientNRGBA(48, 48)
	rc, err := parseRenderMode("Z")
	if err != nil {
		t.Fatal(err)
	}
	rc.jobs = 1
	serial := renderReconstruction(img, rc)
	rc.jobs = 4
	parallel := renderReconstruction(img, rc)
	if !sameImagePixels(serial, parallel) {
		t.Fatal("Z reconstruction differs between serial and worker paths")
	}
}

func sameImagePixels(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if color.RGBAModel.Convert(a.At(x, y)) != color.RGBAModel.Convert(b.At(x, y)) {
				return false
			}
		}
	}
	return true
}
