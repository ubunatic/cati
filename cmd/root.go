// Package cmd implements the cati CLI using Cobra.
package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/ubunatic/cati/internal/halfblock"
	"github.com/spf13/cobra"
)

// imageExts is the set of still-image file extensions cati recognises.
var imageExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
}

// New returns the root Cobra command for cati.
func New() *cobra.Command {
	var ansiMode     bool
	var recursive    bool
	var noHeader     bool
	var playMode     bool
	var interactMode bool
	var fps          int
	var width        int
	var height       int

	root := &cobra.Command{
		Use:   "cati [flags] <image|dir> [image|dir ...]",
		Short: "cati — cat for images, renders PNGs/JPEGs in the terminal",
		Long: `cati renders PNG/JPEG images in your terminal using Unicode half-block
characters (▀ ▄ █) combined with 24-bit ANSI true-color sequences.

Each terminal cell encodes two vertical pixel rows, giving an effective
resolution of (terminal width) × (2 × terminal height) pixels.

Directories are expanded to all supported images (*.png, *.jpg, *.jpeg)
in sorted order. Use -r to recurse into subdirectories.

Use --play to animate a sequence of frames at --fps frames per second.
Press Ctrl+C to stop playback.`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(opts{
				ansi:        ansiMode,
				recursive:   recursive,
				noHeader:    noHeader,
				playMode:    playMode,
				interactive: interactMode,
				fps:         fps,
				width:       width,
				height:      height,
			}, args)
		},
	}

	root.Flags().BoolVar(&ansiMode,      "ansi",        true,  "render with 24-bit ANSI true-color (default)")
	root.Flags().BoolVarP(&recursive,    "recursive",   "r",   false, "recurse into subdirectories")
	root.Flags().BoolVar(&noHeader,      "no-header",   false, "suppress filename headers between images")
	root.Flags().BoolVarP(&playMode,     "play",        "p",   false, "animate frames in a loop (Ctrl+C to stop)")
	root.Flags().BoolVarP(&interactMode, "interactive", "i",   false, "interactive viewer: +/- zoom, arrow keys pan, q quit")
	root.Flags().IntVar(&fps,            "fps",         0,     "frames per second (0 = auto: native fps for video, 15 for images)")
	root.Flags().IntVarP(&width,         "width",       "w",   0,     "target width in terminal columns (0 = auto)")
	root.Flags().IntVar(&height,         "height",      0,     "target height in terminal rows (0 = auto)")

	return root
}

// ── options ───────────────────────────────────────────────────────────────────

type opts struct {
	ansi        bool
	recursive   bool
	noHeader    bool
	playMode    bool
	interactive bool
	fps         int
	width       int // terminal columns; 0 = auto
	height      int // terminal rows;   0 = auto
}

// ── run ───────────────────────────────────────────────────────────────────────

func run(o opts, args []string) error {
	if !o.ansi {
		return fmt.Errorf("only --ansi mode is supported in this version")
	}

	// Expand args: directories → sorted image file list.
	paths, err := expandArgs(args, o.recursive)
	if err != nil {
		return err
	}

	if o.playMode {
		if len(paths) == 0 {
			return fmt.Errorf("no supported images found")
		}
		return play(paths, o.fps, o.width, o.height)
	}

	if o.interactive {
		isDir := false
		if len(args) == 1 {
			info, err := os.Stat(args[0])
			if err == nil && info.IsDir() {
				isDir = true
			}
		}
		if len(args) > 1 || isDir {
			return browser(args, o.width, o.height)
		}
		if len(paths) == 0 {
			return fmt.Errorf("no supported images found")
		}
		if halfblock.IsVideo(paths[0]) {
			return interactiveVideo(paths[0], o.width, o.height, nil, nil, nil, nil)
		}
		return interactive(paths[0], o.width, o.height)
	}

	if len(paths) == 0 {
		return fmt.Errorf("no supported images found")
	}

	// ── Static render ─────────────────────────────────────────────────────────
	// Determine display dimensions: explicit flags take priority; fall back to
	// the terminal size when both are zero.
	cols, rows := o.width, o.height
	if cols == 0 && rows == 0 {
		cols = halfblock.TermWidth()
	}
	multi := len(paths) > 1

	for _, path := range paths {
		if multi && !o.noHeader {
			fmt.Printf("# %s\n", path)
		}

		img, err := halfblock.LoadImage(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		if cols > 0 || rows > 0 {
			img = halfblock.ScaleToFit(img, cols, rows)
		}

		if err := halfblock.Render(os.Stdout, img); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

// ── directory expansion ───────────────────────────────────────────────────────

// expandArgs resolves each arg: files are kept as-is, directories are walked
// for image files. Returns a sorted, deduplicated list of file paths.
func expandArgs(args []string, recursive bool) ([]string, error) {
	var out []string
	seen := map[string]bool{}

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", arg, err)
		}

		if !info.IsDir() {
			if !seen[arg] {
				out = append(out, arg)
				seen[arg] = true
			}
			continue
		}

		// It's a directory — walk it.
		err = filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Skip subdirectories unless -r was given; always enter the root.
				if path != arg && !recursive {
					return filepath.SkipDir
				}
				return nil
			}
			if isImageFile(path) && !seen[path] {
				out = append(out, path)
				seen[path] = true
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", arg, err)
		}
	}
	return out, nil
}

// isImageFile returns true when the file extension is a supported still image
// or video type.
func isImageFile(path string) bool {
	return imageExts[strings.ToLower(filepath.Ext(path))] || halfblock.IsVideo(path)
}
