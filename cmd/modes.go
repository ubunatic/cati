package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/cati/internal/metrics"
	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/quadblock"
	"ubunatic.com/cati/v1/sextant"
	"ubunatic.com/cati/v1/sparkline"
)

// This is assets/cati_0001.png, kept inline so the command works from an
// installed binary without depending on the repository's asset directory.
const embeddedCatiLogo = "iVBORw0KGgoAAAANSUhEUgAAABgAAAAOCAYAAAA1+Nx+AAAAAXNSR0IArs4c6QAAARdJREFUOI1jVBL3/f/WQ42BFkB4xy0GlrceagzCO24x/BWUpKrhzO+fM/wVlGRgYWBggBhuZ0xVC97+vMUgcPIzAxOlBvG5yGBlwwCKBeLF4hgKQmaKMITMFEFRA8PoBn+5/RG/Bd8vsqJoDJkpwnD4FjPD4VvMcMO/X2Rl+H6RFcUQmME8qvwYFrAgcz7tecLAwCDDIF4szvCy9yUDAwMDnEZVw8DAqY9wyL+HnzEMxuoDiMbfGC6kBKD4ABYEMFcevsUMD/816W8wghAG+Fxk4HrwWoBsOAMDJHgOIxkIC0JkgByx2CIZSxygAlxxAAPI4Y8tLijOB4QACwMDJFu//XmLqgbDih9GJXHf/9Quh2CA+f1zBgAYdXGLiM0UnAAAAABJRU5ErkJggg=="

type modesFilter struct {
	list     bool
	all      bool
	composed bool
	legacy   bool
	exact    bool
	approx   bool
	setIDs   []int
	maxGeo   *spec.RenderModeGeometry
	sort     string
	bySSIM   bool
}

