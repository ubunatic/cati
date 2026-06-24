package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Info describes the audio stream found in a media file.
type Info struct {
	Codec      string
	SampleRate int
	Channels   int
}

// ffprobeStreams is the minimal shape of ffprobe -of json -show_entries output.
type ffprobeStreams struct {
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		SampleRate string `json:"sample_rate"`
		Channels   int    `json:"channels"`
	} `json:"streams"`
}

// Probe returns the audio stream info for path, or nil if no audio stream is
// present.  An error is returned only when ffprobe is unavailable or the file
// cannot be read.
func Probe(path string) (*Info, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_type,codec_name,sample_rate,channels",
		"-of", "json",
		path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	var result ffprobeStreams
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("ffprobe: parse error: %w", err)
	}
	for _, s := range result.Streams {
		if s.CodecType != "audio" {
			continue
		}
		sr, _ := strconv.Atoi(s.SampleRate)
		return &Info{
			Codec:      s.CodecName,
			SampleRate: sr,
			Channels:   s.Channels,
		}, nil
	}
	return nil, nil // no audio stream
}

// HasAudio returns true when path contains at least one audio stream.
func HasAudio(path string) (bool, error) {
	info, err := Probe(path)
	if err != nil {
		return false, err
	}
	return info != nil, nil
}

// Player manages an audio playback session backed by ffmpeg → aplay.
// Audio is decoded by ffmpeg as signed 16-bit LE PCM and written to aplay's
// stdin.  Call Stop to terminate early; Done is closed when playback ends.
type Player struct {
	ffmpeg *exec.Cmd
	aplay  *exec.Cmd
	done   chan struct{}
	err    error
}

// PCMRate and PCMChannels are the fixed PCM parameters used between ffmpeg and
// aplay.  Changing them requires matching updates in both commands.
const (
	PCMRate     = 44100
	PCMChannels = 2
)

// Open starts audio playback for path in the background.  The player reads
// audio from path via ffmpeg and writes raw PCM to aplay.  Cancelling ctx
// stops playback.  Returns an error immediately if either process fails to
// start.
func Open(ctx context.Context, path string) (*Player, error) {
	if err := checkDeps(); err != nil {
		return nil, err
	}

	// ffmpeg decodes audio to raw signed 16-bit LE stereo PCM on stdout.
	ffmpegCmd := exec.CommandContext(ctx, "ffmpeg",
		"-v", "quiet",
		"-i", path,
		"-vn", // drop video
		"-f", "s16le",
		"-ar", strconv.Itoa(PCMRate),
		"-ac", strconv.Itoa(PCMChannels),
		"pipe:1")

	// aplay reads raw PCM from stdin.
	aplayCmd := exec.CommandContext(ctx, "aplay",
		"-q",
		"-f", "S16_LE",
		"-r", strconv.Itoa(PCMRate),
		"-c", strconv.Itoa(PCMChannels))

	// Wire ffmpeg stdout → aplay stdin.
	pipe, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("audio: pipe: %w", err)
	}
	aplayCmd.Stdin = pipe

	if err := ffmpegCmd.Start(); err != nil {
		return nil, fmt.Errorf("audio: ffmpeg: %w", err)
	}
	if err := aplayCmd.Start(); err != nil {
		_ = ffmpegCmd.Process.Kill()
		return nil, fmt.Errorf("audio: aplay: %w", err)
	}

	p := &Player{
		ffmpeg: ffmpegCmd,
		aplay:  aplayCmd,
		done:   make(chan struct{}),
	}

	go func() {
		defer close(p.done)
		// Wait for ffmpeg to finish (EOF or killed); aplay drains then exits.
		if err := ffmpegCmd.Wait(); err != nil && ctx.Err() == nil {
			p.err = fmt.Errorf("audio: ffmpeg: %w", err)
		}
		_ = aplayCmd.Wait()
	}()

	return p, nil
}

// Stop terminates playback immediately.
func (p *Player) Stop() {
	_ = p.ffmpeg.Process.Kill()
	_ = p.aplay.Process.Kill()
	<-p.done
}

// Done returns a channel that is closed when playback finishes naturally or
// after Stop is called.
func (p *Player) Done() <-chan struct{} { return p.done }

// Err returns any error encountered during playback.  Only valid after Done is
// closed.
func (p *Player) Err() error { return p.err }

// ── dependency check ──────────────────────────────────────────────────────────

// checkDeps returns an error listing any missing binaries.
func checkDeps() error {
	var missing []string
	for _, bin := range []string{"ffmpeg", "aplay"} {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("audio: missing required binaries: %s", strings.Join(missing, ", "))
	}
	return nil
}
