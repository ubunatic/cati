package cmd

import (
	"fmt"
	"image"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"golang.org/x/term"
)

// play renders frames in a loop at the given fps until the user presses 'q'
// or a signal (SIGINT/SIGTERM) is received.
// Frames are pre-loaded and scaled once before the animation starts.
func play(paths []string, fps int) error {
	if len(paths) == 0 {
		return fmt.Errorf("no images to play")
	}
	if fps <= 0 {
		return fmt.Errorf("--fps must be > 0")
	}

	cols := halfblock.TermWidth()

	// ── Pre-load & scale all frames ───────────────────────────────────────────
	frames := make([]image.Image, 0, len(paths))
	for _, p := range paths {
		img, err := halfblock.LoadImage(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if cols > 0 {
			img = halfblock.Scale(img, cols)
		}
		frames = append(frames, img)
	}

	// ── Raw terminal mode (so 'q' is read without Enter) ──────────────────────
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// stdin is not a tty (e.g. in tests) — skip raw mode; q won't work.
		oldState = nil
	}
	restore := func() {
		if oldState != nil {
			_ = term.Restore(fd, oldState)
		}
	}
	defer restore()

	// ── Signal handling ───────────────────────────────────────────────────────
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	// ── Keypress reader ───────────────────────────────────────────────────────
	quit := make(chan struct{}, 1)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			// 'q', 'Q', ESC, or Ctrl+C all quit.
			if buf[0] == 'q' || buf[0] == 'Q' || buf[0] == 27 || buf[0] == 3 {
				quit <- struct{}{}
				return
			}
		}
	}()

	// ── Enter animation mode ──────────────────────────────────────────────────
	halfblock.HideCursor(os.Stdout)
	halfblock.ClearScreen(os.Stdout)
	defer func() {
		halfblock.EraseDown(os.Stdout)
		halfblock.ShowCursor(os.Stdout)
		fmt.Fprintln(os.Stdout)
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
			// Home cursor before render; ansiEraseLine inside Render clears
			// each line so previous-frame pixels don't bleed through.
			halfblock.CursorHome(os.Stdout)
			if err := halfblock.Render(os.Stdout, frames[i]); err != nil {
				return err
			}
			halfblock.EraseDown(os.Stdout) // clear below if frame shrank
			i = (i + 1) % len(frames)
		}
	}
}