func modesCommand() *cobra.Command {
	var width int
	var smart bool
	var info bool
	var listOnly bool
	var allModes bool
	var composedOnly bool
	var legacyOnly bool
	var exactOnly bool
	var approxOnly bool
	var setFilter string
	var maxGeoFilter string
	var listSets bool
	var sortOrder string
	var bySSIM bool
	var byPSNR bool

	cmd := &cobra.Command{
		Use:   "modes [modes...]",
		Short: "list render modes with a cati/emojig logo demo",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listSets {
				return listGlyphSets(cmd.OutOrStdout())
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
			filter := modesFilter{
				list:     listOnly || (width == 0 && smart),
				all:      allModes,
				composed: composedOnly,
				legacy:   legacyOnly,
				exact:    exactOnly,
				approx:   approxOnly,
				setIDs:   setIDs,
				maxGeo:   maxGeo,
				sort:     sortOrder,
				bySSIM:   bySSIM || byPSNR,
			}
			if filter.list {
				return listModesFiltered(cmd.OutOrStdout(), info, args, filter)
			}
			if width < 1 {
				return fmt.Errorf("--width must be greater than zero")
			}
			return runModesDemoSelectedFiltered(cmd.OutOrStdout(), width, smart, info, args, filter)
		},
	}
	cmd.Flags().IntVarP(&width, "width", "w", 12, "target width of each logo demo (0 with --smart or -l lists modes only)")
	cmd.Flags().BoolVar(&smart, "smart", false, "choose the best nearby width by PSNR")
	cmd.Flags().BoolVar(&info, "info", false, "explain each mode and list its supported Unicode shapes")
	cmd.Flags().BoolVarP(&listOnly, "list", "l", false, "list mode names without rendering logo demos")
	cmd.Flags().BoolVarP(&allModes, "all", "a", false, "include legacy modes in list or demo")
	cmd.Flags().BoolVarP(&composedOnly, "composed", "c", false, "filter to composable / registry modes")
	cmd.Flags().BoolVarP(&legacyOnly, "legacy", "L", false, "filter to legacy renderer modes")
	cmd.Flags().BoolVar(&exactOnly, "exact", false, "filter to modes with exact (non-approximate) glyph coverage")
	cmd.Flags().BoolVar(&approxOnly, "approx", false, "filter to modes with approximate glyph coverage")
	cmd.Flags().StringVarP(&setFilter, "set", "s", "", "filter to modes containing specific glyph set ID(s) (comma-separated, e.g. -s 6, -s 1,6)")
	cmd.Flags().StringVar(&maxGeoFilter, "max-geo", "", "filter to modes with geometry at most WxH (e.g. --max-geo 4x4)")
	cmd.Flags().BoolVar(&listSets, "sets", false, "list registered glyph sets from the spec registry")
	cmd.Flags().StringVar(&sortOrder, "sort", "", "sort modes by: ssim (highest SSIM first), -ssim, time (fastest), -time, eff (or efficiency: lowest (1-ssim)*time), -eff, name, -name")
	cmd.Flags().BoolVar(&bySSIM, "by-ssim", false, "sort modes by highest SSIM first")
	cmd.Flags().BoolVar(&byPSNR, "by-psnr", false, "sort modes by highest SSIM first (alias for --by-ssim)")
	return cmd
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

func sortModeEntries(entries []renderModeEntry, sortKey string) ([]renderModeEntry, error) {
	switch strings.ToLower(strings.TrimSpace(sortKey)) {
	case "name":
		res := append([]renderModeEntry(nil), entries...)
		sort.SliceStable(res, func(i, j int) bool {
			return res[i].name < res[j].name
		})
		return res, nil
	case "-name":
		res := append([]renderModeEntry(nil), entries...)
		sort.SliceStable(res, func(i, j int) bool {
			return res[i].name > res[j].name
		})
		return res, nil
	case "ssim", "psnr", "best", "quality", "-ssim", "-psnr", "worst", "eff", "efficiency", "-eff", "-efficiency":
		cati, err := decodeEmbeddedLogo()
		if err != nil {
			return nil, err
		}
		emojigPath := filepath.Join("testdata", "emojig-icon.svg")
		emojig, err := halfblock.LoadImage(emojigPath)
		if err != nil {
			return nil, fmt.Errorf("load emojig logo %q: %w", emojigPath, err)
		}
		// Phase 1: Sequential isolated measurement
		items := make([]*renderedDemoItem, 0, len(entries))
		for _, entry := range entries {
			raw, err := renderModePairRaw(cati, emojig, 12, entry.cfg, false, entry.name)
			if err != nil {
				return nil, err
			}
			items = append(items, &renderedDemoItem{
				entry:       entry,
				normalStats: modeDemoStats{dur: raw.dur, w: raw.contentW},
				normalRaw:   raw,
			})
		}
		// Phase 2: Parallel SSIM computation
		var wg sync.WaitGroup
		for _, it := range items {
			wg.Add(1)
			go func(item *renderedDemoItem) {
				defer wg.Done()
				item.normalStats.ssim = calcModePairSSIM(cati, emojig, item.normalRaw, 12)
			}(it)
		}
		wg.Wait()

		if err := sortRenderedDemoItems(items, sortKey, false); err != nil {
			return nil, err
		}
		res := make([]renderModeEntry, len(items))
		for i, it := range items {
			res[i] = it.entry
		}
		return res, nil
	case "time", "dur", "fastest", "-time", "-dur", "slowest":
		cati, err := decodeEmbeddedLogo()
		if err != nil {
			return nil, err
		}
		emojigPath := filepath.Join("testdata", "emojig-icon.svg")
		emojig, err := halfblock.LoadImage(emojigPath)
		if err != nil {
			return nil, fmt.Errorf("load emojig logo %q: %w", emojigPath, err)
		}
		items := make([]*renderedDemoItem, 0, len(entries))
		for _, entry := range entries {
			raw, err := renderModePairRaw(cati, emojig, 12, entry.cfg, false, entry.name)
			if err != nil {
				return nil, err
			}
			items = append(items, &renderedDemoItem{
				entry:       entry,
				normalStats: modeDemoStats{dur: raw.dur, w: raw.contentW},
			})
		}
		if err := sortRenderedDemoItems(items, sortKey, false); err != nil {
			return nil, err
		}
		res := make([]renderModeEntry, len(items))
		for i, it := range items {
			res[i] = it.entry
		}
		return res, nil
	default:
		return nil, fmt.Errorf("unknown sort order %q (expected: ssim, -ssim, time, -time, eff, -eff, name, -name)", sortKey)
	}
}

func applyModesFilter(entries []renderModeEntry, filter modesFilter) []renderModeEntry {
	if !filter.composed && !filter.legacy && !filter.exact && !filter.approx && len(filter.setIDs) == 0 && filter.maxGeo == nil {
		return entries
	}
	modeSpec, err := spec.LoadRenderModes()
	if err != nil {
		return entries
	}
	compOrderSet := make(map[string]bool, len(modeSpec.CompositionOrder))
	for _, name := range modeSpec.CompositionOrder {
		compOrderSet[name] = true
	}

	result := make([]renderModeEntry, 0, len(entries))
	for _, entry := range entries {
		res, resErr := spec.ResolveGlyphSetExpression(entry.name)
		isComposed := compOrderSet[entry.name] || entry.registryOnly || entry.cfg.useGlyphs() || (resErr == nil && entry.definition.Renderer == "glyph_union")
		if filter.composed && !isComposed {
			continue
		}
		if filter.legacy && isComposed {
			continue
		}
		if filter.exact {
			if resErr == nil {
				if res.Approximate {
					continue
				}
			}
		}
		if filter.approx {
			if resErr != nil || !res.Approximate {
				continue
			}
		}
		if len(filter.setIDs) > 0 {
			if resErr != nil {
				continue
			}
			hasAll := true
			for _, wantID := range filter.setIDs {
				found := false
				for _, id := range res.IDs {
					if id == wantID {
						found = true
						break
					}
				}
				if !found {
					hasAll = false
					break
				}
			}
			if !hasAll {
				continue
			}
		}
		if filter.maxGeo != nil {
			var gw, gh int
			if resErr == nil {
				gw, gh = res.Geometry.W, res.Geometry.H
			} else {
				spec := entry.cfg.viewSpec()
				gw, gh = spec.CellW, spec.CellH
			}
			if gw > filter.maxGeo.W || gh > filter.maxGeo.H {
				continue
			}
		}
		result = append(result, entry)
	}
	return result
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
	cati, err := decodeEmbeddedLogo()
	if err != nil {
		return err
	}
	emojigPath := filepath.Join("testdata", "emojig-icon.svg")
	emojig, err := halfblock.LoadImage(emojigPath)
	if err != nil {
		return fmt.Errorf("load emojig logo %q: %w", emojigPath, err)
	}

	fmt.Fprintln(out, "Available render modes (cati logo | emojig logo):")
	modeSpec, err := spec.LoadRenderModes()
	if err != nil {
		return fmt.Errorf("load render mode metadata: %w", err)
	}

	sortKey := filter.sort
	if filter.bySSIM && sortKey == "" {
		sortKey = "ssim"
	}

	// 1. Streaming case: unsorted or alphabetical sort (no need to wait for all modes).
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
		for _, entry := range entries {
			item, err := renderSingleDemoItemRaw(cati, emojig, width, smart, entry)
			if err != nil {
				return err
			}
			computeDemoItemMetrics(cati, emojig, item, width, smart)
			printDemoItem(out, *item, smart, info, modeSpec)
		}
		return nil
	}

	// 2. Metric sorting case:
	// Phase 1: Isolated sequential rendering and latency measurement
	items := make([]*renderedDemoItem, 0, len(entries))
	for _, entry := range entries {
		item, err := renderSingleDemoItemRaw(cati, emojig, width, smart, entry)
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
			computeDemoItemMetrics(cati, emojig, item, width, smart)
		}(it)
	}
	wg.Wait()

	if err := sortRenderedDemoItems(items, sortKey, smart); err != nil {
		return err
	}

	for _, item := range items {
		printDemoItem(out, *item, smart, info, modeSpec)
	}
	return nil
}

