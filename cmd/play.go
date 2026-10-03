package cmd

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"
	"ubunatic.com/cati/internal/viewgeom"
	"ubunatic.com/cati/v1/halfblock"

	catiterm "ubunatic.com/cati/v1/term"
)

// play is the entry point for --play mode.
// It dispatches to playPreview, playImages (pre-load loop), or playVideos (streaming)
// depending on playMode and whether any path is a video file.
// width and height are in terminal characters (0 = auto-detect from terminal).
func play(paths []string, fps, width, height int, rc renderCfg, tr TimeRange, crop cropSpec, aspect, pad string, playMode string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no images to play")
	}

	if playMode == "preview" {
		return playPreview(paths[0], width, height, rc, tr, crop, aspect, pad)
	}

	for _, p := range paths {
		if halfblock.IsVideo(p) {
			return playVideos(paths, fps, width, height, rc, tr, crop, aspect, pad, playMode)
		}
	}
	return playImages(paths, fps, width, height, rc, tr, crop, aspect, pad, playMode)
}

// playPreview renders a single frame for preview mode and exits.
func playPreview(path string, width, height int, rc renderCfg, tr TimeRange, crop cropSpec, aspect, pad string) error {
	termCols, termRows := width, height
	if termCols == 0 && termRows == 0 {
		termCols, termRows = catiterm.TermWidth(), catiterm.TermHeight()
	}
	autoCropCols, autoCropRows := catiterm.TermWidth(), catiterm.TermHeight()

	var img image.Image
	var err error
	if halfblock.IsVideo(path) {
		img, err = halfblock.LoadVideoFrameAt(path, tr.Start)
	} else {
		img, err = halfblock.LoadImage(path)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if pad != "" {
		padCols, padRows, err := parsePadSpec(pad)
		if err != nil {
			return err
		}
		img = padSourceImage(img, padCols, padRows)
	}

	constraints := viewgeom.TargetConstraints{
		ExplicitCols: width,
		ExplicitRows: height,
		TermCols:     termCols,
		TermRows:     termRows,
		AspectMode:   aspect,
	}
	img, err = prepareRenderPlanImage(img, constraints, rc)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	img = applyCellCrop(img, rc, crop, autoCropCols, autoCropRows)
	return renderChecked(os.Stdout, img, rc)
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
func playImages(paths []string, fps, width, height int, rc renderCfg, tr TimeRange, crop cropSpec, aspect, pad string, playMode string) error {
	if fps <= 0 {
		fps = 15
	}

	termCols, termRows := width, height
	if termCols == 0 && termRows == 0 {
		termCols, termRows = catiterm.TermWidth(), catiterm.TermHeight()
	}
	autoCropCols, autoCropRows := catiterm.TermWidth(), catiterm.TermHeight()

	// Apply time range: convert seconds → frame indices.
	// For image sequences the frame rate defines the mapping.
	startFrame := 0
	if tr.Start > 0 {
		startFrame = int(tr.Start * float64(fps))
	}
	endFrame := len(paths) // exclusive; 0 means open-ended
	if tr.End > 0 {
		ef := int(tr.End * float64(fps))
		if ef < endFrame {
			endFrame = ef
		}
	}
	if startFrame >= len(paths) {
		return fmt.Errorf("--range start (%.3fs) is beyond the last frame", tr.Start)
	}

	// Pre-load & scale the selected frame window.
	frames := make([]image.Image, 0, endFrame-startFrame)
	for _, p := range paths[startFrame:endFrame] {
		img, err := halfblock.LoadImage(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if pad != "" {
			padCols, padRows, err := parsePadSpec(pad)
			if err != nil {
				return err
			}
			img = padSourceImage(img, padCols, padRows)
		}
		constraints := viewgeom.TargetConstraints{
			ExplicitCols: width,
			ExplicitRows: height,
			TermCols:     termCols,
			TermRows:     termRows,
			AspectMode:   aspect,
		}
		img, err = prepareRenderPlanImage(img, constraints, rc)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		img = applyCellCrop(img, rc, crop, autoCropCols, autoCropRows)
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

	// checkGate throttles the ANSI invariant check to once per second.
	// For image sequences all frames share the same dimensions so the check
	// is redundant after the first frame passes.
	checkGate := &renderCheckGate{interval: time.Second}

	i := 0
	for {
		select {
		case <-quit:
			return nil
		case <-sigs:
			return nil
		case <-ticker.C:
			halfblock.CursorHome(os.Stdout)
			if err := renderCheckedGated(os.Stdout, frames[i], rc, checkGate); err != nil {
				return err
			}
			halfblock.EraseDown(os.Stdout)
			i++
			if i >= len(frames) {
				if playMode == "once" {
					return nil
				}
				i = 0
			}
		}
	}
}

// ── video streaming mode ──────────────────────────────────────────────────────

// playVideos streams one or more video files sequentially, playing each once or repeating.
// All paths must be video files.
func playVideos(paths []string, fps, width, height int, rc renderCfg, tr TimeRange, crop cropSpec, aspect, pad string, playMode string) error {
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

	termCols, termRows := width, height
	if termCols == 0 && termRows == 0 {
		termCols, termRows = catiterm.TermWidth(), catiterm.TermHeight()
	}
	autoCropCols, autoCropRows := catiterm.TermWidth(), catiterm.TermHeight()

	if rc.useGlyphs() {
		w, h := rc.renderCellSize()
		if w*h > 128 {
			fmt.Fprintf(os.Stderr, "cati: warning: mode %q is not yet optimized (geometry %dx%d > 128px); realtime playback may drop frames\n", rc.name, w, h)
		}
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

	// checkGate throttles the ANSI and aspect invariant checks to once per
	// second while frame dimensions are stable. This eliminates O(output-len)
	// validation work from the hot ticker path.
	checkGate := &renderCheckGate{interval: time.Second}

	// index into paths; restartStream opens a fresh stream for paths[videoIdx].
	videoIdx := 0
	frames, cleanup, err := halfblock.OpenVideoStream(ctx, paths[videoIdx], displayFPS, tr.Start, tr.End)
	if err != nil {
		return fmt.Errorf("open video stream: %w", err)
	}
	defer cleanup()

	audioPlayer := openAudio(ctx, paths[videoIdx], tr.Start, tr.End)
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
						if playMode == "repeat" {
							videoIdx = 0
							failedVideos = 0
						} else {
							return nil // played every video once
						}
					}
					if failedVideos >= len(paths) {
						return fmt.Errorf("failed to decode any frames from video stream(s)")
					}
					currentVideoHadFrames = false
					// Subsequent videos in a playlist play without range restriction.
					frames, cleanup, err = halfblock.OpenVideoStream(ctx, paths[videoIdx], displayFPS, 0, 0)
					if err != nil {
						return fmt.Errorf("open video stream: %w", err)
					}
					audioPlayer = openAudio(ctx, paths[videoIdx], 0, 0)
					continue
				}
				currentVideoHadFrames = true
				if pad != "" {
					padCols, padRows, err := parsePadSpec(pad)
					if err != nil {
						return err
					}
					img = padSourceImage(img, padCols, padRows)
				}
				constraints := viewgeom.TargetConstraints{
					ExplicitCols: width,
					ExplicitRows: height,
					TermCols:     termCols,
					TermRows:     termRows,
					AspectMode:   aspect,
				}
				img, err = prepareRenderPlanImage(img, constraints, rc)
				if err != nil {
					return err
				}
				img = applyCellCrop(img, rc, crop, autoCropCols, autoCropRows)
				lastFrame = img
			default:
				// No frame ready — keep showing the current frame.
			}

			if lastFrame == nil {
				continue
			}
			halfblock.CursorHome(os.Stdout)
			if err := renderCheckedGated(os.Stdout, lastFrame, rc, checkGate); err != nil {
				return err
			}
			halfblock.EraseDown(os.Stdout)
		}
	}
}
