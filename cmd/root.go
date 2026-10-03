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
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/cati/internal/viewgeom"
	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/quadblock"

	catiterm "ubunatic.com/cati/v1/term"
)

// imageExts is the set of still-image file extensions cati recognises.
var imageExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".svg":  true,
	".webp": true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
}

// New returns the root Cobra command for cati.
func New() *cobra.Command {
	var ansiMode bool
	var recursive bool
	var noHeader bool
	var playVal string
	var interactMode bool
	var inputTest bool
	var fps int
	var jobs int
	var width int
	var height int
	var pad string
	var aspect string
	var renderMode string
	var prescaler string
	var fullComp bool
	var smart bool
	var initialZoom string
	var timeRange string
	var crop string
	var bench bool
	var benchBudget time.Duration

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
			if bench {
				if len(args) != 1 {
					return fmt.Errorf("--bench requires exactly one media file")
				}
				if jobs < 0 {
					return fmt.Errorf("--jobs must be 0 or greater")
				}
				if benchBudget <= 0 {
					return fmt.Errorf("--bench-budget must be greater than 0")
				}
				return runMediaBenchmark(cmd.OutOrStdout(), args[0], width, height, jobs, renderMode, benchBudget)
			}
			if inputTest {
				return runInputTest()
			}
			if len(args) == 0 {
				return fmt.Errorf("requires at least 1 arg(s), only received 0")
			}
			if err := validateCommonFlags(width, height, aspect, initialZoom, pad); err != nil {
				return err
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
			rc.smart = smart
			playMode, err := parsePlayMode(playVal, cmd.Flags().Changed("play"), &args)
			if err != nil {
				return err
			}
			if playMode != "" {
				err := forwardCommand("catiplay", os.Args[1:])
				if err == nil || !errors.Is(err, exec.ErrNotFound) {
					return err
				}
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
				playMode:    playMode,
				fps:         fps,
				jobs:        jobs,
				width:       width,
				height:      height,
				pad:         pad,
				aspect:      aspect,
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
	root.Flags().StringVarP(&playVal, "play", "p", "", "playback mode: once|repeat|preview (default \"once\")")
	root.Flags().Lookup("play").NoOptDefVal = "once"
	root.Flags().BoolVarP(&interactMode, "interactive", "i", false, "interactive viewer: +/- zoom, arrow keys pan, q quit")
	root.Flags().IntVar(&fps, "fps", 0, "legacy playback frames per second")
	root.Flags().IntVarP(&jobs, "jobs", "j", 0, "parallel worker count for thumbnail and async render work (0 = auto)")
	root.Flags().IntVarP(&width, "width", "W", 0, "target image width in terminal columns (0 = auto)")
	root.Flags().IntVarP(&height, "height", "H", 0, "target image height in terminal rows (0 = auto)")
	root.Flags().StringVar(&pad, "pad", "", "transparently pad source image in pixels (<cols>,<rows>)")
	root.Flags().StringVar(&aspect, "aspect", "default", "source aspect mapping into target cell grid: default|aligned")
	root.Flags().StringVarP(&renderMode, "mode", "m", "", "render mode: h|half, hs|half/split, q|quad, s|spark, sq|spark+quad, x|six, xh|six+half, sx|spark+six")
	root.Flags().StringVarP(&prescaler, "prescaler", "S", "", "resize prescaler: nn|nearest-neighbor, pyramid")
	root.Flags().BoolVar(&fullComp, "full-comp", false, "compare render quality against original source pixels (slow)")
	root.Flags().BoolVar(&smart, "smart", false, "choose the best nearby width by PSNR (static renders; slow)")
	root.Flags().StringVarP(&initialZoom, "zoom", "z", "", `initial zoom: "0" = fit to viewport, "1", "1.0", "100%", "1:1" (k=1), "w" = scale to term width, "h" = scale to term height`)
	root.Flags().StringVarP(&crop, "crop", "c", "", "crop final output in terminal cells: W:H, W:H:X:Y, auto|a|1|true, or [l|c|r],[t|m|b]")
	root.Flags().StringVar(&timeRange, "range", "", `playback window: "5s" plays first 5 s; "5s:7s" plays 5 s–7 s (supports s/m/h suffixes, bare seconds, mm:ss)`)
	root.Flags().BoolVar(&inputTest, "input-test", false, "")
	root.Flags().BoolVar(&bench, "bench", false, "benchmark render speed on one image or video file")
	root.Flags().DurationVar(&benchBudget, "bench-budget", 3*time.Second, "total time budget for an image --bench, split across modes and fast/simple paths (each mode always renders at least once)")
	// Hide the debug flag from help output.
	_ = root.Flags().MarkHidden("input-test")
	_ = root.Flags().MarkHidden("play")
	_ = root.Flags().MarkHidden("interactive")
	_ = root.Flags().MarkHidden("fps")

	root.AddCommand(forwardSubcommand("play", "catiplay", "play media with catiplay"))
	root.AddCommand(forwardSubcommand("browse", "catibrowse", "browse files with catibrowse"))
	root.AddCommand(modesCommand())

	registerRootCompletion(root)

	return root
}

// NewPlay returns the root command for the catiplay binary.
func NewPlay() *cobra.Command {
	var ansiMode bool
	var recursive bool
	var playVal string
	var legacyInteractive bool
	var fps int
	var jobs int
	var width int
	var height int
	var pad string
	var aspect string
	var renderMode string
	var prescaler string
	var fullComp bool
	var smart bool
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
			if err := validateCommonFlags(width, height, aspect, initialZoom, pad); err != nil {
				return err
			}
			if smart {
				return fmt.Errorf("--smart is currently supported for static cati renders only")
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
			rc.smart = smart
			rc = canonicalRenderCfg(rc)
			tr, err := parseTimeRange(timeRange)
			if err != nil {
				return err
			}
			playMode, err := parsePlayMode(playVal, cmd.Flags().Changed("play"), &args)
			if err != nil {
				return err
			}
			paths, err := expandArgs(args, recursive)
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				return fmt.Errorf("no supported images found")
			}
			if playMode != "" || len(paths) > 1 || singleArgIsDir(args) {
				if playMode == "" {
					playMode = "once"
				}
				cropSpec, err := parseCropSpec(crop)
				if err != nil {
					return err
				}
				return play(paths, fps, width, height, rc, tr, cropSpec, aspect, pad, playMode)
			}
			if halfblock.IsVideo(paths[0]) {
				return interactiveVideo(paths[0], width, height, rc, tr, nil, nil, nil, nil, nil, nil, fullComp, initialZoom)
			}
			_ = legacyInteractive
			return interactive(paths[0], width, height, rc, fullComp, initialZoom)
		},
	}

	root.Flags().BoolVar(&ansiMode, "ansi", true, "render with 24-bit ANSI true-color (default)")
	root.Flags().BoolVarP(&recursive, "recursive", "r", false, "recurse into subdirectories")
	root.Flags().StringVarP(&playVal, "play", "p", "", "playback mode: once|repeat|preview (default \"once\")")
	root.Flags().Lookup("play").NoOptDefVal = "once"
	root.Flags().BoolVarP(&legacyInteractive, "interactive", "i", false, "legacy compatibility: interactive mode is the default")
	root.Flags().IntVar(&fps, "fps", 0, "frames per second (0 = auto: native fps for video, 15 for images)")
	root.Flags().IntVarP(&jobs, "jobs", "j", 0, "parallel worker count for async render work (0 = auto)")
	root.Flags().IntVarP(&width, "width", "W", 0, "target image width in terminal columns (0 = auto)")
	root.Flags().IntVarP(&height, "height", "H", 0, "target image height in terminal rows (0 = auto)")
	root.Flags().StringVar(&pad, "pad", "", "transparently pad source image in pixels (<cols>,<rows>)")
	root.Flags().StringVar(&aspect, "aspect", "default", "source aspect mapping into target cell grid: default|aligned")
	root.Flags().StringVarP(&renderMode, "mode", "m", "", "render mode: h|half, hs|half/split, q|quad, s|spark, sq|spark+quad, x|six, xh|six+half, sx|spark+six")
	root.Flags().StringVarP(&prescaler, "prescaler", "S", "", "resize prescaler: nn|nearest-neighbor, pyramid")
	root.Flags().BoolVar(&fullComp, "full-comp", false, "compare render quality against original source pixels (slow)")
	root.Flags().BoolVar(&smart, "smart", false, "choose the best nearby width by PSNR (static renders; slow)")
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
	var pad string
	var aspect string
	var renderMode string
	var prescaler string
	var fullComp bool
	var smart bool
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
			if err := validateCommonFlags(width, height, aspect, initialZoom, pad); err != nil {
				return err
			}
			if smart {
				return fmt.Errorf("--smart is currently supported for static cati renders only")
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
			rc.smart = smart
			_ = legacyInteractive
			return browser(args, width, height, canonicalRenderCfg(rc), fullComp, initialZoom, jobs)
		},
	}

	root.Flags().BoolVar(&ansiMode, "ansi", true, "render with 24-bit ANSI true-color (default)")
	root.Flags().BoolVarP(&legacyInteractive, "interactive", "i", false, "legacy compatibility: browser mode is the default")
	root.Flags().IntVarP(&jobs, "jobs", "j", 0, "parallel worker count for thumbnail and async render work (0 = auto)")
	root.Flags().IntVarP(&width, "width", "W", 0, "target image width in terminal columns (0 = auto)")
	root.Flags().IntVarP(&height, "height", "H", 0, "target image height in terminal rows (0 = auto)")
	root.Flags().StringVar(&pad, "pad", "", "transparently pad source image in pixels (<cols>,<rows>)")
	root.Flags().StringVar(&aspect, "aspect", "default", "source aspect mapping into target cell grid: default|aligned")
	root.Flags().StringVarP(&renderMode, "mode", "m", "", "render mode: h|half, hs|half/split, q|quad, s|spark, sq|spark+quad, x|six, xh|six+half, sx|spark+six")
	root.Flags().StringVarP(&prescaler, "prescaler", "S", "", "resize prescaler: nn|nearest-neighbor, pyramid")
	root.Flags().BoolVar(&fullComp, "full-comp", false, "compare render quality against original source pixels (slow)")
	root.Flags().BoolVar(&smart, "smart", false, "choose the best nearby width by PSNR (static renders; slow)")
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

func isNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist)
}

func forwardCommand(executable string, args []string) error {
	c := exec.Command(executable, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Run()
	if err == nil || !isNotFound(err) {
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
	err2 := c.Run()
	if err2 != nil && isNotFound(err2) {
		return exec.ErrNotFound
	}
	return err2
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
	if rc.prescaler == prescalePyramid {
		args = append(args, "--prescaler", "pyramid")
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
	if rc.smart {
		args = append(args, "--smart")
	}
	args = append(args, path)
	return forwardCommand("catiplay", args)
}

func validateCommonFlags(width, height int, aspect, initialZoom, pad string) error {
	if width < 0 {
		return fmt.Errorf("--width must be 0 or greater, got %d", width)
	}
	if height < 0 {
		return fmt.Errorf("--height must be 0 or greater, got %d", height)
	}
	switch strings.ToLower(strings.TrimSpace(aspect)) {
	case "", "default", "aligned":
		// valid
	default:
		return fmt.Errorf("unknown --aspect %q; valid: default, aligned", aspect)
	}
	if initialZoom != "" {
		if err := validateZoom(initialZoom); err != nil {
			return err
		}
	}
	if pad != "" {
		if _, _, err := parsePadSpec(pad); err != nil {
			return err
		}
	}
	return nil
}

func validateZoom(s string) error {
	trimmed := strings.ToLower(strings.TrimSpace(s))
	if trimmed == "" || trimmed == "w" || trimmed == "h" {
		return nil
	}
	k := viewgeom.ParseZoomK(trimmed)
	if k < 0 {
		return fmt.Errorf("invalid --zoom %q; valid: \"0\", \"1\", \"w\", \"h\", \"100%%\", \"1:1\"", s)
	}
	return nil
}

// ── options ───────────────────────────────────────────────────────────────────

type opts struct {
	ansi        bool
	recursive   bool
	noHeader    bool
	playMode    string
	interactive bool
	fps         int
	jobs        int
	width       int    // terminal columns; 0 = auto
	height      int    // image/render rows; 0 = auto
	pad         string // transparent source padding: <cols>,<rows>
	aspect      string // aspect mapping mode: default|aligned
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

	if o.playMode != "" {
		if len(paths) == 0 {
			return fmt.Errorf("no supported images found")
		}
		tr, err := parseTimeRange(o.timeRange)
		if err != nil {
			return err
		}
		return play(paths, o.fps, o.width, o.height, rc, tr, cropSpec, o.aspect, o.pad, o.playMode)
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
			return interactiveVideo(paths[0], o.width, o.height, rc, TimeRange{}, nil, nil, nil, nil, nil, nil, o.fullComp, o.initialZoom)
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

		if o.pad != "" {
			padCols, padRows, err := parsePadSpec(o.pad)
			if err != nil {
				return err
			}
			img = padSourceImage(img, padCols, padRows)
		}

		if o.initialZoom == "" {
			if o.height > 0 || (o.width > 0 && o.height > 0) || o.aspect == "aligned" {
				img, err = prepareExplicitGridImage(img, termCols, termRows, rc, o.aspect)
			} else {
				img, err = smartPrepare(img, termCols, termRows, rc)
			}
		} else {
			img, err = prepareRenderedImageChecked(img, nil, termCols, termRows, rc, o.initialZoom)
		}
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
	key := mode
	if name, ok := renderModeAliases[key]; ok {
		return findRenderModeByName(name)
	}
	if name, ok := legacyRenderModeAliases[key]; ok {
		return findRenderModeByName(name)
	}
	resolution, err := spec.ResolveGlyphSetExpression(key)
	if err == nil {
		return renderCfg{id: -1, name: key, glyph: &resolution}, nil
	}
	return renderCfg{}, fmt.Errorf("invalid --mode %q: %w", mode, err)
}

func findRenderModeByName(name string) (renderCfg, error) {
	if canonical, ok := renderModeAliases[name]; ok {
		name = canonical
	}
	for _, m := range renderModes {
		if m.name == name {
			return m.cfg, nil
		}
	}
	if legCanon, ok := legacyRenderModeAliases[name]; ok {
		name = legCanon
	}
	for _, m := range legacyRenderModes {
		if m.name == name {
			return m.cfg, nil
		}
	}
	return parseRenderMode(name)
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

func parsePlayMode(playVal string, changed bool, args *[]string) (string, error) {
	if !changed && playVal == "" {
		return "", nil
	}
	mode := strings.ToLower(playVal)
	if mode == "" {
		mode = "once"
	}

	if args != nil {
		origArgs := *args
		for i, arg := range origArgs {
			lower := strings.ToLower(arg)
			if lower == "once" || lower == "repeat" || lower == "preview" {
				mode = lower
				*args = append(origArgs[:i], origArgs[i+1:]...)
				break
			}
		}
	}

	if mode != "once" && mode != "repeat" && mode != "preview" {
		return "", fmt.Errorf("invalid --play mode %q: expected once, repeat, or preview", mode)
	}
	return mode, nil
}


func parsePadSpec(raw string) (int, int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, 0, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) == 1 {
		v, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || v < 0 {
			return 0, 0, fmt.Errorf("invalid --pad value %q: expected <cols>,<rows> or single non-negative integer", raw)
		}
		return v, v, nil
	}
	if len(parts) == 2 {
		cols, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		rows, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil || cols < 0 || rows < 0 {
			return 0, 0, fmt.Errorf("invalid --pad value %q: expected <cols>,<rows> as non-negative integers", raw)
		}
		return cols, rows, nil
	}
	return 0, 0, fmt.Errorf("invalid --pad value %q: expected <cols>,<rows>", raw)
}

func padSourceImage(img image.Image, padCols, padRows int) image.Image {
	if padCols <= 0 && padRows <= 0 {
		return img
	}
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW <= 0 || srcH <= 0 {
		return img
	}
	newW := srcW + padCols
	newH := srcH + padRows
	out := image.NewNRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			out.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}

func prepareExplicitGridImage(orig image.Image, explicitCols, explicitRows int, rc renderCfg, aspectMode string) (image.Image, error) {
	if rc.gray {
		orig = quadblock.ReduceColors(orig, rc.grayColors)
	}
	b := orig.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return orig, nil
	}

	cellW, cellH := rc.renderCellSize()
	targetW := explicitCols * cellW
	targetH := explicitRows * cellH

	if aspectMode == "aligned" {
		if srcW <= targetW && srcH <= targetH {
			extraW := targetW - srcW
			extraH := targetH - srcH
			if extraW > 0 || extraH > 0 {
				return padSourceImage(orig, extraW, extraH), nil
			}
			return orig, nil
		}
	}

	if srcW <= targetW && srcH <= targetH {
		diffW := targetW - srcW
		diffH := targetH - srcH
		if diffW < cellW && diffH < cellH {
			if diffW > 0 || diffH > 0 {
				return padSourceImage(orig, diffW, diffH), nil
			}
			return orig, nil
		}
	}

	if srcW == targetW && srcH == targetH {
		return orig, nil
	}
	return resizeRenderedImage(orig, targetW, targetH, rc), nil
}