func renderSingleDemoItemRaw(cati, emojig image.Image, width int, smart bool, entry renderModeEntry) (*renderedDemoItem, error) {
	normalRaw, err := renderModePairRaw(cati, emojig, width, entry.cfg, false, entry.name)
	if err != nil {
		return nil, err
	}
	item := &renderedDemoItem{
		entry:       entry,
		normalLines: normalRaw.pairLines,
		normalStats: modeDemoStats{dur: normalRaw.dur, w: normalRaw.contentW},
		normalRaw:   normalRaw,
	}
	if smart {
		smartRaw, err := renderModePairRaw(cati, emojig, width, entry.cfg, true, entry.name)
		if err != nil {
			return nil, err
		}
		item.smartLines = smartRaw.pairLines
		item.smartStats = modeDemoStats{dur: smartRaw.dur, w: smartRaw.contentW}
		item.smartRaw = smartRaw
	}
	return item, nil
}

func computeDemoItemMetrics(cati, emojig image.Image, item *renderedDemoItem, width int, smart bool) {
	item.normalStats.ssim = calcModePairSSIM(cati, emojig, item.normalRaw, width)
	item.leftTitle = fmt.Sprintf("%s (%dms, w=%d, ssim=%.2f)", item.entry.name, item.normalStats.dur.Milliseconds(), item.normalStats.w, item.normalStats.ssim)
	if smart {
		item.smartStats.ssim = calcModePairSSIM(cati, emojig, item.smartRaw, width)
		item.smartTitle = fmt.Sprintf("+smart (%dms, w=%d, ssim=%.2f)", item.smartStats.dur.Milliseconds(), item.smartStats.w, item.smartStats.ssim)
		colW := ansiLinesWidth(item.normalLines) + 4
		if len(item.leftTitle)+4 > colW {
			colW = len(item.leftTitle) + 4
		}
		item.colW = colW
	}
}

