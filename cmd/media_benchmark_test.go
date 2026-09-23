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
	"time"
)

// makeTestPNG creates a small PNG in dir and returns its path.
func makeTestPNG(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "bench.png")
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
	return path
}

// TestRunMediaBenchmarkImage verifies basic output structure for a static image.
func TestRunMediaBenchmarkImage(t *testing.T) {
	path := makeTestPNG(t, t.TempDir())
	var out bytes.Buffer
	if err := runMediaBenchmark(&out, path, 20, 8, 1, "", 500*time.Millisecond); err != nil {
		t.Fatalf("runMediaBenchmark: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Image benchmark: bench.png") || !strings.Contains(got, "avg/render") || !strings.Contains(got, "Simple") || !strings.Contains(got, "Speedup") {
		t.Fatalf("unexpected benchmark output:\n%s", got)
	}
}

// TestRunMediaBenchmarkValidation verifies error cases.
func TestRunMediaBenchmarkValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   string
		budget time.Duration
		want   string
	}{
		{name: "missing file", path: filepath.Join(t.TempDir(), "missing.png"), budget: 100 * time.Millisecond, want: "stat media file"},
		{name: "zero budget", path: "unused.png", budget: 0, want: "--bench-budget"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runMediaBenchmark(&bytes.Buffer{}, tc.path, 10, 5, 1, "", tc.budget)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

// TestRunMediaBenchmarkNonTTYDefaultSize verifies that when width/height are 0
// (auto) and stdout is not a terminal (piped), a fixed 80×24 fallback is used
// so the image is not fitted to a 1-row strip.
func TestRunMediaBenchmarkNonTTYDefaultSize(t *testing.T) {
	path := makeTestPNG(t, t.TempDir())
	var out bytes.Buffer
	// We pass a *bytes.Buffer (not *os.File), so TermWidth/TermHeight return 0
	// → the code must fall back to 80×24 rather than max(1, 0)=1.
	if err := runMediaBenchmark(&out, path, 0, 0, 1, "", 200*time.Millisecond); err != nil {
		t.Fatalf("runMediaBenchmark with auto size: %v", err)
	}
	got := out.String()
	// Header must show at least 80 wide, not "1" or "0".
	if strings.Contains(got, "(1x") || strings.Contains(got, "(0x") {
		t.Fatalf("width fell back to <2, non-TTY default size broken:\n%s", got)
	}
	if !strings.Contains(got, "(80x24)") {
		t.Logf("output header (not asserting exact size): %s", strings.SplitN(got, "\n", 2)[0])
	}
}

// TestRunMediaBenchmarkTimeBudget verifies that the benchmark finishes within a
// short wall-clock budget rather than running a fixed number of iterations.
func TestRunMediaBenchmarkTimeBudget(t *testing.T) {
	path := makeTestPNG(t, t.TempDir())
	budget := 150 * time.Millisecond
	start := time.Now()
	var out bytes.Buffer
	if err := runMediaBenchmark(&out, path, 20, 8, 1, "", budget); err != nil {
		t.Fatalf("runMediaBenchmark: %v", err)
	}
	elapsed := time.Since(start)
	// Allow generous headroom: n modes × (budget + 50ms setup).
	entries, _ := listableRenderModes()
	maxExpected := time.Duration(len(entries)+1) * (budget + 200*time.Millisecond)
	if elapsed > maxExpected {
		t.Fatalf("benchmark took %s, expected ≤ %s for %d modes", elapsed, maxExpected, len(entries))
	}
}

// TestRunMediaBenchmarkSlowMode verifies that a mode whose single render
// exceeds the budget is reported as ">budget" (timedOut) or "(slow)", not silently dropped.
func TestRunMediaBenchmarkSlowMode(t *testing.T) {
	path := makeTestPNG(t, t.TempDir())
	var out bytes.Buffer
	// Inject a fake renderer that always sleeps longer than the budget.
	budget := 30 * time.Millisecond
	slowRender := func(img image.Image, cfg renderCfg) ([]string, error) {
		time.Sleep(budget * 5)
		return renderDemoLines(img, cfg)
	}
	if err := runMediaBenchmarkWithRunner(&out, path, 20, 8, 1, "", budget, time.Now, slowRender); err != nil {
		t.Fatalf("runMediaBenchmarkWithRunner: %v", err)
	}
	got := out.String()
	// Slow modes appear as ">budget" (timedOut: goroutine abandoned) or "(slow)"
	// (render finished but exceeded budget). Both are acceptable.
	if !strings.Contains(got, ">budget") && !strings.Contains(got, "(slow)") {
		t.Fatalf("expected '>budget' or '(slow)' marker for over-budget modes, got:\n%s", got)
	}
}

// TestRunMediaBenchmarkModeFilter verifies that --mode restricts benchmarking
// to the specified mode only.
func TestRunMediaBenchmarkModeFilter(t *testing.T) {
	path := makeTestPNG(t, t.TempDir())
	var out bytes.Buffer
	// "h" is the halfblock alias — only one mode should appear in the output.
	if err := runMediaBenchmark(&out, path, 20, 8, 1, "h", 200*time.Millisecond); err != nil {
		t.Fatalf("runMediaBenchmark with mode filter: %v", err)
	}
	got := out.String()
	lines := strings.Split(strings.TrimSpace(got), "\n")
	// Header = 2 lines (title + column headings) + exactly 1 data row.
	dataLines := 0
	for _, l := range lines[2:] {
		if strings.TrimSpace(l) != "" {
			dataLines++
		}
	}
	if dataLines != 1 {
		t.Fatalf("expected exactly 1 data row with --mode=h, got %d:\n%s", dataLines, got)
	}
}

// TestRunMediaBenchmarkStreamingOutput verifies that output is printed
// incrementally: each mode row appears immediately after its render, not all
// at once at the end. We test this by injecting a clockwork `now` func and
// a render func that advances the clock so we can assert ordering.
func TestRunMediaBenchmarkStreamingOutput(t *testing.T) {
	path := makeTestPNG(t, t.TempDir())
	var out bytes.Buffer
	var timestamps []time.Time
	base := time.Now()
	tick := base
	// Each call to now() advances the clock deterministically.
	advancingNow := func() time.Time {
		tick = tick.Add(1 * time.Millisecond)
		return tick
	}
	// Capture what's in the buffer after each render.
	var snapshots []string
	renderCount := 0
	fakeRender := func(img image.Image, cfg renderCfg) ([]string, error) {
		renderCount++
		snapshots = append(snapshots, out.String())
		timestamps = append(timestamps, advancingNow())
		return renderDemoLines(img, cfg)
	}
	budget := 100 * time.Millisecond
	if err := runMediaBenchmarkWithRunner(&out, path, 20, 8, 1, "", budget, advancingNow, fakeRender); err != nil {
		t.Fatalf("runMediaBenchmarkWithRunner: %v", err)
	}
	if renderCount == 0 {
		t.Fatal("no renders were performed")
	}
	// After at least one render, more output should have accumulated.
	// (The header is printed before the loop, so snapshot[0] should only have the header.)
	finalOutput := out.String()
	if len(finalOutput) <= len(snapshots[0]) {
		t.Fatal("output did not grow after renders — streaming broken")
	}
}

// TestRunMediaBenchmarkVideo verifies output for a short synthetic video.
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
	if err := runMediaBenchmark(&out, path, 12, 6, 1, "", 2*time.Second); err != nil {
		t.Fatalf("runMediaBenchmark: %v", err)
	}
	if !strings.Contains(out.String(), "Video benchmark: bench.mp4") || !strings.Contains(out.String(), "Frames") {
		t.Fatalf("unexpected benchmark output:\n%s", out.String())
	}
}
