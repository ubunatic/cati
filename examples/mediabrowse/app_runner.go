package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ubunatic.com/loom"
	"ubunatic.com/loom/media"

	catiterm "ubunatic.com/cati/v1/term"
)

type options struct {
	themeName  string
	modeStr    string
	fps        float64
	imagesOnly bool
}

func runBrowser(targetPath string, opts options) error {
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	dir := absPath
	selectedFile := ""
	if !info.IsDir() {
		dir = filepath.Dir(absPath)
		selectedFile = filepath.Base(absPath)
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

	app, err := newApp(dir, opts.themeName, mode, opts.fps, opts.imagesOnly, termH-2, selectedFile)
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

	return pane.RunWatch(context.Background(), app, cadence, func(now time.Time) error {
		app.Tick(now)
		return nil
	})
}
