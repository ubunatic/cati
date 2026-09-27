package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/ubunatic/loom"
	"codeberg.org/ubunatic/loom/media"

	catiterm "ubunatic.com/cati/v1/term"
)

type options struct {
	themeName  string
	modeStr    string
	fps        float64
	imagesOnly bool
}

func runBrowser(dir string, opts options) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", absDir)
	}

	mode := media.ModeHalfblock
	switch strings.ToLower(opts.modeStr) {
	case "half", "halfblock":
		mode = media.ModeHalfblock
	case "quad", "quadblock":
		mode = media.ModeQuadblock
	case "six", "sextant":
		mode = media.ModeSextant
	}

	termH := 32
	if th := catiterm.TermHeight(); th > 12 {
		if th > 36 {
			termH = th - 4
		} else if th > 24 {
			termH = th - 2
		} else {
			termH = th
		}
	}

	app, err := newApp(absDir, opts.themeName, mode, opts.fps, opts.imagesOnly, termH-2)
	if err != nil {
		return err
	}
	defer app.Close()

	pane, err := loom.New(termH)
	if err != nil {
		return err
	}
	defer pane.Close()

	pane.Resizeable = true
	pane.MaxCols = 0
	pane.DisableDefaultQuit = true
	pane.EnableMouseClicks()

	cadence := loom.Cadence{
		Collect: 40 * time.Millisecond,
		Redraw:  40 * time.Millisecond,
	}

	return pane.RunWatch(context.Background(), app, cadence, func(time.Time) error {
		return nil
	})
}
