// Command imgbrowser demonstrates cati rendering inside a loom split-pane
// TUI: a file list on the left (borrowed from loom's examples/filebrowser)
// and a live cati image preview on the right.
package main

import (
	"flag"
	"fmt"
	"os"

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
	return pane.Run(app)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
