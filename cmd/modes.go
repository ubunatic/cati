package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"ubunatic.com/cati/internal/ansi"
	modelayout "ubunatic.com/cati/internal/modes"
	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/quadblock"
	"ubunatic.com/cati/v1/sextant"
	"ubunatic.com/cati/v1/sparkline"
	catiterm "ubunatic.com/cati/v1/term"
)

// This is assets/cati_0001.png, kept inline so the command works from an
// installed binary without depending on the repository's asset directory.
const embeddedCatiLogo = "iVBORw0KGgoAAAANSUhEUgAAABgAAAAOCAYAAAA1+Nx+AAAAAXNSR0IArs4c6QAAARdJREFUOI1jVBL3/f/WQ42BFkB4xy0GlrceagzCO24x/BWUpKrhzO+fM/wVlGRgYWBggBhuZ0xVC97+vMUgcPIzAxOlBvG5yGBlwwCKBeLF4hgKQmaKMITMFEFRA8PoBn+5/RG/Bd8vsqJoDJkpwnD4FjPD4VvMcMO/X2Rl+H6RFcUQmME8qvwYFrAgcz7tecLAwCDDIF4szvCy9yUDAwMDnEZVw8DAqY9wyL+HnzEMxuoDiMbfGC6kBKD4ABYEMFcevsUMD/816W8wghAG+Fxk4HrwWoBsOAMDJHgOIxkIC0JkgByx2CIZSxygAlxxAAPI4Y8tLijOB4QACwMDJFu//XmLqgbDih9GJXHf/9Quh2CA+f1zBgAYdXGLiM0UnAAAAABJRU5ErkJggg=="

