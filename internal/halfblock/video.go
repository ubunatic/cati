package halfblock

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// VideoExts is the set of file extensions cati recognises as video files.
var VideoExts = map[string]bool{
	".mp4":  true,
	".webm": true,
	".mkv":  true,
	".mov":  true,
	".avi":  true,
}

// IsVideo reports whether path has a recognised video file extension.
func IsVideo(path string) bool {
	return VideoExts[strings.ToLower(filepath.Ext(path))]
}

// ── ffprobe ───────────────────────────────────────────────────────────────────

// ProbeVideoDimensions returns the pixel width and height of the first video
// stream in path.  It requires ffprobe to be on $PATH.
func ProbeVideoDimensions(path string) (width, height int, err error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0",
		path)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	// Output: "width,height\n"  e.g. "1920,1080\n"
	s := strings.TrimSpace(string(out))
	parts := strings.SplitN(s, ",", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("ffprobe: unexpected dimensions %q", s)
	}
	w, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe: parse width %q: %w", parts[0], err)
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe: parse height %q: %w", parts[1], err)
	}
	return w, h, nil
}

// ProbeVideoFPS returns the native frame rate of the first video stream.
// It requires ffprobe to be on $PATH.
func ProbeVideoFPS(path string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "v:0",
		"-show_entries", "stream=r_frame_rate",
		"-of", "csv=p=0",
		path)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	// Output format: "num/den\n" (e.g. "30/1" or "25000/1001").
	s := strings.TrimSpace(string(out))
	parts := strings.SplitN(s, "/", 2)
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || num == 0 {
		return 0, fmt.Errorf("ffprobe: unexpected fps %q", s)
	}
	if len(parts) == 1 {
		return num, nil
	}
	den, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || den == 0 {
		return 0, fmt.Errorf("ffprobe: unexpected fps denominator %q", s)
	}
	return num / den, nil
}

// ProbeVideoDuration returns the total duration of the video in seconds.
// It requires ffprobe to be on $PATH.
func ProbeVideoDuration(path string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		path)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	s := strings.TrimSpace(string(out))
	dur, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("ffprobe: unexpected duration %q", s)
	}
	return dur, nil
}

// ── single-frame load ─────────────────────────────────────────────────────────

// LoadVideoFrame extracts the first frame of path as an image using ffmpeg.
// It requires ffmpeg to be on $PATH.
func LoadVideoFrame(path string) (image.Image, error) {
	cmd := exec.Command("ffmpeg",
		"-v", "quiet",
		"-i", path,
		"-vframes", "1",
		"-f", "image2pipe",
		"-vcodec", "png",
		"pipe:1")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg %s: %w", path, err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("decode frame from %s: %w", path, err)
	}
	return img, nil
}

// LoadVideoFrameAt extracts one frame at offsetSec seconds from path.
// It requires ffmpeg to be on $PATH.
func LoadVideoFrameAt(path string, offsetSec float64) (image.Image, error) {
	cmd := exec.Command("ffmpeg",
		"-v", "quiet",
		"-ss", strconv.FormatFloat(offsetSec, 'f', 3, 64),
		"-i", path,
		"-vframes", "1",
		"-f", "image2pipe",
		"-vcodec", "png",
		"pipe:1")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg %s@%.3fs: %w", path, offsetSec, err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("decode frame from %s: %w", path, err)
	}
	return img, nil
}

// ── streaming ─────────────────────────────────────────────────────────────────

// OpenVideoStream starts an ffmpeg process that decodes path and sends frames
// on the returned channel.  The channel is closed when the video ends, the
// context is cancelled, or a decode error occurs.
//
// displayFPS controls the output frame rate passed to ffmpeg via -vf fps=N.
// ffmpeg drops or duplicates source frames to hit the target rate while
// preserving natural wallclock speed — a 30 fps source at displayFPS=15
// outputs every 2nd frame but plays in the same real time.
// Pass displayFPS ≤ 0 to disable rate limiting (decodes as fast as possible).
//
// The caller must invoke the returned cleanup function to release resources
// (safe to call more than once).
//
// It requires ffmpeg to be on $PATH.
func OpenVideoStream(ctx context.Context, path string, displayFPS float64) (<-chan image.Image, func(), error) {
	// Probe source dimensions so we can read fixed-size raw frames.
	w, h, err := ProbeVideoDimensions(path)
	if err != nil {
		return nil, nil, fmt.Errorf("probe video dimensions: %w", err)
	}

	args := []string{"-v", "quiet", "-i", path}
	if displayFPS > 0 {
		args = append(args, "-vf", fmt.Sprintf("fps=%.6g", displayFPS))
		args = append(args, "-threads", "4")
	}
	// Raw RGBA output: no PNG encode/decode overhead.  Frame size = w*h*4 bytes.
	args = append(args, "-f", "rawvideo", "-pix_fmt", "rgba", "pipe:1")
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("ffmpeg: %w", err)
	}

	// cleanup kills ffmpeg and waits for it to exit.
	done := make(chan struct{})
	cleanup := func() {
		select {
		case <-done:
			return // already finished
		default:
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}

	ch := make(chan image.Image, 8)
	frameSize := w * h * 4
	go func() {
		defer close(ch)
		defer close(done)
		defer cmd.Wait() //nolint:errcheck
		buf := make([]byte, frameSize)
		for {
			if _, err := io.ReadFull(stdout, buf); err != nil {
				return // EOF or partial read — video ended or ffmpeg exited
			}
			img := image.NewRGBA(image.Rect(0, 0, w, h))
			copy(img.Pix, buf)
			select {
			case ch <- img:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, cleanup, nil
}
