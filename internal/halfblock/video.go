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

// ── streaming ─────────────────────────────────────────────────────────────────

// OpenVideoStream starts an ffmpeg process that decodes path and sends frames
// on the returned channel.  The channel is closed when the video ends, the
// context is cancelled, or a decode error occurs.
//
// The caller must invoke the returned cleanup function to release resources
// (safe to call more than once).
//
// It requires ffmpeg to be on $PATH.
func OpenVideoStream(ctx context.Context, path string) (<-chan image.Image, func(), error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-v", "quiet",
		"-i", path,
		"-f", "image2pipe",
		"-vcodec", "png",
		"pipe:1")

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

	ch := make(chan image.Image, 8) // buffer a handful of frames
	go func() {
		defer close(ch)
		defer close(done)
		defer cmd.Wait() //nolint:errcheck
		r := io.Reader(stdout)
		for {
			img, err := png.Decode(r)
			if err != nil {
				return // EOF or decode error — stop streaming
			}
			select {
			case ch <- img:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, cleanup, nil
}