func printDemoItem(out io.Writer, item renderedDemoItem, smart, info bool, modeSpec spec.RenderModesSpec) {
	if !smart {
		fmt.Fprintf(out, "\n%s\n", item.leftTitle)
		for _, line := range item.normalLines {
			fmt.Fprintln(out, line)
		}
		if info {
			writeModeInfo(out, item.entry, modeSpec)
		}
		return
	}
	fmt.Fprintf(out, "\n%-*s%s\n", item.colW, item.leftTitle, item.smartTitle)
	for i := 0; i < max(len(item.normalLines), len(item.smartLines)); i++ {
		var left, right string
		if i < len(item.normalLines) {
			left = item.normalLines[i]
		}
		if i < len(item.smartLines) {
			right = item.smartLines[i]
		}
		fmt.Fprintf(out, "%s    %s\n", padANSILine(left, item.colW-4), right)
	}
	if info {
		writeModeInfo(out, item.entry, modeSpec)
	}
}

type renderedDemoItem struct {
	entry       renderModeEntry
	leftTitle   string
	normalLines []string
	normalStats modeDemoStats
	normalRaw   modePairRawResult
	smartTitle  string
	smartLines  []string
	smartStats  modeDemoStats
	smartRaw    modePairRawResult
	colW        int
}

func sortRenderedDemoItems(items []*renderedDemoItem, sortKey string, smart bool) error {
	switch strings.ToLower(strings.TrimSpace(sortKey)) {
	case "ssim", "psnr", "best", "quality":
		sort.SliceStable(items, func(i, j int) bool {
			sI := items[i].normalStats.ssim
			sJ := items[j].normalStats.ssim
			if smart {
				sI = items[i].smartStats.ssim
				sJ = items[j].smartStats.ssim
			}
			if sI != sJ {
				return sI > sJ
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "-ssim", "-psnr", "worst":
		sort.SliceStable(items, func(i, j int) bool {
			sI := items[i].normalStats.ssim
			sJ := items[j].normalStats.ssim
			if smart {
				sI = items[i].smartStats.ssim
				sJ = items[j].smartStats.ssim
			}
			if sI != sJ {
				return sI < sJ
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "time", "dur", "fastest":
		sort.SliceStable(items, func(i, j int) bool {
			durI := items[i].normalStats.dur
			durJ := items[j].normalStats.dur
			if smart {
				durI = items[i].smartStats.dur
				durJ = items[j].smartStats.dur
			}
			if durI != durJ {
				return durI < durJ
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "-time", "-dur", "slowest":
		sort.SliceStable(items, func(i, j int) bool {
			durI := items[i].normalStats.dur
			durJ := items[j].normalStats.dur
			if smart {
				durI = items[i].smartStats.dur
				durJ = items[j].smartStats.dur
			}
			if durI != durJ {
				return durI > durJ
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "eff", "efficiency":
		sort.SliceStable(items, func(i, j int) bool {
			statI := items[i].normalStats
			statJ := items[j].normalStats
			if smart {
				statI = items[i].smartStats
				statJ = items[j].smartStats
			}
			effI := (1.0 - statI.ssim) * float64(max(1, statI.dur.Microseconds()))
			effJ := (1.0 - statJ.ssim) * float64(max(1, statJ.dur.Microseconds()))
			if effI != effJ {
				return effI < effJ
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "-eff", "-efficiency":
		sort.SliceStable(items, func(i, j int) bool {
			statI := items[i].normalStats
			statJ := items[j].normalStats
			if smart {
				statI = items[i].smartStats
				statJ = items[j].smartStats
			}
			effI := (1.0 - statI.ssim) * float64(max(1, statI.dur.Microseconds()))
			effJ := (1.0 - statJ.ssim) * float64(max(1, statJ.dur.Microseconds()))
			if effI != effJ {
				return effI > effJ
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "name":
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].entry.name < items[j].entry.name
		})
	case "-name":
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].entry.name > items[j].entry.name
		})
	default:
		return fmt.Errorf("unknown sort order %q (expected: ssim, -ssim, time, -time, eff, -eff, name, -name)", sortKey)
	}
	return nil
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
	resolution, err := spec.ResolveGlyphSetExpression(entry.name)
	if err == nil {
		fmt.Fprintf(out, "  info: %s\n  sets: %v\n  geometry: %dx%d%s\n", def.Description, resolution.IDs, resolution.Geometry.W, resolution.Geometry.H, approximateSuffix(resolution.Approximate))
	} else {
		fmt.Fprintf(out, "  info: %s\n", def.Description)
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

func wrapGlyphs(shapes []rune, width, prefixWidth int) string {
	if len(shapes) == 0 {
		return "(none)"
	}
	var b strings.Builder
	lineWidth := prefixWidth
	for i, r := range shapes {
		glyphWidth := 1
		if i > 0 {
			glyphWidth++
		}
		if lineWidth+glyphWidth > width && b.Len() > 0 {
			b.WriteString("\n")
			b.WriteString(strings.Repeat(" ", prefixWidth))
			lineWidth = prefixWidth
			glyphWidth = 1
		}
		if i > 0 && lineWidth > prefixWidth {
			b.WriteByte(' ')
		}
		if r == ' ' {
			// A literal space is invisible in an inventory; U+2420 is the
			// conventional visible marker for the supported U+0020 shape.
			b.WriteRune('␠')
		} else {
			b.WriteRune(r)
		}
		lineWidth += glyphWidth
	}
	return b.String()
}

type modeDemoStats struct {
	dur  time.Duration
	w    int
	ssim float64
}

type modePairRawResult struct {
	left       image.Image
	right      image.Image
	leftLines  []string
	rightLines []string
	pairLines  []string
	contentW   int
	dur        time.Duration
	cfg        renderCfg
}

func renderModePairRaw(cati, emojig image.Image, width int, cfg renderCfg, smart bool, name string) (modePairRawResult, error) {
	start := time.Now()
	cfg.smart = smart
	left, _, err := smartPrepareWithWidth(cati, width, 7, cfg)
	if err != nil {
		return modePairRawResult{}, fmt.Errorf("fit %s cati logo: %w", name, err)
	}
	right, rightW, err := smartPrepareWithWidth(emojig, width, 7, cfg)
	if err != nil {
		return modePairRawResult{}, fmt.Errorf("fit %s emojig logo: %w", name, err)
	}
	leftLines, err := renderDemoLines(left, cfg)
	if err != nil {
		return modePairRawResult{}, fmt.Errorf("render %s cati logo: %w", name, err)
	}
	rightLines, err := renderDemoLines(right, cfg)
	if err != nil {
		return modePairRawResult{}, fmt.Errorf("render %s emojig logo: %w", name, err)
	}
	dur := time.Since(start)

	contentW := rightW
	if contentW <= 0 {
		contentW = width
	}

	leftWidth := ansiLinesWidth(leftLines)
	lines := make([]string, 0, max(len(leftLines), len(rightLines)))
	for i := 0; i < max(len(leftLines), len(rightLines)); i++ {
		var leftLine, rightLine string
		if i < len(leftLines) {
			leftLine = leftLines[i]
		}
		if i < len(rightLines) {
			rightLine = rightLines[i]
		}
		lines = append(lines, fmt.Sprintf("  %s  |  %s", padANSILine(leftLine, leftWidth), padANSILine(rightLine, leftWidth)))
	}
	return modePairRawResult{
		left:       left,
		right:      right,
		leftLines:  leftLines,
		rightLines: rightLines,
		pairLines:  lines,
		contentW:   contentW,
		dur:        dur,
		cfg:        cfg,
	}, nil
}

func calcModePairSSIM(cati, emojig image.Image, raw modePairRawResult, width int) float64 {
	recLeft := renderReconstruction(raw.left, raw.cfg)
	recRight := renderReconstruction(raw.right, raw.cfg)

	// Compare reconstructions against original source images on a common high-resolution
	// canonical canvas (12x24 subpixels per cell) so that low-resolution modes (full, half)
	// correctly reflect their structural fidelity compared to high-resolution modes (six, quad).
	const cellSubW = 12
	const cellSubH = 24

	leftCols := renderedCellSize(raw.left, raw.cfg).Cols
	if leftCols <= 0 {
		leftCols = width
	}
	leftCanonW := leftCols * cellSubW
	leftCanonH := 7 * cellSubH
	refLeft := metrics.PyramidDownscale(cati, leftCanonW, leftCanonH)
	upscaledLeft := nnUpscale(recLeft, leftCanonW, leftCanonH)
	ssimLeft := metrics.SSIMLuminance(refLeft, upscaledLeft)

	rightCols := renderedCellSize(raw.right, raw.cfg).Cols
	if rightCols <= 0 {
		rightCols = raw.contentW
	}
	rightCanonW := rightCols * cellSubW
	rightCanonH := 7 * cellSubH
	refRight := metrics.PyramidDownscale(emojig, rightCanonW, rightCanonH)
	upscaledRight := nnUpscale(recRight, rightCanonW, rightCanonH)
	ssimRight := metrics.SSIMLuminance(refRight, upscaledRight)

	return (ssimLeft + ssimRight) / 2.0
}

func renderModePair(cati, emojig image.Image, width int, cfg renderCfg, smart bool, name string) ([]string, modeDemoStats, error) {
	raw, err := renderModePairRaw(cati, emojig, width, cfg, smart, name)
	if err != nil {
		return nil, modeDemoStats{}, err
	}
	ssim := calcModePairSSIM(cati, emojig, raw, width)
	return raw.pairLines, modeDemoStats{dur: raw.dur, w: raw.contentW, ssim: ssim}, nil
}

func nnUpscale(img image.Image, dstW, dstH int) image.Image {
	sb := img.Bounds()
	srcW, srcH := sb.Dx(), sb.Dy()
	if srcW == 0 || srcH == 0 || dstW == 0 || dstH == 0 {
		return image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		sy := y * srcH / dstH
		for x := 0; x < dstW; x++ {
			sx := x * srcW / dstW
			dst.Set(x, y, img.At(sb.Min.X+sx, sb.Min.Y+sy))
		}
	}
	return dst
}

func imageMSEPercent(ref, rend image.Image) float64 {
	rb, tb := ref.Bounds(), rend.Bounds()
	dx := min(rb.Dx(), tb.Dx())
	dy := min(rb.Dy(), tb.Dy())
	if dx <= 0 || dy <= 0 {
		return 0
	}
	var sum float64
	for y := 0; y < dy; y++ {
		for x := 0; x < dx; x++ {
			ar, ag, ab, aa := ref.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			br, bg, bb, ba := rend.At(tb.Min.X+x, tb.Min.Y+y).RGBA()
			for _, d := range [4]float64{
				float64(ar>>8) - float64(br>>8),
				float64(ag>>8) - float64(bg>>8),
				float64(ab>>8) - float64(bb>>8),
				float64(aa>>8) - float64(ba>>8),
			} {
				sum += d * d
			}
		}
	}
	return (sum / (float64(dx*dy*4) * 255.0 * 255.0)) * 100.0
}

func ansiLinesWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, ansiLineWidth(line))
	}
	return width
}

func ansiLineWidth(line string) int {
	widths := visibleLineWidths(line)
	if len(widths) == 0 {
		return 0
	}
	return widths[0]
}

func padANSILine(line string, width int) string {
	return line + strings.Repeat(" ", max(0, width-ansiLineWidth(line)))
}

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
