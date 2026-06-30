package geomshape

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func patternImage(mask uint8, on, off color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := 0; i < 4; i++ {
		c := off
		if maskContains(mask, i) {
			c = on
		}
		img.Set(i%2, i/2, c)
	}
	return img
}

func transparentImage(w, h int) image.Image {
	return image.NewRGBA(image.Rect(0, 0, w, h))
}

func TestGeomShapeTable(t *testing.T) {
	if got := len(geomshapeMasks); got != 16 {
		t.Fatalf("len(geomshapeMasks) = %d, want 16", got)
	}
	if got := len(geomshapeRuneByMask); got != 16 {
		t.Fatalf("len(geomshapeRuneByMask) = %d, want 16", got)
	}
	if got := geomshapeRuneByMask[0]; got != '\U0001FB40' {
		t.Fatalf("mask 0 rune = %q, want %q", got, '\U0001FB40')
	}
	if got := geomshapeRuneByMask[15]; got != '\U0001FB4F' {
		t.Fatalf("mask 15 rune = %q, want %q", got, '\U0001FB4F')
	}
}

func TestGeomShapeDirectMask(t *testing.T) {
	on := color.RGBA{R: 255, A: 255}
	off := color.RGBA{B: 255, A: 255}
	cases := []struct {
		name string
		img  image.Image
	}{
		{"empty", transparentImage(2, 2)},
		{"single bit", patternImage(bitForIndex(0), on, off)},
		{"corner pair", patternImage(bitForIndex(0)|bitForIndex(3), on, off)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pixels := sampleBlock(tc.img, 0, 2, 0, 2, SamplerLegacy)
			cell := chooseCell(pixels, ModeShape)
			if tc.name == "empty" {
				if !cell.transparent {
					t.Fatal("chooseCell(2x2) returned visible glyph, want transparent cell")
				}
				return
			}
			if cell.transparent {
				t.Fatal("chooseCell(2x2) returned transparent cell, want visible glyph")
			}
		})
	}
}

func TestGeomShapeHeuristicModesStayOnSupportedMasks(t *testing.T) {
	on := color.RGBA{R: 255, A: 255}
	off := color.RGBA{B: 255, A: 255}
	cases := []struct {
		name string
		img  image.Image
	}{
		{"single bit", patternImage(bitForIndex(0), on, off)},
		{"diagonal", patternImage(bitForIndex(0)|bitForIndex(3), on, off)},
		{"almost full", patternImage(0b1111, on, on)},
	}
	supported := make(map[uint8]struct{}, len(geomshapeMasks))
	for _, mask := range geomshapeMasks {
		supported[mask] = struct{}{}
	}
	for _, tc := range cases {
		for _, mode := range []Mode{ModeGeom, ModeBest} {
			t.Run(tc.name+"/"+mode.String(), func(t *testing.T) {
				pixels := sampleBlock(tc.img, 0, 2, 0, 2, SamplerLegacy)
				cell := chooseCell(pixels, mode)
				if _, ok := supported[cell.mask]; !ok {
					t.Fatalf("chooseCell(%s) mask = %04b, want supported geomshape mask", mode, cell.mask)
				}
				if cell.transparent {
					t.Fatalf("chooseCell(%s) returned transparent cell, want visible geomshape mask", mode)
				}
			})
		}
	}
}

func TestGeomShapeDirectModeFallsBackFromHoleyMask(t *testing.T) {
	on := color.RGBA{R: 255, A: 255}
	off := color.RGBA{B: 255, A: 255}
	img := patternImage(bitForIndex(0)|bitForIndex(3), on, off)
	pixels := sampleBlock(img, 0, 2, 0, 2, SamplerLegacy)
	cell := chooseCell(pixels, ModeShape)
	if cell.transparent {
		t.Fatal("chooseCell(2x2) returned transparent cell, want visible fallback")
	}
	var buf bytes.Buffer
	if err := Render(&buf, img, ModeShape); err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}
}

func TestRenderToImagePreservesOpacity(t *testing.T) {
	on := color.RGBA{R: 255, A: 255}
	src := patternImage(0b1111, on, on)
	got := RenderToImage(src, ModeBest)
	gb := got.Bounds()
	if gb.Dx() != 2 || gb.Dy() != 2 {
		t.Fatalf("RenderToImage size = %dx%d, want 2x2", gb.Dx(), gb.Dy())
	}
	opaque := 0
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			gotPixel := toRGBA(got.At(x, y))
			if gotPixel.A != 0 {
				opaque++
			}
		}
	}
	if opaque == 0 {
		t.Fatal("RenderToImage() returned an all-transparent image")
	}
}

func TestGeomShapeV2SamplerIsIsolated(t *testing.T) {
	on := color.RGBA{R: 255, A: 255}
	off := color.RGBA{B: 255, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			img.Set(x, y, off)
		}
	}
	img.Set(1, 1, on)
	legacy := sampleBlock(img, 0, 3, 0, 3, SamplerLegacy)
	v2 := sampleBlock(img, 0, 3, 0, 3, SamplerV2)
	if legacy == v2 {
		t.Fatal("SamplerV2 matched legacy sampling exactly; want an isolated copy for tuning")
	}
}

func TestGeomShapeLegacySamplerMatchesWrapper(t *testing.T) {
	on := color.RGBA{R: 255, A: 255}
	off := color.RGBA{B: 255, A: 255}
	img := patternImage(bitForIndex(0)|bitForIndex(3), on, off)
	legacy := RenderToImage(img, ModeBest)
	withSampler := RenderToImageWithSampler(img, ModeBest, SamplerLegacy)
	if !bytes.Equal(legacy.Pix, withSampler.Pix) {
		t.Fatal("RenderToImageWithSampler(SamplerLegacy) differs from RenderToImage")
	}
}
