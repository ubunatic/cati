package halfblock

import (
	"bytes"
	"context"
	"encoding/json"
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

// ── ffprobe JSON metadata ─────────────────────────────────────────────────────

type ffprobeOut struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	FrameRate string `json:"r_frame_rate"`
}

type ffprobeFormat struct {
	Duration   string            `json:"duration"`
	Size       string            `json:"size"`
	BitRate    string            `json:"bit_rate"`
	FormatName string            `json:"format_name"`
	Tags       map[string]string `json:"tags"`
}

// ProbeResult holds parsed ffprobe output for a media file.
type ProbeResult struct {
	Width     int
	Height    int
	FPS       float64
	VCodec    string
	ACodec    string
	Duration  float64 // seconds
	FileSize  int64
	Bitrate   int64  // bits/s
	Container string // first format_name token
	Title     string
	Artist    string
	Comment   string
	Date      string // creation_time tag or DATE
	Location  string // GPS string from tags
	Camera    string // device model tag
}

// ProbeMediaMeta runs ffprobe once with JSON output and returns all available
// metadata for path. Returns a zero-value ProbeResult on any error.
func ProbeMediaMeta(path string) ProbeResult {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path)
	out, err := cmd.Output()
	if err != nil {
		return ProbeResult{}
	}
	var raw ffprobeOut
	if err := json.Unmarshal(out, &raw); err != nil {
		return ProbeResult{}
	}

	var r ProbeResult

	for _, s := range raw.Streams {
		switch s.CodecType {
		case "video":
			if r.VCodec == "" {
				r.VCodec = s.CodecName
				r.Width = s.Width
				r.Height = s.Height
				if parts := strings.SplitN(s.FrameRate, "/", 2); len(parts) == 2 {
					num, _ := strconv.ParseFloat(parts[0], 64)
					den, _ := strconv.ParseFloat(parts[1], 64)
					if den != 0 {
						r.FPS = num / den
					}
				}
			}
		case "audio":
			if r.ACodec == "" {
				r.ACodec = s.CodecName
			}
		}
	}

	f := raw.Format
	if v, _ := strconv.ParseFloat(f.Duration, 64); v > 0 {
		r.Duration = v
	}
	if v, _ := strconv.ParseInt(f.Size, 10, 64); v > 0 {
		r.FileSize = v
	}
	if v, _ := strconv.ParseInt(f.BitRate, 10, 64); v > 0 {
		r.Bitrate = v
	}
	if f.FormatName != "" {
		r.Container = strings.SplitN(f.FormatName, ",", 2)[0]
	}

	tag := func(keys ...string) string {
		for _, k := range keys {
			if v := f.Tags[k]; v != "" {
				return v
			}
		}
		return ""
	}
	r.Title = tag("title", "TITLE")
	r.Artist = tag("artist", "ARTIST", "author", "AUTHOR")
	r.Comment = tag("comment", "COMMENT")
	r.Date = tag("creation_time", "date", "DATE")
	r.Location = tag("location", "com.apple.quicktime.location.ISO6709", "LOCATION")
	r.Camera = tag("com.apple.quicktime.model", "model", "make", "MAKE")

	return r
}