func modesCommand() *cobra.Command {
	var width int
	var renderMode string
	var smart bool
	var info bool
	var listOnly bool
	var allModes bool
	var composedOnly bool
	var legacyOnly bool
	var exactOnly bool
	var approxOnly bool
	var optimizedOnly bool
	var unoptimizedOnly bool
	var setFilter string
	var maxGeoFilter string
	var listSets bool
	var sortOrder string
	var bySSIM bool
	var byPSNR bool
	var imageFlags []string
	var samplePresetFlag string
	var presetAliasFlag string
	var benchmarkFlag bool
	var suiteFlag bool
	var colsFlag string

	cmd := &cobra.Command{
		Use:   "modes [modes/images...]",
		Short: "list render modes with logo demos, presets, custom images, or benchmark scorecards",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listSets {
				return listGlyphSets(cmd.OutOrStdout())
			}

			// Check preset listing
			chosenPreset := samplePresetFlag
			if chosenPreset == "" {
				chosenPreset = presetAliasFlag
			}
			if chosenPreset == "list" || chosenPreset == "help" {
				return listSamplePresets(cmd.OutOrStdout())
			}

			var setIDs []int
			if setFilter != "" {
				ids, err := parseSetIDs(setFilter)
				if err != nil {
					return err
				}
				setIDs = ids
			}
			var maxGeo *spec.RenderModeGeometry
			if maxGeoFilter != "" {
				geo, err := parseGeometry(maxGeoFilter)
				if err != nil {
					return err
				}
				maxGeo = geo
			}

			var cols int
			cleanCols := strings.TrimSpace(strings.ToLower(colsFlag))
			if cleanCols == "" || cleanCols == "auto" || cleanCols == "0" {
				cols = 0
			} else {
				n, err := strconv.Atoi(cleanCols)
				if err != nil || n < 1 {
					return fmt.Errorf("invalid --cols value %q: must be \"auto\" or a positive integer", colsFlag)
				}
				cols = n
			}

			// Separate mode filter names from image/preset arguments
			var modeNames []string
			var inputRefs []string

			for _, imgRef := range imageFlags {
				if imgRef != "" {
					inputRefs = append(inputRefs, imgRef)
				}
			}
			if chosenPreset != "" {
				for _, p := range strings.Split(chosenPreset, ",") {
					p = strings.TrimSpace(p)
					if p != "" {
						inputRefs = append(inputRefs, p)
					}
				}
			}

			for _, arg := range args {
				if len(imageFlags) == 0 && chosenPreset == "" && isImageFileOrPreset(arg) {
					inputRefs = append(inputRefs, arg)
				} else {
					modeNames = append(modeNames, arg)
				}
			}
			if renderMode != "" {
				modeNames = append(modeNames, renderMode)
			}

			var inputImages []loadedImage
			for _, ref := range inputRefs {
				loaded, err := loadNamedImage(ref)
				if err != nil {
					return err
				}
				inputImages = append(inputImages, loaded)
			}

			filter := modesFilter{
				list:        listOnly || (width == 0 && smart),
				all:         allModes,
				composed:    composedOnly,
				legacy:      legacyOnly,
				exact:       exactOnly,
				approx:      approxOnly,
				optimized:   optimizedOnly,
				unoptimized: unoptimizedOnly,
				setIDs:      setIDs,
				maxGeo:      maxGeo,
				sort:        sortOrder,
				bySSIM:      bySSIM || byPSNR,
				cols:        cols,
			}

			if filter.list {
				return listModesFiltered(cmd.OutOrStdout(), info, modeNames, filter)
			}
			if width < 1 {
				return fmt.Errorf("--width must be greater than zero")
			}

			// Filter mode entries
			var entries []renderModeEntry
			var err error
			if len(modeNames) == 0 {
				if filter.legacy {
					entries = append([]renderModeEntry(nil), legacyRenderModes...)
				} else if filter.all {
					entries = append(append([]renderModeEntry(nil), renderModes...), legacyRenderModes...)
				} else {
					entries, err = listableRenderModes()
				}
			} else {
				entries, err = selectedRenderModes(modeNames)
			}
			if err != nil {
				return err
			}
			entries = applyModesFilter(entries, filter)

			if benchmarkFlag || suiteFlag {
				return runModesBenchmarkScorecard(cmd.OutOrStdout(), width, smart, entries, sortOrder)
			}

			return runModesDemoWithImages(cmd.OutOrStdout(), width, smart, info, entries, filter, inputImages)
		},
	}
	cmd.Flags().IntVarP(&width, "width", "W", 0, "target width of each logo demo (0 with --smart or -l lists modes only)")
	cmd.Flags().StringVarP(&renderMode, "mode", "m", "", "render mode to demo (same values as cati --mode)")
	cmd.Flags().BoolVar(&smart, "smart", false, "choose the best nearby width by PSNR")
	cmd.Flags().BoolVar(&info, "info", false, "explain each mode and list its supported Unicode shapes")
	cmd.Flags().BoolVarP(&listOnly, "list", "l", false, "list mode names without rendering logo demos")
	cmd.Flags().BoolVarP(&allModes, "all", "a", false, "include legacy modes in list or demo")
	cmd.Flags().BoolVarP(&composedOnly, "composed", "c", false, "filter to composable / registry modes")
	cmd.Flags().BoolVarP(&legacyOnly, "legacy", "L", false, "filter to legacy renderer modes")
	cmd.Flags().BoolVar(&exactOnly, "exact", false, "filter to modes with exact (non-approximate) glyph coverage")
	cmd.Flags().BoolVar(&approxOnly, "approx", false, "filter to modes with approximate glyph coverage")
	cmd.Flags().BoolVar(&optimizedOnly, "optimized", false, "filter to modes with bitwise optimized solvers")
	cmd.Flags().BoolVar(&optimizedOnly, "opt", false, "alias for --optimized")
	cmd.Flags().BoolVar(&unoptimizedOnly, "unoptimized", false, "filter to modes without bitwise optimized solvers (scalar fallback)")
	cmd.Flags().BoolVar(&unoptimizedOnly, "no-opt", false, "alias for --unoptimized")
	cmd.Flags().StringVarP(&setFilter, "set", "s", "", "filter to modes containing specific glyph set ID(s) (comma-separated, e.g. -s 6, -s 1,6)")
	cmd.Flags().StringVar(&maxGeoFilter, "max-geo", "", "filter to modes with geometry at most WxH (e.g. --max-geo 4x4)")
	cmd.Flags().BoolVar(&listSets, "sets", false, "list registered glyph sets from the spec registry")
	cmd.Flags().StringVar(&sortOrder, "sort", "", "sort modes by: ssim (highest SSIM first), -ssim, time (fastest), -time, eff (or efficiency: lowest (1-ssim)*time), -eff, name, -name")
	cmd.Flags().BoolVar(&bySSIM, "by-ssim", false, "sort modes by highest SSIM first")
	cmd.Flags().BoolVar(&byPSNR, "by-psnr", false, "sort modes by highest SSIM first (alias for --by-ssim)")
	cmd.Flags().StringSliceVarP(&imageFlags, "image", "i", nil, "custom image path(s) or presets to evaluate (repeatable or comma-separated)")
	cmd.Flags().StringVarP(&samplePresetFlag, "sample", "p", "", "test asset sample preset or 'list' (e.g. circle, soldering, summer, darth)")
	cmd.Flags().StringVar(&presetAliasFlag, "preset", "", "alias for --sample")
	cmd.Flags().BoolVar(&benchmarkFlag, "benchmark", false, "run dataset benchmark scorecard across test assets")
	cmd.Flags().BoolVar(&suiteFlag, "suite", false, "alias for --benchmark")
	cmd.Flags().StringVar(&colsFlag, "cols", "", "grid layout columns: auto (1-3 based on terminal width) or explicit number 1, 2, 3...")
	cmd.AddCommand(modesGlyphsCommand())
	cmd.AddCommand(modesProbeCommand())
	return applyCLIDefaultsUnder(cmd, "cati")
}

