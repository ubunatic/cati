package cmd

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMediaBenchmarkImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bench.png")
	img := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 17), G: uint8(y * 25), A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runMediaBenchmark(&out, path, 20, 8, 1, 2); err != nil {
		t.Fatalf("runMediaBenchmark: %v", err)
	}
	if !strings.Contains(out.String(), "Image benchmark: bench.png") || !strings.Contains(out.String(), "Avg/render") {
		t.Fatalf("unexpected benchmark output:\n%s", out.String())
	}
}

func TestRunMediaBenchmarkValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		path  string
		iters int
		want  string
	}{
		{name: "missing file", path: filepath.Join(t.TempDir(), "missing.png"), iters: 1, want: "stat media file"},
		{name: "iterations", path: "unused.png", iters: 0, want: "--bench-iterations"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runMediaBenchmark(&bytes.Buffer{}, tc.path, 10, 5, 1, tc.iters)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestRunMediaBenchmarkVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	path := filepath.Join(t.TempDir(), "bench.mp4")
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=32x24:rate=4:duration=1", "-pix_fmt", "yuv420p", "-an", "-y", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("unable to create video fixture: %v (%s)", err, output)
	}
	var out bytes.Buffer
	if err := runMediaBenchmark(&out, path, 12, 6, 1, 1); err != nil {
		t.Fatalf("runMediaBenchmark: %v", err)
	}
	if !strings.Contains(out.String(), "Video benchmark: bench.mp4") || !strings.Contains(out.String(), "Frames") {
		t.Fatalf("unexpected benchmark output:\n%s", out.String())
	}
}
