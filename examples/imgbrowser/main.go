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
)

func run() error {
	flag.Parse()
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	app, err := newBrowser(dir)
	if err != nil {
		return err
	}
	pane, err := loom.New(20)
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