func isImageFileOrPreset(arg string) bool {
	clean := strings.TrimSpace(arg)
	if clean == "" {
		return false
	}
	lower := strings.ToLower(clean)
	// Known non-mode presets
	for _, p := range samplePresets {
		if p.name == lower {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(clean))
	if imageExts[ext] || halfblock.IsVideo(clean) {
		return true
	}
	if _, err := os.Stat(clean); err == nil {
		// If it's a file on disk and not a known mode alias
		if _, ok := renderModeAliases[clean]; !ok {
			if _, ok := legacyRenderModeAliases[clean]; !ok {
				return true
			}
		}
	}
	return false
}

func parseSetIDs(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.Atoi(p)
		if err != nil || id < 0 {
			return nil, fmt.Errorf("invalid set ID %q", p)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseGeometry(s string) (*spec.RenderModeGeometry, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "x")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid geometry %q (expected WxH, e.g. 4x4)", s)
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid geometry %q (expected positive WxH)", s)
	}
	return &spec.RenderModeGeometry{W: w, H: h}, nil
}

func listGlyphSets(out io.Writer) error {
	modeSpec, err := spec.LoadRenderModes()
	if err != nil {
		return fmt.Errorf("load render mode metadata: %w", err)
	}
	fmt.Fprintln(out, "Registered glyph sets:")
	for _, def := range modeSpec.SetRegistry {
		approx := ""
		if def.Approximate {
			approx = " [approx]"
		}
		var glyphStr string
		if def.Generated == "sextant_2x3" {
			glyphStr = "(64 generated sextants)"
		} else if len(def.Glyphs) > 0 {
			shapes := make([]rune, 0, len(def.Glyphs))
			for _, g := range def.Glyphs {
				r := []rune(g)
				if len(r) > 0 {
					shapes = append(shapes, r[0])
				}
			}
			glyphStr = wrapGlyphs(shapes, 74, 30)
		}
		fmt.Fprintf(out, "  set %-3d %-10s (%dx%d)%s  %s\n", def.ID, def.Name, def.Geometry.W, def.Geometry.H, approx, glyphStr)
	}
	return nil
}

func runModesDemo(out io.Writer, width int, smart bool) error {
	return runModesDemoSelected(out, width, smart, false, nil)
}

func listModes(out io.Writer, info bool, names []string) error {
	return listModesFiltered(out, info, names, modesFilter{})
}

func listModesFiltered(out io.Writer, info bool, names []string, filter modesFilter) error {
	var entries []renderModeEntry
	var err error
	if len(names) == 0 {
		if filter.legacy {
			entries = append([]renderModeEntry(nil), legacyRenderModes...)
		} else if filter.all {
			entries = append(append([]renderModeEntry(nil), renderModes...), legacyRenderModes...)
		} else {
			entries, err = listableRenderModes()
		}
	} else {
		entries, err = selectedRenderModes(names)
	}
	if err != nil {
		return err
	}
	entries = applyModesFilter(entries, filter)
	sortKey := filter.sort
	if filter.bySSIM && sortKey == "" {
		sortKey = "ssim"
	}
	if sortKey != "" {
		sorted, err := sortModeEntries(entries, sortKey)
		if err != nil {
			return err
		}
		entries = sorted
	}
	fmt.Fprintln(out, "Available render modes:")
	modeSpec, err := spec.LoadRenderModes()
	if err != nil {
		return fmt.Errorf("load render mode metadata: %w", err)
	}
	for _, entry := range entries {
		fmt.Fprintln(out, entry.name)
		if info {
			writeModeInfo(out, entry, modeSpec)
		}
	}
	return nil
}

func listableRenderModes() ([]renderModeEntry, error) {
	entries := append([]renderModeEntry(nil), renderModes...)
	rm, err := spec.LoadRenderModes()
	if err != nil {
		return nil, err
	}
	for _, name := range rm.CompositionOrder {
		if _, ok := renderModeAliases[name]; ok {
			continue
		}
		entries = append(entries, renderModeEntry{name: name, registryOnly: true})
	}
	return entries, nil
}

func runModesDemoSelected(out io.Writer, width int, smart, info bool, names []string) error {
	return runModesDemoSelectedFiltered(out, width, smart, info, names, modesFilter{})
}

func runModesDemoSelectedFiltered(out io.Writer, width int, smart, info bool, names []string, filter modesFilter) error {
	var entries []renderModeEntry
	var err error
	if len(names) == 0 {
		if filter.legacy {
			entries = append([]renderModeEntry(nil), legacyRenderModes...)
		} else if filter.all {
			entries = append(append([]renderModeEntry(nil), renderModes...), legacyRenderModes...)
		} else {
			entries, err = listableRenderModes()
		}
	} else {
		entries, err = selectedRenderModes(names)
	}
	if err != nil {
		return err
	}
	entries = applyModesFilter(entries, filter)
	return runModesDemoWithImages(out, width, smart, info, entries, filter, nil)
}

func runModesDemoWithImages(out io.Writer, width int, smart, info bool, entries []renderModeEntry, filter modesFilter, inputImages []loadedImage) error {
	modeSpec, err := spec.LoadRenderModes()
	if err != nil {
		return fmt.Errorf("load render mode metadata: %w", err)
	}

	sortKey := filter.sort
	if filter.bySSIM && sortKey == "" {
		sortKey = "ssim"
	}

	// Case 1: Single Image Mode
	if len(inputImages) == 1 {
		imgItem := inputImages[0]
		fmt.Fprintf(out, "Available render modes for %s (standard | +smart):\n", imgItem.name)

		var items []*renderedSingleDemoItem
		if sortKey == "" || sortKey == "name" || sortKey == "-name" {
			if sortKey == "name" {
				sort.SliceStable(entries, func(i, j int) bool {
					return entries[i].name < entries[j].name
				})
			} else if sortKey == "-name" {
				sort.SliceStable(entries, func(i, j int) bool {
					return entries[i].name > entries[j].name
				})
			}
			items = make([]*renderedSingleDemoItem, 0, len(entries))
			for _, entry := range entries {
				item, err := renderSingleImageItemRaw(imgItem.img, width, entry)
				if err != nil {
					return err
				}
				computeSingleImageItemMetrics(imgItem.img, item, width)
				items = append(items, item)
			}
		} else {
			// Phase 1: Isolated sequential rendering and latency measurement
			items = make([]*renderedSingleDemoItem, 0, len(entries))
			for _, entry := range entries {
				item, err := renderSingleImageItemRaw(imgItem.img, width, entry)
				if err != nil {
					return err
				}
				items = append(items, item)
			}

			// Phase 2: Parallel SSIM computation across all cores
			var wg sync.WaitGroup
			for _, it := range items {
				wg.Add(1)
				go func(item *renderedSingleDemoItem) {
					defer wg.Done()
					computeSingleImageItemMetrics(imgItem.img, item, width)
				}(it)
			}
			wg.Wait()

			if err := sortSingleImageDemoItems(items, sortKey); err != nil {
				return err
			}
		}

		cards := make([][]string, len(items))
		for idx, it := range items {
			cards[idx] = singleImageCardLines(it, info, modeSpec)
		}
		printCardsGrid(out, cards, filter.cols)
		return nil
	}

	// Case 2: Pair Mode (Default or 2 images)
	var leftImg, rightImg image.Image
	var leftName, rightName string
	var termRows int
	if len(inputImages) >= 2 {
		leftImg = inputImages[0].img
		leftName = inputImages[0].name
		rightImg = inputImages[1].img
		rightName = inputImages[1].name
		termRows = 0
	} else {
		cati, err := decodeEmbeddedLogo()
		if err != nil {
			return err
		}
		emojigPath := filepath.Join("testdata", "emojig-icon.svg")
		emojig, err := halfblock.LoadImage(emojigPath)
		if err != nil {
			emojig = cati
		}
		leftImg = cati
		leftName = "cati logo"
		rightImg = emojig
		rightName = "emojig logo"
		termRows = 7
	}

	fmt.Fprintf(out, "Available render modes (%s | %s):\n", leftName, rightName)

	var items []*renderedDemoItem
	if sortKey == "" || sortKey == "name" || sortKey == "-name" {
		if sortKey == "name" {
			sort.SliceStable(entries, func(i, j int) bool {
				return entries[i].name < entries[j].name
			})
		} else if sortKey == "-name" {
			sort.SliceStable(entries, func(i, j int) bool {
				return entries[i].name > entries[j].name
			})
		}
		items = make([]*renderedDemoItem, 0, len(entries))
		for _, entry := range entries {
			item, err := renderSingleDemoItemRaw(leftImg, rightImg, width, smart, entry, termRows)
			if err != nil {
				return err
			}
			computeDemoItemMetrics(leftImg, rightImg, item, width, smart)
			items = append(items, item)
		}
	} else {
		// Metric sorting case:
		// Phase 1: Isolated sequential rendering and latency measurement
		items = make([]*renderedDemoItem, 0, len(entries))
		for _, entry := range entries {
			item, err := renderSingleDemoItemRaw(leftImg, rightImg, width, smart, entry, termRows)
			if err != nil {
				return err
			}
			items = append(items, item)
		}

		// Phase 2: Parallel error and quality metric computation across all cores
		var wg sync.WaitGroup
		for _, it := range items {
			wg.Add(1)
			go func(item *renderedDemoItem) {
				defer wg.Done()
				computeDemoItemMetrics(leftImg, rightImg, item, width, smart)
			}(it)
		}
		wg.Wait()

		if err := sortRenderedDemoItems(items, sortKey, smart); err != nil {
			return err
		}
	}

	cards := make([][]string, len(items))
	for idx, it := range items {
		cards[idx] = pairDemoCardLines(it, smart, info, modeSpec)
	}
	printCardsGrid(out, cards, filter.cols)
	return nil
}

func modeInfoLines(entry renderModeEntry, modeSpec spec.RenderModesSpec) []string {
	var buf bytes.Buffer
	writeModeInfo(&buf, entry, modeSpec)
	text := strings.TrimRight(buf.String(), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func printCardsGrid(out io.Writer, cards [][]string, cols int) {
	lines := formatCardsGrid(cards, cols, catiterm.TermWidth())
	for _, l := range lines {
		fmt.Fprintln(out, l)
	}
}

func printSingleImageDemoItem(out io.Writer, item renderedSingleDemoItem, info bool, modeSpec spec.RenderModesSpec) {
	cards := [][]string{singleImageCardLines(&item, info, modeSpec)}
	printCardsGrid(out, cards, 1)
}

func printDemoItem(out io.Writer, item renderedDemoItem, smart, info bool, modeSpec spec.RenderModesSpec) {
	cards := [][]string{pairDemoCardLines(&item, smart, info, modeSpec)}
	printCardsGrid(out, cards, 1)
}

func selectedRenderModes(names []string) ([]renderModeEntry, error) {
	if len(names) == 0 {
		return listableRenderModes()
	}
	selected := make([]renderModeEntry, 0, len(names))
	for _, name := range names {
		if canonical, ok := renderModeAliases[name]; ok {
			for _, entry := range renderModes {
				if entry.name == canonical {
					selected = append(selected, entry)
					break
				}
			}
			continue
		}
		if legCanon, ok := legacyRenderModeAliases[name]; ok {
			for _, entry := range legacyRenderModes {
				if entry.name == legCanon {
					selected = append(selected, entry)
					break
				}
			}
			continue
		}
		if cfg, err := parseRenderMode(name); err == nil {
			selected = append(selected, renderModeEntry{name: name, cfg: cfg, registryOnly: true})
			continue
		}
		return nil, fmt.Errorf("unknown render mode %q", name)
	}
	return selected, nil
}

func writeModeInfo(out io.Writer, entry renderModeEntry, modeSpec spec.RenderModesSpec) {
	def := entry.definition
	if def.Description == "" {
		for _, candidate := range modeSpec.Modes {
			if candidate.Name == entry.name {
				def = candidate
				break
			}
		}
	}
	if entry.registryOnly {
		def.Description = "Resolved composable glyph-set mode."
	}
	statusStr := "optimized (bitwise algebraic solver)"
	if !isModeOptimized(entry) {
		statusStr = "not yet optimized (slow scalar fallback; cell geometry > 128px)"
	}
	resolution, err := spec.ResolveGlyphSetExpression(entry.name)
	if err == nil {
		fmt.Fprintf(out, "  info: %s\n  sets: %v\n  geometry: %dx%d%s\n  status: %s\n", def.Description, resolution.IDs, resolution.Geometry.W, resolution.Geometry.H, approximateSuffix(resolution.Approximate), statusStr)
	} else {
		fmt.Fprintf(out, "  info: %s\n  status: %s\n", def.Description, statusStr)
	}
	shapes := modeGlyphs(entry, modeSpec)
	fmt.Fprintln(out, "  shapes:", wrapGlyphs(shapes, 74, len("  shapes: ")))
}

func approximateSuffix(approximate bool) string {
	if approximate {
		return " (approximate coverage)"
	}
	return ""
}

func modeGlyphs(entry renderModeEntry, modeSpec spec.RenderModesSpec) []rune {
	if entry.registryOnly || entry.cfg.useGlyphs() {
		resolution, err := spec.ResolveGlyphSetExpression(entry.name)
		if err == nil {
			return resolution.Glyphs
		}
	}
	seen := map[rune]struct{}{}
	var result []rune
	for _, setName := range entry.definition.GlyphSets {
		values := modeSpec.GlyphSets[setName]
		if len(values) == 1 && strings.HasPrefix(values[0], "generated:") {
			if setName == "six" {
				values = make([]string, 0)
				for _, r := range sextant.Glyphs() {
					values = append(values, string(r))
				}
			}
		}
		for _, value := range values {
			for _, r := range value {
				if _, ok := seen[r]; !ok {
					seen[r] = struct{}{}
					result = append(result, r)
				}
			}
		}
	}
	return result
}

func ansiLinesWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, ansiLineWidth(line))
	}
	return width
}

