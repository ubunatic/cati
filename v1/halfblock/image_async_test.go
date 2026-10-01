package halfblock

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ubunatic.com/cati/v1/core"
)

func writeAsyncTestPNG(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 9), B: 42, A: 255})
		}
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadImageContextReportsProgress(t *testing.T) {
	path := writeAsyncTestPNG(t)
	var events []core.Progress
	img, err := LoadImageContext(context.Background(), path, func(p core.Progress) { events = append(events, p) })
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 32 || img.Bounds().Dy() != 24 {
		t.Fatalf("unexpected image bounds: %v", img.Bounds())
	}
	if len(events) < 4 {
		t.Fatalf("got %d progress events, want loading, decoding, byte progress, complete", len(events))
	}
	if events[0].Stage != core.StageLoading || events[1].Stage != core.StageDecoding || events[len(events)-1].Stage != core.StageComplete {
		t.Fatalf("unexpected stage sequence: %+v", events)
	}
	for _, event := range events {
		if event.Ratio < 0 || event.Ratio > 1 {
			t.Errorf("progress ratio out of bounds: %+v", event)
		}
	}
}

func TestLoadImageContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var events []core.Progress
	img, err := LoadImageContext(ctx, writeAsyncTestPNG(t), func(p core.Progress) { events = append(events, p) })
	if err != context.Canceled {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if img != nil {
		t.Fatal("canceled load returned an image")
	}
	if len(events) != 1 || events[0].Stage != core.StageError {
		t.Fatalf("unexpected cancellation events: %+v", events)
	}
}

func TestLoadImageAsyncReturnsResult(t *testing.T) {
	result := <-LoadImageAsync(context.Background(), writeAsyncTestPNG(t), nil)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Image == nil || result.Image.Bounds() != image.Rect(0, 0, 32, 24) {
		t.Fatalf("unexpected image result: %v", result.Image)
	}
}

func TestLoadImageSynchronousPathHasNoProgressOverhead(t *testing.T) {
	path := writeAsyncTestPNG(t)
	legacy := testing.AllocsPerRun(5, func() {
		if _, err := LoadImageWithTarget(path, 0, 0); err != nil {
			panic(err)
		}
	})
	current := testing.AllocsPerRun(5, func() {
		if _, err := LoadImage(path); err != nil {
			panic(err)
		}
	})
	if !reflect.DeepEqual(legacy, current) {
		t.Fatalf("LoadImage allocations = %v, LoadImageWithTarget allocations = %v", current, legacy)
	}
}
