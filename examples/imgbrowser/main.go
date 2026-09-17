// Command imgbrowser demonstrates cati rendering inside a loom split-pane
// TUI: a file list on the left (borrowed from loom's examples/filebrowser)
// and a live cati image preview on the right.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"codeberg.org/ubunatic/loom"

	catiterm "ubunatic.com/cati/v1/term"
)

func run() error {
	flag.Parse()
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
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
	boxH := termH - 2
	if boxH < 10 {
		boxH = 10
	}
	app, err := newBrowser(dir, boxH)
	if err != nil {
		return err
	}
	pane, err := loom.New(termH)
	if err != nil {
		return err
	}
	defer pane.Close()
	pane.Resizeable = true
	pane.MaxCols = 0
	pane.DisableDefaultQuit = true
	pane.EnableMouseClicks()

	// RunWatch drives redraws at a steady cadence (50ms) so background async
	// renders immediately display when complete without waiting for key events.
	cadence := loom.Cadence{
		Collect: 50 * time.Millisecond,
		Redraw:  50 * time.Millisecond,
	}
	return pane.RunWatch(context.Background(), app, cadence, func(time.Time) error {
		return nil
	})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
