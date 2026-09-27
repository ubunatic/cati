package sparkline

import (
	"image"
	"image/color"
	"strings"
	"sync"
	"testing"

	"ubunatic.com/cati/v1/core"
)

func TestRenderJMatchesSerial(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: uint8(11 * x), G: uint8(19 * y), B: uint8(x + y*2), A: 255})
		}
	}

	var serial, parallel strings.Builder
	if err := RenderOpts(&serial, img, 4, 2, Vertical); err != nil {
		t.Fatalf("Render serial: %v", err)
	}
	if err := RenderJ(&parallel, img, 4, 2, Vertical, 4); err != nil {
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
		grid, err := RenderToGrid(image.NewRGBA(image.Rect(0, 0, 64, 48)), 8, Options{Rows: 6, Jobs: jobs, OnProgress: func(p core.Progress) {
			mu.Lock()
			events = append(events, p)
			mu.Unlock()
		}})
		if err != nil {
			t.Fatal(err)
		}
		assertProgressEvents(t, events, grid.Height)
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
	img := image.NewRGBA(image.Rect(0, 0, 16, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: uint8(11 * x), G: uint8(19 * y), B: uint8(x + y*2), A: 255})
		}
	}

	serial := RenderToImage(img, 4, 2, Vertical)
	parallel := RenderToImageJ(img, 4, 2, Vertical, 4)
	if serial.Bounds() != parallel.Bounds() {
		t.Fatalf("RenderToImageJ bounds = %v, want %v", parallel.Bounds(), serial.Bounds())
	}
	for y := serial.Bounds().Min.Y; y < serial.Bounds().Max.Y; y++ {
		for x := serial.Bounds().Min.X; x < serial.Bounds().Max.X; x++ {
			if serial.At(x, y) != parallel.At(x, y) {
				t.Fatalf("RenderToImageJ pixel %d,%d differs: %v vs %v", x, y, serial.At(x, y), parallel.At(x, y))
			}
		}
	}
}
