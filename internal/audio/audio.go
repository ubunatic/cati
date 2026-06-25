package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
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

// Player manages an audio playback session backed by ffplay.
// ffplay handles its own audio device routing without needing an intermediate
// PCM pipe.  Call Stop to terminate early; Done is closed when playback ends.
type Player struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

// Open starts audio playback for path in the background using ffplay.
// Cancelling ctx stops playback.  Returns an error immediately if ffplay
// fails to start.
func Open(ctx context.Context, path string) (*Player, error) {
	if err := checkDeps(); err != nil {
		return nil, err
	}

	// ffplay plays audio directly; -nodisp suppresses the video window,
	// -vn drops video decoding entirely, -autoexit quits when the stream ends.
	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, "ffplay",
		"-v", "quiet",
		"-nodisp",
		"-vn",
		"-autoexit",
		path)
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("audio: ffplay: %w", err)
	}

	p := &Player{
		cmd:  cmd,
		done: make(chan struct{}),
	}

	go func() {
		defer close(p.done)
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			msg := strings.TrimSpace(stderr.String())
			if msg != "" {
				p.err = fmt.Errorf("audio: ffplay: %w: %s", err, msg)
			} else {
				p.err = fmt.Errorf("audio: ffplay: %w", err)
			}
		}
	}()

	return p, nil
}

// Stop terminates playback immediately.
func (p *Player) Stop() {
	_ = p.cmd.Process.Kill()
	<-p.done
}

// Pause suspends playback by sending SIGSTOP to the ffplay process.
func (p *Player) Pause() {
	if p == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(syscall.SIGSTOP)
}

// Resume resumes a suspended player by sending SIGCONT.
func (p *Player) Resume() {
	if p == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(syscall.SIGCONT)
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
	for _, bin := range []string{"ffprobe", "ffplay"} {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("audio: missing required binaries: %s", strings.Join(missing, ", "))
	}
	return nil
}
