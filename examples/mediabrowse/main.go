// Package main provides the entrypoint for the mediabrowse example CLI.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	flagTheme      string
	flagMode       string
	flagFPS        float64
	flagImagesOnly bool
)

func newRootCmd() *cobra.Command {
	return newRootCmdWithRunner(runBrowser)
}

func newRootCmdWithRunner(run func(string, options) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mediabrowse [path]",
		Short: "Explore directories and preview images/videos using loom & cati",
		Long: `mediabrowse is a split-pane terminal file and media viewer.
It integrates Loom's Frame/Box layout and navigation with Cati's halfblock,
quadblock, and sextant renderers and video frame streaming.

When given a file path, mediabrowse starts in that file's directory and
immediately opens and selects the file in the preview pane.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			return run(dir, options{
				themeName:  flagTheme,
				modeStr:    flagMode,
				fps:        flagFPS,
				imagesOnly: flagImagesOnly,
			})
		},
	}

	cmd.Flags().StringVarP(&flagTheme, "theme", "t", "mc", fmt.Sprintf("color theme (%s)", strings.Join(themeNames(), ", ")))
	cmd.Flags().StringVarP(&flagMode, "mode", "m", "halfblock", "initial media render mode (halfblock, quadblock, sextant)")
	cmd.Flags().Float64Var(&flagFPS, "fps", 24.0, "playback frame rate for videos")
	cmd.Flags().BoolVarP(&flagImagesOnly, "images-only", "i", false, "filter file list to media files only")

	return cmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
