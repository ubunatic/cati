// Package cmd implements the cati CLI using Cobra.
package cmd

import (
	"errors"
	"fmt"
	"image"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/cati/v1/halfblock"

	catiterm "ubunatic.com/cati/v1/term"
)

// imageExts is the set of still-image file extensions cati recognises.
var imageExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".svg":  true,
}

// New returns the root Cobra command for cati.
func New() *cobra.Command {
	var ansiMode bool
	var recursive bool
	var noHeader bool
	var playMode bool
	var interactMode bool
	var inputTest bool
	var fps int
	var jobs int
	var width int
	var height int
	var renderMode string
	var prescaler string
	var fullComp bool
	var initialZoom string
	var timeRange string
	var crop string

	root := &cobra.Command{
		Use:   "cati [flags] <image|dir> [image|dir ...]",
		Short: "cati — cat for images, renders PNGs/JPEGs in the terminal",
		Long: `cati renders PNG/JPEG images in your terminal using Unicode half-block
characters (▀ ▄ █) combined with 24-bit ANSI true-color sequences.

Each terminal cell encodes two vertical pixel rows, giving an effective
resolution of (terminal width) × (2 × terminal height) pixels.

Directories are expanded to all supported images (*.png, *.jpg, *.jpeg)
in sorted order. Use -r to recurse into subdirectories.

Use "cati play" for media playback and "cati browse" for the preview browser.`,
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputTest {
				return runInputTest()
			}
			if len(args) == 0 {
				return fmt.Errorf("requires at least 1 arg(s), only received 0")
			}
			rc, err := parseRenderMode(renderMode)
			if err != nil {
				return err
			}
			rc.prescaler, err = parsePrescaleMode(prescaler)
			if err != nil {
				return err
			}
			rc.jobs = jobs
			if playMode {
				return forwardCommand("catiplay", os.Args[1:])
			}
			if interactMode {
				target := "catiplay"
				if len(args) > 1 || singleArgIsDir(args) {
					target = "catibrowse"
				}
				return forwardCommand(target, os.Args[1:])
			}
			return run(opts{
				ansi:        ansiMode,
				recursive:   recursive,
				noHeader:    noHeader,
				fps:         fps,
				jobs:        jobs,
				width:       width,
				height:      height,
				fullComp:    fullComp,
				initialZoom: initialZoom,
				timeRange:   timeRange,
				crop:        crop,
			}, rc, args)
		},
	}

	root.Flags().BoolVar(&ansiMode, "ansi", true, "render with 24-bit ANSI true-color (default)")
	root.Flags().BoolVarP(&recursive, "recursive", "r", false, "recurse into subdirectories")
	root.Flags().BoolVar(&noHeader, "no-header", false, "suppress filename headers between images")
	root.Flags().BoolVarP(&playMode, "play", "p", false, "animate frames in a loop (Ctrl+C to stop)")
	root.Flags().BoolVarP(&interactMode, "interactive", "i", false, "interactive viewer: +/- zoom, arrow keys pan, q quit")
	root.Flags().IntVar(&fps, "fps", 0, "legacy playback frames per second")
	root.Flags().IntVarP(&jobs, "jobs", "j", 0, "parallel worker count for thumbnail and async render work (0 = auto)")
	root.Flags().IntVarP(&width, "width", "w", 0, "target image width in terminal columns (0 = auto)")
	root.Flags().IntVar(&height, "height", 0, "target image height in terminal rows (0 = auto)")
	root.Flags().StringVarP(&renderMode, "mode", "m", "", "render mode: h|half, hs|half/split, q|quad, s|spark, sq|spark+quad, x|six, xh|six+half, sx|spark+six")
	root.Flags().StringVarP(&prescaler, "prescaler", "S", "", "resize prescaler: nn|nearest-neighbor, pyramid")
	root.Flags().BoolVar(&fullComp, "full-comp", false, "compare render quality against original source pixels (slow)")
	root.Flags().StringVarP(&initialZoom, "zoom", "z", "", `initial zoom: "0" = fit to viewport, "1", "1.0", "100%", "1:1" (k=1), "w" = scale to term width, "h" = scale to term height`)
	root.Flags().StringVarP(&crop, "crop", "c", "", "crop final output in terminal cells: W:H, W:H:X:Y, auto|a|1|true, or [l|c|r],[t|m|b]")
	root.Flags().StringVar(&timeRange, "range", "", `playback window: "5s" plays first 5 s; "5s:7s" plays 5 s–7 s (supports s/m/h suffixes, bare seconds, mm:ss)`)
	root.Flags().BoolVar(&inputTest, "input-test", false, "")
	// Hide the debug flag from help output.
	_ = root.Flags().MarkHidden("input-test")
	_ = root.Flags().MarkHidden("play")
	_ = root.Flags().MarkHidden("interactive")
	_ = root.Flags().MarkHidden("fps")

	root.AddCommand(forwardSubcommand("play", "catiplay", "play media with catiplay"))
	root.AddCommand(forwardSubcommand("browse", "catibrowse", "browse files with catibrowse"))

	return root
}

