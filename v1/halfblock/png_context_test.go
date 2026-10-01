package halfblock

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/cati/v1/core"
)

func writeTestPNG(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadImageContextProgress(t *testing.T) {
	var events []core.Progress
	img, err := LoadImageContext(context.Background(), writeTestPNG(t), func(p core.Progress) { events = append(events, p) })
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 2 || len(events) < 2 {
		t.Fatalf("image=%v events=%#v", img.Bounds(), events)
	}
	if events[0].Stage != core.StageLoading {
		t.Fatalf("first stage=%q", events[0].Stage)
	}
	foundDecode := false
	for _, event := range events {
		if event.Stage == core.StageDecoding {
			foundDecode = true
		}
	}
	if !foundDecode {
		t.Fatalf("missing decoding event: %#v", events)
	}
}

func TestLoadImageContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var events []core.Progress
	img, err := LoadImageContext(ctx, writeTestPNG(t), func(p core.Progress) { events = append(events, p) })
	if img != nil || !errors.Is(err, context.Canceled) || len(events) != 1 || events[0].Stage != core.StageError {
		t.Fatalf("image=%v err=%v events=%#v", img, err, events)
	}
}

func TestLoadImageAsync(t *testing.T) {
	ch := LoadImageAsync(context.Background(), writeTestPNG(t), nil)
	result, ok := <-ch
	if !ok || result.Err != nil || result.Image == nil {
		t.Fatalf("result=%#v open=%v", result, ok)
	}
	if _, ok := <-ch; ok {
		t.Fatal("result channel should close after one result")
	}
}
