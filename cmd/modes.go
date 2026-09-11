package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
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
	composed bool
	legacy   bool
	exact    bool
	approx   bool
	setIDs   []int
	maxGeo   *spec.RenderModeGeometry
}

func modesCommand() *cobra.Command {
	var width int
	var smart bool
	var info bool
	var listOnly bool
	var composedOnly bool
	var legacyOnly bool
	var exactOnly bool
	var approxOnly bool
	var setFilter string
	var maxGeoFilter string
	var listSets bool

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
				composed: composedOnly,
				legacy:   legacyOnly,
				exact:    exactOnly,
				approx:   approxOnly,
				setIDs:   setIDs,
				maxGeo:   maxGeo,
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
	cmd.Flags().BoolVarP(&composedOnly, "composed", "c", false, "filter to composable / registry modes")
	cmd.Flags().BoolVarP(&legacyOnly, "legacy", "L", false, "filter to legacy renderer modes")
	cmd.Flags().BoolVar(&exactOnly, "exact", false, "filter to modes with exact (non-approximate) glyph coverage")
	cmd.Flags().BoolVar(&approxOnly, "approx", false, "filter to modes with approximate glyph coverage")
	cmd.Flags().StringVarP(&setFilter, "set", "s", "", "filter to modes containing specific glyph set ID(s) (comma-separated, e.g. -s 6, -s 1,6)")
	cmd.Flags().StringVar(&maxGeoFilter, "max-geo", "", "filter to modes with geometry at most WxH (e.g. --max-geo 4x4)")
	cmd.Flags().BoolVar(&listSets, "sets", false, "list registered glyph sets from the spec registry")
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
	entries, err := selectedRenderModes(names)
	if err != nil {
		return err
	}
	entries = applyModesFilter(entries, filter)
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
		entries = renderModes
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
	for _, entry := range entries {
		normal, err := renderModePair(cati, emojig, width, entry.cfg, false, entry.name)
		if err != nil {
			return err
		}
		if !smart {
			fmt.Fprintf(out, "\n%s\n", entry.name)
			for _, line := range normal {
				fmt.Fprintln(out, line)
			}
			if info {
				writeModeInfo(out, entry, modeSpec)
			}
			continue
		}
		smartLines, err := renderModePair(cati, emojig, width, entry.cfg, true, entry.name)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "\n%-*s+smart\n", ansiLinesWidth(normal)+4, entry.name)
		for i := 0; i < max(len(normal), len(smartLines)); i++ {
			var left, right string
			if i < len(normal) {
				left = normal[i]
			}
			if i < len(smartLines) {
				right = smartLines[i]
			}
			fmt.Fprintf(out, "%s    %s\n", left, right)
		}
		if info {
			writeModeInfo(out, entry, modeSpec)
		}
	}
	return nil
}

func selectedRenderModes(names []string) ([]renderModeEntry, error) {
	if len(names) == 0 {
		return listableRenderModes()
	}
	selected := make([]renderModeEntry, 0, len(names))
	for _, name := range names {
		canonical, ok := renderModeAliases[name]
		if !ok {
			if cfg, err := parseRenderMode(name); err == nil {
				selected = append(selected, renderModeEntry{name: name, cfg: cfg, registryOnly: true})
				continue
			}
			return nil, fmt.Errorf("unknown render mode %q", name)
		}
		for _, entry := range renderModes {
			if entry.name == canonical {
				selected = append(selected, entry)
				break
			}
		}
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

func renderModePair(cati, emojig image.Image, width int, cfg renderCfg, smart bool, name string) ([]string, error) {
	cfg.smart = smart
	left, err := smartPrepare(cati, width, 7, cfg)
	if err != nil {
		return nil, fmt.Errorf("fit %s cati logo: %w", name, err)
	}
	right, err := smartPrepare(emojig, width, 7, cfg)
	if err != nil {
		return nil, fmt.Errorf("fit %s emojig logo: %w", name, err)
	}
	leftLines, err := renderDemoLines(left, cfg)
	if err != nil {
		return nil, fmt.Errorf("render %s cati logo: %w", name, err)
	}
	rightLines, err := renderDemoLines(right, cfg)
	if err != nil {
		return nil, fmt.Errorf("render %s emojig logo: %w", name, err)
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
	return lines, nil
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
