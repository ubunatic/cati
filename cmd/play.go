package cmd

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"golang.org/x/term"
)

// play is the entry point for --play mode.
// It dispatches to playImages (pre-load loop) or playVideos (streaming)
// depending on whether any path is a video file.
// width and height are in terminal characters (0 = auto-detect from terminal).
func play(paths []string, fps, width, height int, rc renderCfg) error {
	if len(paths) == 0 {
		return fmt.Errorf("no images to play")
	}

	for _, p := range paths {
		if halfblock.IsVideo(p) {
			return playVideos(paths, fps, width, height, rc)
		}
	}
	return playImages(paths, fps, width, height, rc)
}

// ── shared terminal setup ─────────────────────────────────────────────────────

// playTerminal sets up raw mode, signals, and the quit channel.
// Returns a restore function, a signal channel, and a quit channel.
// The caller must defer restore().
func playTerminal() (restore func(), sigs chan os.Signal, quit chan struct{}) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		oldState = nil
	}
	restore = func() {
		if oldState != nil {
			_ = term.Restore(fd, oldState)
		}
	}

	sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	quit = make(chan struct{}, 1)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			if buf[0] == 'q' || buf[0] == 'Q' || buf[0] == 27 || buf[0] == 3 {
				quit <- struct{}{}
				return
			}
		}
	}()

	halfblock.HideCursor(os.Stdout)
	halfblock.ClearScreen(os.Stdout)
	return
}

// ── image sequence mode ───────────────────────────────────────────────────────

// playImages pre-loads all frames and loops them at fps.
func playImages(paths []string, fps, width, height int, rc renderCfg) error {
	if fps <= 0 {
		fps = 15
	}

	cols, rows := width, height
	if cols == 0 && rows == 0 {
		cols, rows = halfblock.TermWidth(), halfblock.TermHeight()
	}

	// Pre-load & scale all frames.
	frames := make([]image.Image, 0, len(paths))
	for _, p := range paths {
		img, err := halfblock.LoadImage(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		img = prepareRenderedImage(img, nil, cols, rows, rc, "")
		frames = append(frames, img)
	}

	restore, sigs, quit := playTerminal()
	defer restore()
	defer signal.Stop(sigs)
	defer func() {
		halfblock.EraseDown(os.Stdout)
		halfblock.ShowCursor(os.Stdout)
		fmt.Fprint(os.Stdout, "\r\n")
	}()

	ticker := time.NewTicker(time.Duration(float64(time.Second) / float64(fps)))
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-quit:
			return nil
		case <-sigs:
			return nil
		case <-ticker.C:
			halfblock.CursorHome(os.Stdout)
			if err := rc.render(os.Stdout, frames[i]); err != nil {
				return err
			}
			halfblock.EraseDown(os.Stdout)
			i = (i + 1) % len(frames)
		}
	}
}

// ── video streaming mode ──────────────────────────────────────────────────────

// playVideos streams one or more video files sequentially, playing each once.
// All paths must be video files.
func playVideos(paths []string, fps, width, height int, rc renderCfg) error {
	// Validate: all paths must be video files.
	for _, p := range paths {
		if !halfblock.IsVideo(p) {
			return fmt.Errorf("%s: cannot mix image and video files in --play mode", p)
		}
	}

	// Resolve display fps: probe native fps from the first video if not set.
	displayFPS := float64(fps)
	if displayFPS <= 0 {
		native, err := halfblock.ProbeVideoFPS(paths[0])
		if err != nil {
			// ffprobe not available or failed; fall back to 15.
			native = 15
		}
		displayFPS = native
	}
	if displayFPS <= 0 {
		displayFPS = 15
	}

	cols, rows := width, height
	if cols == 0 && rows == 0 {
		cols, rows = halfblock.TermWidth(), halfblock.TermHeight()
	}

	restore, sigs, quit := playTerminal()
	defer restore()
	defer signal.Stop(sigs)
	defer func() {
		halfblock.EraseDown(os.Stdout)
		halfblock.ShowCursor(os.Stdout)
		fmt.Fprint(os.Stdout, "\r\n")
	}()

	interval := time.Duration(float64(time.Second) / displayFPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// index into paths; restartStream opens a fresh stream for paths[videoIdx].
	videoIdx := 0
	frames, cleanup, err := halfblock.OpenVideoStream(ctx, paths[videoIdx], displayFPS)
	if err != nil {
		return fmt.Errorf("open video stream: %w", err)
	}
	defer cleanup()

	audioPlayer := openAudio(ctx, paths[videoIdx])
	defer stopAudio(audioPlayer)

	var lastFrame image.Image
	failedVideos := 0
	currentVideoHadFrames := false

	for {
		select {
		case <-quit:
			return nil
		case <-sigs:
			return nil

		case <-ticker.C:
			// Pull exactly one frame per tick so playback advances at displayFPS.
			// A non-blocking inner select avoids stalling the ticker when ffmpeg
			// hasn't produced a frame yet.
			select {
			case img, ok := <-frames:
				if !ok {
					// This video ended.
					cleanup()
					stopAudio(audioPlayer)
					if !currentVideoHadFrames {
						failedVideos++
					}
					videoIdx++
					if videoIdx >= len(paths) {
						return nil // played every video once
					}
					if failedVideos >= len(paths) {
						return fmt.Errorf("failed to decode any frames from video stream(s)")
					}
					currentVideoHadFrames = false
					frames, cleanup, err = halfblock.OpenVideoStream(ctx, paths[videoIdx], displayFPS)
					if err != nil {
						return fmt.Errorf("open video stream: %w", err)
					}
					audioPlayer = openAudio(ctx, paths[videoIdx])
					continue
				}
				currentVideoHadFrames = true
				img = prepareRenderedImage(img, nil, cols, rows, rc, "")
				lastFrame = img
			default:
				// No frame ready — keep showing the current frame.
			}

			if lastFrame == nil {
				continue
			}
			halfblock.CursorHome(os.Stdout)
			if err := rc.render(os.Stdout, lastFrame); err != nil {
				return err
			}
			halfblock.EraseDown(os.Stdout)
		}
	}
}