// NewPlay returns the root command for the catiplay binary.
func NewPlay() *cobra.Command {
	var ansiMode bool
	var recursive bool
	var legacyPlay bool
	var legacyInteractive bool
	var fps int
	var jobs int
	var width int
	var height int
	var renderMode string
	var prescaler string
	var fullComp bool
	var initialZoom string
	var timeRange string
	var crop string

	root := &cobra.Command{
		Use:          "catiplay [flags] <image|video|dir> [image|video|dir ...]",
		Short:        "catiplay — terminal media player for images and videos",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("requires at least 1 arg(s), only received 0")
			}
			if !ansiMode {
				return fmt.Errorf("only --ansi mode is supported in this version")
			}
			if jobs < 0 {
				return fmt.Errorf("--jobs must be 0 or greater")
			}
			rc, err := parseRenderMode(renderMode)
			if err != nil {
				return err
			}
			rc.prescaler, err = parsePrescaleMode(prescaler)
			if err != nil {
				return err
			}
			rc.jobs = jobs
			rc = canonicalRenderCfg(rc)
			paths, err := expandArgs(args, recursive)
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				return fmt.Errorf("no supported images found")
			}
			if legacyPlay || len(paths) > 1 || singleArgIsDir(args) {
				tr, err := parseTimeRange(timeRange)
				if err != nil {
					return err
				}
				cropSpec, err := parseCropSpec(crop)
				if err != nil {
					return err
				}
				return play(paths, fps, width, height, rc, tr, cropSpec)
			}
			if halfblock.IsVideo(paths[0]) {
				return interactiveVideo(paths[0], width, height, rc, nil, nil, nil, nil, nil, nil, fullComp, initialZoom)
			}
			_ = legacyInteractive
			return interactive(paths[0], width, height, rc, fullComp, initialZoom)
		},
	}

	root.Flags().BoolVar(&ansiMode, "ansi", true, "render with 24-bit ANSI true-color (default)")
	root.Flags().BoolVarP(&recursive, "recursive", "r", false, "recurse into subdirectories")
	root.Flags().BoolVarP(&legacyPlay, "play", "p", false, "legacy compatibility: play inputs as a frame sequence")
	root.Flags().BoolVarP(&legacyInteractive, "interactive", "i", false, "legacy compatibility: interactive mode is the default")
	root.Flags().IntVar(&fps, "fps", 0, "frames per second (0 = auto: native fps for video, 15 for images)")
	root.Flags().IntVarP(&jobs, "jobs", "j", 0, "parallel worker count for async render work (0 = auto)")
	root.Flags().IntVarP(&width, "width", "w", 0, "target image width in terminal columns (0 = auto)")
	root.Flags().IntVar(&height, "height", 0, "target image height in terminal rows (0 = auto)")
	root.Flags().StringVarP(&renderMode, "mode", "m", "", "render mode: h|half, hs|half/split, q|quad, s|spark, sq|spark+quad, x|six, xh|six+half, sx|spark+six")
	root.Flags().StringVarP(&prescaler, "prescaler", "S", "", "resize prescaler: nn|nearest-neighbor, pyramid")
	root.Flags().BoolVar(&fullComp, "full-comp", false, "compare render quality against original source pixels (slow)")
	root.Flags().StringVarP(&initialZoom, "zoom", "z", "", `initial zoom: "0" = fit to viewport, "1", "1.0", "100%", "1:1" (k=1), "w" = scale to term width, "h" = scale to term height`)
	root.Flags().StringVarP(&crop, "crop", "c", "", "crop final playback output in terminal cells: W:H, W:H:X:Y, auto|a|1|true, or [l|c|r],[t|m|b]")
	root.Flags().StringVar(&timeRange, "range", "", `playback window: "5s" plays first 5 s; "5s:7s" plays 5 s-7 s (supports s/m/h suffixes, bare seconds, mm:ss)`)

	return root
}