func ansiLineWidth(line string) int             { return ansi.FirstLineWidth(line) }
func padANSILine(line string, width int) string { return ansi.PadLine(line, width) }

func decodeEmbeddedLogo() (image.Image, error) {
	b, err := base64.StdEncoding.DecodeString(embeddedCatiLogo)
	if err != nil {
		return nil, fmt.Errorf("decode embedded cati logo: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("decode embedded cati logo PNG: %w", err)
	}
	return img, nil
}

func renderDemoLines(img image.Image, rc renderCfg) ([]string, error) {
	var buf bytes.Buffer
	var err error
	switch {
	case rc.useGlyphs():
		spec := rc.viewSpec()
		opts := rc.glyphOptions()
		opts.NoLinePrefix = true
		opts.Rows = max(1, ceilDiv(img.Bounds().Dy(), spec.CellH))
		err = sparkline.Render(&buf, img, max(1, ceilDiv(img.Bounds().Dx(), spec.CellW)), opts)
	case rc.useSextant():
		err = sextant.Render(&buf, img, 0, sextant.Options{Mode: rc.sextantMode, NoLinePrefix: true})
	case rc.useQuad():
		opts := rc.quadOpts
		opts.NoLinePrefix = true
		err = quadblock.Render(&buf, img, 0, opts)
	case rc.useSpark():
		spec := rc.viewSpec()
		err = sparkline.Render(&buf, img, max(1, img.Bounds().Dx()/spec.CellW), sparkline.Options{Mode: rc.sparkMode, Rows: max(1, img.Bounds().Dy()/spec.CellH), CellW: spec.CellW, CellH: spec.CellH, AspectX: spec.AspectX, NoLinePrefix: true})
	default:
		err = halfblock.Render(&buf, img, 0, halfblock.Options{NoLinePrefix: true})
	}
	if err != nil {
		return nil, err
	}
	text := strings.TrimSuffix(buf.String(), "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

func autoFitGridCols(cards [][]string, termWidth int) int {
	return modelayout.AutoFitGridCols(cards, termWidth)
}
func formatCardsGrid(cards [][]string, cols, termWidth int) []string {
	return modelayout.FormatCardsGrid(cards, cols, termWidth)
}
func wrapGlyphs(shapes []rune, width, prefixWidth int) string {
	return modelayout.WrapGlyphs(shapes, width, prefixWidth)
}
