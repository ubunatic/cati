package quadblock

import (
	"image"
	"image/color"
	"strings"
	"sync"
	"testing"

	"ubunatic.com/cati/v1/core"
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

func TestRenderProgress(t *testing.T) {
	for _, jobs := range []int{1, 8} {
		var mu sync.Mutex
		var events []core.Progress
		_, err := RenderToGrid(image.NewRGBA(image.Rect(0, 0, 64, 48)), 0, Options{Jobs: jobs, OnProgress: func(p core.Progress) {
			mu.Lock()
			events = append(events, p)
			mu.Unlock()
		}})
		if err != nil {
			t.Fatal(err)
		}
		assertProgressEvents(t, events, 32*24)
	}
}

func assertProgressEvents(t *testing.T, events []core.Progress, total int) {
	t.Helper()
	if len(events) != total {
		t.Fatalf("events = %d, want %d", len(events), total)
	}
	for _, p := range events {
		if p.Stage != core.StageRendering || p.Current < 1 || p.Current > total || p.Total != total || p.Ratio < 0 || p.Ratio > 1 || p.Ratio != float64(p.Current)/float64(total) {
			t.Errorf("invalid progress event: %+v", p)
		}
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