// NewBrowse returns the root command for the catibrowse binary.
func NewBrowse() *cobra.Command {
	var ansiMode bool
	var legacyInteractive bool
	var jobs int
	var width int
	var height int
	var renderMode string
	var prescaler string
	var fullComp bool
	var initialZoom string

	root := &cobra.Command{
		Use:          "catibrowse [flags] <image|video|dir> [image|video|dir ...]",
		Short:        "catibrowse — terminal file browser with media previews",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("requires at least 1 arg(s), only received 0")
			}
			if !ansiMode {
				return fmt.Errorf("only --ansi mode is supported in this version")
			}
			if jobs < 0 {
				return fmt.Errorf("--jobs must be 0 or greater")
			}
			rc, err := parseRenderMode(renderMode)
			if err != nil {
				return err
			}
			rc.prescaler, err = parsePrescaleMode(prescaler)
			if err != nil {
				return err
			}
			rc.jobs = jobs
			_ = legacyInteractive
			return browser(args, width, height, canonicalRenderCfg(rc), fullComp, initialZoom, jobs)
		},
	}

	root.Flags().BoolVar(&ansiMode, "ansi", true, "render with 24-bit ANSI true-color (default)")
	root.Flags().BoolVarP(&legacyInteractive, "interactive", "i", false, "legacy compatibility: browser mode is the default")
	root.Flags().IntVarP(&jobs, "jobs", "j", 0, "parallel worker count for thumbnail and async render work (0 = auto)")
	root.Flags().IntVarP(&width, "width", "w", 0, "target image width in terminal columns (0 = auto)")
	root.Flags().IntVar(&height, "height", 0, "target image height in terminal rows (0 = auto)")
	root.Flags().StringVarP(&renderMode, "mode", "m", "", "render mode: h|half, hs|half/split, q|quad, s|spark, sq|spark+quad, x|six, xh|six+half, sx|spark+six")
	root.Flags().StringVarP(&prescaler, "prescaler", "S", "", "resize prescaler: nn|nearest-neighbor, pyramid")
	root.Flags().BoolVar(&fullComp, "full-comp", false, "compare render quality against original source pixels (slow)")
	root.Flags().StringVarP(&initialZoom, "zoom", "z", "", `initial zoom: "0" = fit to viewport, "1", "1.0", "100%", "1:1" (k=1), "w" = scale to term width, "h" = scale to term height`)

	return root
}

func forwardSubcommand(name, executable, short string) *cobra.Command {
	return &cobra.Command{
		Use:                name + " [args...]",
		Short:              short,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return forwardCommand(executable, args)
		},
	}
}

func forwardCommand(executable string, args []string) error {
	c := exec.Command(executable, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Run()
	if err == nil || !errors.Is(err, exec.ErrNotFound) {
		return err
	}
	self, selfErr := os.Executable()
	if selfErr != nil {
		return err
	}
	c = exec.Command(filepath.Join(filepath.Dir(self), executable), args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func singleArgIsDir(args []string) bool {
	if len(args) != 1 {
		return false
	}
	info, err := os.Stat(args[0])
	return err == nil && info.IsDir()
}

func forwardToPlayer(path string, width, height int, rc renderCfg, fullComp bool, initialZoom string, jobs int) error {
	args := []string{}
	if width > 0 {
		args = append(args, "--width", strconv.Itoa(width))
	}
	if height > 0 {
		args = append(args, "--height", strconv.Itoa(height))
	}
	if name := rcModeName(rc); name != "" && name != "?" && name != "half" {
		args = append(args, "--mode", name)
	}
	if initialZoom != "" {
		args = append(args, "--zoom", initialZoom)
	}
	if jobs > 0 {
		args = append(args, "--jobs", strconv.Itoa(jobs))
	}
	if fullComp {
		args = append(args, "--full-comp")
	}
	args = append(args, path)
	return forwardCommand("catiplay", args)
}

// ── options ───────────────────────────────────────────────────────────────────

type opts struct {
	ansi        bool
	recursive   bool
	noHeader    bool
	playMode    bool
	interactive bool
	fps         int
	jobs        int
	width       int    // terminal columns; 0 = auto
	height      int    // image/render rows; 0 = auto
	fullComp    bool   // compare render quality against original source pixels
	initialZoom string // zoom level: 0 → fit to viewport; 1, 1.0, 100%, 1:1 → pixel-perfect (k=1)
	timeRange   string // raw --range value; parsed in run()
	crop        string // final terminal-cell crop spec
}

// ── run ───────────────────────────────────────────────────────────────────────

func run(o opts, rc renderCfg, args []string) error {
	if !o.ansi {
		return fmt.Errorf("only --ansi mode is supported in this version")
	}
	if o.jobs < 0 {
		return fmt.Errorf("--jobs must be 0 or greater")
	}

	// Expand args: directories → sorted image file list.
	paths, err := expandArgs(args, o.recursive)
	if err != nil {
		return err
	}

	rc = canonicalRenderCfg(rc)
	cropSpec, err := parseCropSpec(o.crop)
	if err != nil {
		return err
	}

	if o.playMode {
		if len(paths) == 0 {
			return fmt.Errorf("no supported images found")
		}
		tr, err := parseTimeRange(o.timeRange)
		if err != nil {
			return err
		}
		return play(paths, o.fps, o.width, o.height, rc, tr, cropSpec)
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
			return browser(args, o.width, o.height, rc, o.fullComp, o.initialZoom, o.jobs)
		}
		if len(paths) == 0 {
			return fmt.Errorf("no supported images found")
		}
		if halfblock.IsVideo(paths[0]) {
			return interactiveVideo(paths[0], o.width, o.height, rc, nil, nil, nil, nil, nil, nil, o.fullComp, o.initialZoom)
		}
		return interactive(paths[0], o.width, o.height, rc, o.fullComp, o.initialZoom)
	}

	if len(paths) == 0 {
		return fmt.Errorf("no supported images found")
	}

	// ── Static render ─────────────────────────────────────────────────────────
	// Determine display dimensions: explicit flags take priority; fall back to
	// the terminal size when both are zero.
	multi := len(paths) > 1
	termCols, termRows := o.width, o.height
	if termCols == 0 && termRows == 0 {
		termCols = catiterm.TermWidth()
		termRows = catiterm.TermHeight()
	}

	// Parse --range: for video files in static mode we seek to tr.Start so
	// the displayed frame matches the play-mode entry point.
	tr, err := parseTimeRange(o.timeRange)
	if err != nil {
		return err
	}
	autoCropCols, autoCropRows := catiterm.TermWidth(), catiterm.TermHeight()

	for _, path := range paths {
		if multi && !o.noHeader {
			fmt.Printf("# %s\n", path)
		}

		var img image.Image
		if halfblock.IsVideo(path) && tr.Start > 0 {
			img, err = halfblock.LoadVideoFrameAt(path, tr.Start)
		} else {
			img, err = loadImageForRender(path, termCols, termRows, rc, o.initialZoom)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		img, err = prepareRenderedImageChecked(img, nil, termCols, termRows, rc, o.initialZoom)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
			continue
		}
		img = applyCellCrop(img, rc, cropSpec, autoCropCols, autoCropRows)
		if err := renderChecked(os.Stdout, img, rc); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func loadImageForRender(path string, termCols, termRows int, rc renderCfg, initialZoom string) (image.Image, error) {
	if !halfblock.IsSVG(path) {
		return halfblock.LoadImage(path)
	}
	if termCols == 0 && termRows == 0 {
		termCols = catiterm.TermWidth()
		termRows = catiterm.TermHeight()
	}
	srcW, srcH, err := halfblock.ProbeSVGDimensions(path)
	if err != nil {
		return halfblock.LoadImageWithTarget(path, 0, 0)
	}
	targetW, targetH := renderTargetForSource(srcW, srcH, termCols, termRows, rc, initialZoom)
	return halfblock.LoadImageWithTarget(path, targetW, targetH)
}

// parseRenderMode converts a --mode flag value into a canonical renderCfg.
// The empty value defaults to halfblock.
func parseRenderMode(mode string) (renderCfg, error) {
	key := strings.ToLower(strings.TrimSpace(mode))
	if name, ok := renderModeAliases[key]; ok {
		return findRenderModeByName(name)
	}
	return renderCfg{}, fmt.Errorf("unknown --mode %q; valid: h, hs, q, s, sq, x, xh, sx", mode)
}

func findRenderModeByName(name string) (renderCfg, error) {
	if canonical, ok := renderModeAliases[strings.ToLower(strings.TrimSpace(name))]; ok {
		name = canonical
	}
	for _, m := range renderModes {
		if m.name == name {
			return m.cfg, nil
		}
	}
	return renderCfg{}, fmt.Errorf("unknown render mode %q", name)
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
