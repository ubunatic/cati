package spec

import (
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── Style Spec ───────────────────────────────────────────────────────────────

type StyleApp struct {
	Bg          string `yaml:"bg"`
	BorderStyle string `yaml:"border_style"`
	BorderColor string `yaml:"border_color"`
}

type StyleButtons struct {
	Fg          string `yaml:"fg"`
	Bg          string `yaml:"bg"`
	BorderColor string `yaml:"border_color"`
	LeftCap     string `yaml:"left_cap"`
	RightCap    string `yaml:"right_cap"`
	ActiveFg    string `yaml:"active_fg"`
	ActiveBg    string `yaml:"active_bg"`
}

type StylePreview struct {
	Bg string `yaml:"bg"`
}

type StyleControlBar struct {
	Bg string `yaml:"bg"`
	Fg string `yaml:"fg"`
}

type StyleHeaderBar struct {
	Fg   string `yaml:"fg"`
	Bg   string `yaml:"bg"`
	Bold bool   `yaml:"bold"`
}

type StyleGrid struct {
	ItemFg         string `yaml:"item_fg"`
	ItemBg         string `yaml:"item_bg"`
	SelectedFg     string `yaml:"selected_fg"`
	SelectedBg     string `yaml:"selected_bg"`
	SelectedBold   bool   `yaml:"selected_bold"`
	SelectedMarker string `yaml:"selected_marker"`
	ImageBorder    string `yaml:"image_border"`
}

type StyleScrollBar struct {
	ThumbChar string `yaml:"thumb_char"`
	RailChar  string `yaml:"rail_char"`
	Width     int    `yaml:"width"`
	ThumbFg   string `yaml:"thumb_fg"`
	RailFg    string `yaml:"rail_fg"`
	RailBg    string `yaml:"rail_bg"`
}

type StylePageTitle struct {
	Fg   string `yaml:"fg"`
	Bold bool   `yaml:"bold"`
}

type StyleSpec struct {
	Schema     string          `yaml:"$schema"`
	App        StyleApp        `yaml:"app"`
	Buttons    StyleButtons    `yaml:"buttons"`
	Preview    StylePreview    `yaml:"preview"`
	ControlBar StyleControlBar `yaml:"control_bar"`
	HeaderBar  StyleHeaderBar  `yaml:"header_bar"`
	Grid       StyleGrid       `yaml:"grid"`
	ScrollBar  StyleScrollBar  `yaml:"scroll_bar"`
	PageTitle  StylePageTitle  `yaml:"page_title"`
}

func LoadStyle() (StyleSpec, error) {
	return LoadStyleFrom(FS)
}

func LoadStyleFrom(fsys fs.FS) (StyleSpec, error) {
	var spec StyleSpec
	data, err := fs.ReadFile(fsys, "style.yaml")
	if err != nil {
		return spec, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return spec, err
	}
	if spec.App.BorderStyle != "none" && spec.App.BorderStyle != "box" && spec.App.BorderStyle != "double" {
		return spec, fmt.Errorf("style.app.border_style must be none, box, or double")
	}
	if spec.ScrollBar.Width != 1 && spec.ScrollBar.Width != 2 {
		return spec, fmt.Errorf("style.scroll_bar.width must be 1 or 2")
	}
	return spec, nil
}

// ── Theme Spec ───────────────────────────────────────────────────────────────

type ThemeToken struct {
	Fg   string `yaml:"fg"`
	Bg   string `yaml:"bg"`
	Bold bool   `yaml:"bold"`
}

type ThemeSpec map[string]ThemeToken

func LoadTheme() (ThemeSpec, error) {
	var spec ThemeSpec
	data, err := fs.ReadFile(FS, "theme.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

// ── Labels Spec ──────────────────────────────────────────────────────────────

func LoadLabels() (map[string]string, error) {
	var spec map[string]string
	data, err := fs.ReadFile(FS, "labels.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

// ── Buttons Spec ─────────────────────────────────────────────────────────────

type ButtonDef struct {
	Text      string   `yaml:"text"`
	Style     string   `yaml:"style"`
	Prio      int      `yaml:"prio"`
	Action    string   `yaml:"action"`
	Keys      []string `yaml:"keys"`
	AltAction string   `yaml:"alt_action"`
	AltKeys   []string `yaml:"alt_keys"`
}

type ButtonsSpec struct {
	Buttons map[string]ButtonDef `yaml:"buttons"`
}

func LoadButtons() (ButtonsSpec, error) {
	var spec ButtonsSpec
	data, err := fs.ReadFile(FS, "buttons.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

// ── Views Spec ───────────────────────────────────────────────────────────────

type ViewRow map[string]string

type ViewsSpec struct {
	Views map[string][]ViewRow `yaml:"views"`
}

func LoadViews() (ViewsSpec, error) {
	var spec ViewsSpec
	data, err := fs.ReadFile(FS, "views.yaml")
	if err != nil {
		return spec, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec, err
}

// ── Zoom Levels Spec ─────────────────────────────────────────────────────────

type ZoomLevelsDef struct {
	Levels             string `yaml:"levels"`
	Extend             string `yaml:"extend"`
	UpscaleSmallImages bool   `yaml:"upscale_small_images"`
}

type ZoomLevelsSpec struct {
	Levels             []float64
	Extend             string
	UpscaleSmallImages bool
}

func LoadZoomLevels() (ZoomLevelsSpec, error) {
	def := ZoomLevelsDef{
		UpscaleSmallImages: true,
	}
	var spec ZoomLevelsSpec
	data, err := fs.ReadFile(FS, "zoom_levels.yaml")
	if err != nil {
		return spec, err
	}
	if err = yaml.Unmarshal(data, &def); err != nil {
		return spec, err
	}
	spec.Extend = def.Extend
	spec.UpscaleSmallImages = def.UpscaleSmallImages

	parts := strings.Split(def.Levels, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		val, err := strconv.ParseFloat(p, 64)
		if err == nil {
			spec.Levels = append(spec.Levels, val)
		}
	}
	return spec, nil
}

// ── Render Modes Spec ────────────────────────────────────────────────────────

type RenderModeGeometry struct {
	W int `yaml:"w"`
	H int `yaml:"h"`
}

type RenderModeDef struct {
	Name        string              `yaml:"name"`
	Aliases     []string            `yaml:"aliases"`
	Description string              `yaml:"description"`
	Renderer    string              `yaml:"renderer"`
	Cell        RenderModeGeometry  `yaml:"cell"`
	Analysis    *RenderModeGeometry `yaml:"analysis"`
	GlyphSets   []string            `yaml:"glyph_sets"`
	Colorer     string              `yaml:"colorer"`
	SmartStep   string              `yaml:"smart_step"`
	NativeStep  int                 `yaml:"native_step"`
	Optimized   bool                `yaml:"optimized"`
}

type SmartRenderPolicy struct {
	Metric       string  `yaml:"metric"`
	MaxReduction float64 `yaml:"max_reduction"`
	Step         string  `yaml:"step"`
	TieBreak     string  `yaml:"tie_break"`
}

type RenderModesSpec struct {
	Cycle            []string            `yaml:"cycle"`
	Modes            []RenderModeDef     `yaml:"modes"`
	GlyphSets        map[string][]string `yaml:"glyph_sets"`
	Colorers         map[string]string   `yaml:"colorers"`
	Renderers        map[string]string   `yaml:"renderers"`
	Smart            SmartRenderPolicy   `yaml:"smart"`
	SetRegistry      []GlyphSetDef       `yaml:"set_registry"`
	Compositions     map[string][]int    `yaml:"compositions"`
	CompositionInfo  map[string]string   `yaml:"composition_info"`
	CompositionOrder []string            `yaml:"composition_order"`
	Experimental     []string            `yaml:"experimental"`
}

func LoadRenderModes() (RenderModesSpec, error) {
	var spec RenderModesSpec
	data, err := fs.ReadFile(FS, "render_modes.yaml")
	if err != nil {
		return spec, err
	}
	if err = yaml.Unmarshal(data, &spec); err != nil {
		return spec, err
	}
	if err = validateGlyphRegistry(spec); err != nil {
		return RenderModesSpec{}, err
	}
	return spec, nil
}

type GlyphSetDef struct {
	ID          int                `yaml:"id"`
	Name        string             `yaml:"name"`
	Geometry    RenderModeGeometry `yaml:"geometry"`
	Glyphs      []string           `yaml:"glyphs"`
	Masks       map[string]string  `yaml:"masks"`
	Generated   string             `yaml:"generated"`
	Approximate bool               `yaml:"approximate"`
	Optimized   bool               `yaml:"optimized"`
}

type GlyphShape struct {
	Glyph rune
	Mask  []bool
}

type GlyphSetResolution struct {
	IDs         []int
	Glyphs      []rune
	Shapes      []GlyphShape
	Geometry    RenderModeGeometry
	Approximate bool
	Optimized   bool
}

// ResolveGlyphSetExpression resolves a named union or the d<ids> debug grammar.
// Names and aliases are case-sensitive; IDs are sorted and set 0 is implicit.
func ResolveGlyphSetExpression(expression string) (GlyphSetResolution, error) {
	rm, err := LoadRenderModes()
	if err != nil {
		return GlyphSetResolution{}, fmt.Errorf("load render mode registry: %w", err)
	}
	defs := make(map[int]GlyphSetDef, len(rm.SetRegistry))
	byName := make(map[string]int, len(rm.SetRegistry))
	for _, def := range rm.SetRegistry {
		if _, ok := defs[def.ID]; ok || def.Name == "" {
			return GlyphSetResolution{}, fmt.Errorf("invalid glyph set %q/%d", def.Name, def.ID)
		}
		defs[def.ID] = def
		byName[def.Name] = def.ID
	}
	ids, err := resolveExpression(expression, rm.Compositions, byName, defs)
	if err != nil {
		return GlyphSetResolution{}, err
	}
	seen := map[int]bool{0: true}
	normalized := []int{0}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			normalized = append(normalized, id)
		}
	}
	sort.Ints(normalized)
	res := GlyphSetResolution{IDs: normalized, Geometry: RenderModeGeometry{W: 1, H: 1}}
	for _, id := range normalized {
		def := defs[id]
		res.Geometry.W = lcm(res.Geometry.W, def.Geometry.W)
		res.Geometry.H = lcm(res.Geometry.H, def.Geometry.H)
		res.Approximate = res.Approximate || def.Approximate
	}
	res.Optimized = (res.Geometry.W * res.Geometry.H) <= 128
	for _, id := range normalized {
		def := defs[id]
		for _, shape := range glyphSetShapes(def) {
			if containsRune(res.Glyphs, shape.Glyph) {
				continue
			}
			res.Glyphs = append(res.Glyphs, shape.Glyph)
			res.Shapes = append(res.Shapes, GlyphShape{
				Glyph: shape.Glyph,
				Mask:  scaleMask(shape.Mask, def.Geometry, res.Geometry),
			})
		}
	}
	return res, nil
}

func validateGlyphRegistry(rm RenderModesSpec) error {
	defs := make(map[int]GlyphSetDef, len(rm.SetRegistry))
	names := make(map[string]int, len(rm.SetRegistry))
	for _, def := range rm.SetRegistry {
		if def.ID < 0 || def.Name == "" || def.Geometry.W <= 0 || def.Geometry.H <= 0 {
			return fmt.Errorf("invalid glyph set %q/%d", def.Name, def.ID)
		}
		if _, exists := defs[def.ID]; exists {
			return fmt.Errorf("duplicate glyph set ID %d", def.ID)
		}
		if owner, exists := names[def.Name]; exists {
			return fmt.Errorf("duplicate glyph set name %q (IDs %d and %d)", def.Name, owner, def.ID)
		}
		defs[def.ID] = def
		names[def.Name] = def.ID
		shapes, err := parseGlyphSetShapes(def)
		if err != nil {
			return fmt.Errorf("glyph set %d (%s): %w", def.ID, def.Name, err)
		}
		if len(shapes) == 0 {
			return fmt.Errorf("glyph set %d (%s) has no shapes", def.ID, def.Name)
		}
	}
	if _, ok := defs[0]; !ok {
		return fmt.Errorf("glyph set 0 is required")
	}
	for name, ids := range rm.Compositions {
		if name == "" {
			return fmt.Errorf("empty composition name")
		}
		for _, id := range ids {
			if _, ok := defs[id]; !ok {
				return fmt.Errorf("composition %q references unknown glyph set ID %d", name, id)
			}
		}
	}
	seenOrder := make(map[string]bool, len(rm.CompositionOrder))
	for _, name := range rm.CompositionOrder {
		if seenOrder[name] {
			return fmt.Errorf("duplicate composition_order name %q", name)
		}
		seenOrder[name] = true
		if _, ok := rm.Compositions[name]; !ok {
			return fmt.Errorf("composition_order references unknown composition %q", name)
		}
		if rm.CompositionInfo[name] == "" {
			return fmt.Errorf("composition %q has no description", name)
		}
	}
	return nil
}

func glyphSetShapes(def GlyphSetDef) []GlyphShape {
	shapes, _ := parseGlyphSetShapes(def)
	return shapes
}

func parseGlyphSetShapes(def GlyphSetDef) ([]GlyphShape, error) {
	if def.Generated != "" {
		switch def.Generated {
		case "sextant_2x3":
			return generatedSextantShapes(), nil
		case "braille_2x4":
			return generatedBrailleShapes(), nil
		case "unicode_bars":
			return generatedBarShapes(def)
		case "octant_2x4":
			return generatedOctantShapes(), nil
		case "vector_linear_splits":
			return generatedVectorShapes(def.Geometry.W, def.Geometry.H), nil
		default:
			return nil, fmt.Errorf("unknown mask generator %q", def.Generated)
		}
	}
	if len(def.Glyphs) == 0 || len(def.Masks) != len(def.Glyphs) {
		return nil, fmt.Errorf("glyph and mask inventories differ (%d glyphs, %d masks)", len(def.Glyphs), len(def.Masks))
	}
	wantLen := def.Geometry.W * def.Geometry.H
	seen := make(map[rune]bool, len(def.Glyphs))
	shapes := make([]GlyphShape, 0, len(def.Glyphs))
	for _, value := range def.Glyphs {
		runes := []rune(value)
		if len(runes) != 1 || seen[runes[0]] {
			return nil, fmt.Errorf("glyph %q must be one unique rune", value)
		}
		seen[runes[0]] = true
		bits, ok := def.Masks[value]
		if !ok || len(bits) != wantLen {
			return nil, fmt.Errorf("glyph %q mask length is %d, want %d", value, len(bits), wantLen)
		}
		mask := make([]bool, wantLen)
		for i, bit := range []byte(bits) {
			switch bit {
			case '0':
			case '1':
				mask[i] = true
			default:
				return nil, fmt.Errorf("glyph %q mask contains %q", value, bit)
			}
		}
		shapes = append(shapes, GlyphShape{Glyph: runes[0], Mask: mask})
	}
	return shapes, nil
}

func scaleMask(mask []bool, from, to RenderModeGeometry) []bool {
	result := make([]bool, to.W*to.H)
	for y := 0; y < to.H; y++ {
		for x := 0; x < to.W; x++ {
			result[y*to.W+x] = mask[(y*from.H/to.H)*from.W+x*from.W/to.W]
		}
	}
	return result
}

func generatedBarShapes(def GlyphSetDef) ([]GlyphShape, error) {
	shapes := make([]GlyphShape, 0, len(def.Glyphs))
	for _, value := range def.Glyphs {
		runes := []rune(value)
		if len(runes) != 1 {
			return nil, fmt.Errorf("glyph %q must be one rune", value)
		}
		ch := runes[0]
		mask := make([]bool, def.Geometry.W*def.Geometry.H)
		for y := 0; y < def.Geometry.H; y++ {
			for x := 0; x < def.Geometry.W; x++ {
				on, ok := unicodeBarContains(ch, x, y, def.Geometry.W, def.Geometry.H)
				if !ok {
					return nil, fmt.Errorf("glyph %q is not a supported bar", value)
				}
				mask[y*def.Geometry.W+x] = on
			}
		}
		shapes = append(shapes, GlyphShape{Glyph: ch, Mask: mask})
	}
	return shapes, nil
}

func unicodeBarContains(ch rune, x, y, w, h int) (bool, bool) {
	if ch == ' ' {
		return false, true
	}
	if ch == '█' {
		return true, true
	}
	if ch >= '▁' && ch <= '▇' {
		level := int(ch-'▁') + 1
		return (h-y)*8 <= level*h, true
	}
	horizontal := map[rune]int{'▏': 1, '▎': 2, '▍': 3, '▌': 4, '▋': 5, '▊': 6, '▉': 7}
	level, ok := horizontal[ch]
	return (x+1)*8 <= level*w, ok
}

func resolveExpression(expr string, compositions map[string][]int, names map[string]int, defs map[int]GlyphSetDef) ([]int, error) {
	if ids, ok := compositions[expr]; ok {
		return append([]int(nil), ids...), nil
	}
	if expr == "" {
		return nil, fmt.Errorf("empty glyph-set expression")
	}
	if strings.HasPrefix(expr, "d") {
		if expr == "d" {
			return []int{0}, nil
		}
		body := strings.TrimPrefix(expr, "d")
		if body == "" {
			return nil, fmt.Errorf("empty debug expression")
		}
		parts := strings.Split(body, ",")
		ids := make([]int, 0, len(parts))
		for _, part := range parts {
			if part == "" {
				return nil, fmt.Errorf("empty token in debug expression %q", expr)
			}
			id, err := strconv.Atoi(part)
			if err != nil || id < 0 || strconv.Itoa(id) != part {
				return nil, fmt.Errorf("invalid set ID %q in %q", part, expr)
			}
			if _, ok := defs[id]; !ok {
				return nil, fmt.Errorf("unknown glyph set ID %d", id)
			}
			ids = append(ids, id)
		}
		return ids, nil
	}
	operands := make(map[string][]int, len(names)+len(compositions))
	for name, id := range names {
		operands[name] = []int{id}
	}
	for name, ids := range compositions {
		operands[name] = ids
	}
	keys := make([]string, 0, len(operands))
	for name := range operands {
		keys = append(keys, name)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	ids, ok := parseNamedUnion(expr, 0, keys, operands)
	if !ok {
		return nil, fmt.Errorf("invalid glyph-set union %q", expr)
	}
	return ids, nil
}

func parseNamedUnion(expr string, offset int, names []string, operands map[string][]int) ([]int, bool) {
	if offset >= len(expr) {
		return nil, false
	}
	for _, name := range names {
		if !strings.HasPrefix(expr[offset:], name) {
			continue
		}
		next := offset + len(name)
		if next == len(expr) {
			return append([]int(nil), operands[name]...), true
		}
		if expr[next] != '+' || next+1 == len(expr) {
			continue
		}
		rest, ok := parseNamedUnion(expr, next+1, names, operands)
		if ok {
			return append(append([]int(nil), operands[name]...), rest...), true
		}
	}
	return nil, false
}

func generatedBrailleShapes() []GlyphShape {
	shapes := make([]GlyphShape, 256)
	dotBitTable := [8]uint8{0x01, 0x08, 0x02, 0x10, 0x04, 0x20, 0x40, 0x80}
	for mask := 0; mask < 256; mask++ {
		m := make([]bool, 8)
		for idx := 0; idx < 8; idx++ {
			if mask&int(dotBitTable[idx]) != 0 {
				m[idx] = true
			}
		}
		shapes[mask] = GlyphShape{Glyph: rune(0x2800 + mask), Mask: m}
	}
	return shapes
}

func generatedOctantShapes() []GlyphShape {
	// The 26 pre-existing Unicode block characters mapped to their exact 2x4 cell subpixel bitmasks:
	// Bit positions: (x=0,y=0)->bit 0, (x=1,y=0)->bit 1, (x=0,y=1)->bit 2, (x=1,y=1)->bit 3,
	// (x=0,y=2)->bit 4, (x=1,y=2)->bit 5, (x=0,y=3)->bit 6, (x=1,y=3)->bit 7.
	preexisting := map[int]rune{
		0x00: ' ',          // empty
		0xFF: '█',          // full block (U+2588)
		0x01: '\U0001CEA8', // left half upper one quarter
		0x02: '\U0001CEAB', // right half upper one quarter
		0x40: '\U0001CEA3', // left half lower one quarter
		0x80: '\U0001CEA0', // right half lower one quarter
		0x05: '▘',          // top-left quad (U+2598)
		0x0A: '▝',          // top-right quad (U+259D)
		0x50: '▖',          // bot-left quad (U+2596)
		0xA0: '▗',          // bot-right quad (U+2597)
		0x0F: '▀',          // top half (U+2580)
		0xF0: '▄',          // bottom half (U+2584)
		0x55: '▌',          // left half (U+258C)
		0xAA: '▐',          // right half (U+2590)
		0x03: '\U0001FB82', // upper 1/4 bar
		0xC0: '▂',          // lower 1/4 bar (U+2582)
		0x14: '\U0001FBE6', // middle left one quarter
		0x28: '\U0001FBE7', // middle right one quarter
		0x5A: '▞',          // top-right and bottom-left quadrants
		0xA5: '▚',          // top-left and bottom-right quadrants
		0x5F: '▛',          // all quadrants except bottom-right
		0xAF: '▜',          // all quadrants except bottom-left
		0xF5: '▙',          // all quadrants except top-right
		0xFA: '▟',          // all quadrants except top-left
		0xFC: '▆',          // lower 3/4 bar (U+2586)
		0x3F: '\U0001FB85', // upper 3/4 bar
	}
	shapes := make([]GlyphShape, 256)
	octantIdx := 0
	for mask := 0; mask < 256; mask++ {
		m := make([]bool, 8)
		for bit := 0; bit < 8; bit++ {
			if (mask & (1 << bit)) != 0 {
				m[bit] = true
			}
		}
		r, ok := preexisting[mask]
		if !ok {
			r = rune(0x1CD00 + octantIdx)
			octantIdx++
		}
		shapes[mask] = GlyphShape{Glyph: r, Mask: m}
	}
	return shapes
}

func generatedVectorShapes(w, h int) []GlyphShape {
	invertMask := func(m []bool) []bool {
		res := make([]bool, len(m))
		for i, b := range m {
			res[i] = !b
		}
		return res
	}

	evalMask := func(inside func(X, Y float64) bool) []bool {
		mask := make([]bool, w*h)
		const N = 32
		const total = N * N
		fw, fh := float64(w), float64(h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				count := 0
				for sy := 0; sy < N; sy++ {
					Y := (float64(y) + (float64(sy)+0.5)/float64(N)) / fh * fh
					for sx := 0; sx < N; sx++ {
						X := (float64(x) + (float64(sx)+0.5)/float64(N)) / fw * fw
						if inside(X, Y) {
							count++
						}
					}
				}
				if count*2 >= total {
					mask[y*w+x] = true
				}
			}
		}
		return mask
	}

	shapesMap := make(map[rune][]bool)

	// 1. Space and Full block
	shapesMap[' '] = make([]bool, w*h)
	fullMask := make([]bool, w*h)
	for i := range fullMask {
		fullMask[i] = true
	}
	shapesMap['█'] = fullMask

	// 2. Halves
	fw, fh := float64(w), float64(h)
	topHalf := evalMask(func(X, Y float64) bool { return Y <= fh/2.0 })
	shapesMap['▀'] = topHalf
	shapesMap['▄'] = invertMask(topHalf)

	leftHalf := evalMask(func(X, Y float64) bool { return X <= fw/2.0 })
	shapesMap['▌'] = leftHalf
	shapesMap['▐'] = invertMask(leftHalf)

	// 3. Middle vertical bar & middle horizontal bars
	shapesMap['┃'] = evalMask(func(X, Y float64) bool { return X >= fw/4.0 && X <= 3.0*fw/4.0 })
	shapesMap[0x1FB03] = evalMask(func(X, Y float64) bool { return Y >= fh/4.0 && Y <= fh/2.0 })
	shapesMap[0x1FB07] = evalMask(func(X, Y float64) bool { return Y >= fh/2.0 && Y <= 3.0*fh/4.0 })
	shapesMap[0x1FB0B] = evalMask(func(X, Y float64) bool { return Y >= fh/4.0 && Y <= 3.0*fh/4.0 })

	// 4. Straight bars (1/4 and 3/4)
	upQuarter := evalMask(func(X, Y float64) bool { return Y <= fh/4.0 })
	shapesMap[0x1FB82] = upQuarter
	shapesMap['▆'] = invertMask(upQuarter)

	lowQuarter := evalMask(func(X, Y float64) bool { return Y >= 3.0*fh/4.0 })
	shapesMap['▂'] = lowQuarter
	shapesMap[0x1FB85] = invertMask(lowQuarter)

	leftQuarter := evalMask(func(X, Y float64) bool { return X <= fw/4.0 })
	shapesMap['▎'] = leftQuarter
	shapesMap[0x1FB8A] = invertMask(leftQuarter)

	rightQuarter := evalMask(func(X, Y float64) bool { return X >= 3.0*fw/4.0 })
	shapesMap[0x1FB87] = rightQuarter
	shapesMap['▊'] = invertMask(rightQuarter)

	// 5. Quadrants
	ulQuad := evalMask(func(X, Y float64) bool { return X <= fw/2.0 && Y <= fh/2.0 })
	urQuad := evalMask(func(X, Y float64) bool { return X >= fw/2.0 && Y <= fh/2.0 })
	llQuad := evalMask(func(X, Y float64) bool { return X <= fw/2.0 && Y >= fh/2.0 })
	lrQuad := evalMask(func(X, Y float64) bool { return X >= fw/2.0 && Y >= fh/2.0 })

	shapesMap['▘'] = ulQuad
	shapesMap['▝'] = urQuad
	shapesMap['▖'] = llQuad
	shapesMap['▗'] = lrQuad

	diag1 := make([]bool, w*h)
	diag2 := make([]bool, w*h)
	for i := 0; i < w*h; i++ {
		diag1[i] = ulQuad[i] || lrQuad[i]
		diag2[i] = urQuad[i] || llQuad[i]
	}
	shapesMap['▚'] = diag1
	shapesMap['▞'] = diag2

	shapesMap['▛'] = invertMask(lrQuad)
	shapesMap['▜'] = invertMask(llQuad)
	shapesMap['▙'] = invertMask(urQuad)
	shapesMap['▟'] = invertMask(ulQuad)

	// 6. Hourglass & Bowtie
	hourglass := evalMask(func(X, Y float64) bool {
		xNorm, yNorm := X/fw, Y/fh
		return (yNorm <= xNorm && yNorm <= 1.0-xNorm) || (yNorm >= xNorm && yNorm >= 1.0-xNorm)
	})
	shapesMap[0x1FB9A] = hourglass
	shapesMap[0x1FB9B] = invertMask(hourglass)

	// 7. Triangular Corner Blocks
	leftTri := evalMask(func(X, Y float64) bool {
		xNorm, yNorm := X/fw, Y/fh
		return yNorm >= xNorm && yNorm <= 1.0-xNorm
	})
	shapesMap[0x1FB6C] = leftTri
	shapesMap[0x1FB68] = invertMask(leftTri)

	topTri := evalMask(func(X, Y float64) bool {
		xNorm, yNorm := X/fw, Y/fh
		return yNorm <= xNorm && yNorm <= 1.0-xNorm
	})
	shapesMap[0x1FB6D] = topTri
	shapesMap[0x1FB69] = invertMask(topTri)

	rightTri := evalMask(func(X, Y float64) bool {
		xNorm, yNorm := X/fw, Y/fh
		return yNorm <= xNorm && yNorm >= 1.0-xNorm
	})
	shapesMap[0x1FB6E] = rightTri
	shapesMap[0x1FB6A] = invertMask(rightTri)

	bottomTri := evalMask(func(X, Y float64) bool {
		xNorm, yNorm := X/fw, Y/fh
		return yNorm >= xNorm && yNorm >= 1.0-xNorm
	})
	shapesMap[0x1FB6F] = bottomTri
	shapesMap[0x1FB6B] = invertMask(bottomTri)

	// 8. 44 Block Diagonals (0x1FB3C .. 0x1FB67)
	type vecPoint struct{ x, y float64 }
	endpoints := map[string]vecPoint{
		"UL":  {0, 0},
		"UC":  {fw / 2.0, 0},
		"UR":  {fw, 0},
		"UML": {0, fh / 3.0},
		"LML": {0, 2.0 * fh / 3.0},
		"LL":  {0, fh},
		"LC":  {fw / 2.0, fh},
		"LR":  {fw, fh},
		"UMR": {fw, fh / 3.0},
		"LMR": {fw, 2.0 * fh / 3.0},
	}
	corners := map[string]vecPoint{
		"LL": {0, fh},
		"LR": {fw, fh},
		"UL": {0, 0},
		"UR": {fw, 0},
	}

	type diagDef struct {
		r1, r2 rune
		p1, p2 string
		corner string
	}

	diagonals := []diagDef{
		{0x1FB3C, 0x1FB52, "LML", "LC", "LL"},
		{0x1FB3D, 0x1FB53, "LML", "LR", "LL"},
		{0x1FB3E, 0x1FB54, "UML", "LC", "LL"},
		{0x1FB3F, 0x1FB55, "UML", "LR", "LL"},
		{0x1FB40, 0x1FB56, "UL", "LC", "LL"},
		{0x1FB41, 0x1FB57, "UML", "UC", "LR"},
		{0x1FB42, 0x1FB58, "UML", "UR", "LR"},
		{0x1FB43, 0x1FB59, "LML", "UC", "LR"},
		{0x1FB44, 0x1FB5A, "LML", "UR", "LR"},
		{0x1FB45, 0x1FB5B, "LL", "UC", "LR"},
		{0x1FB46, 0x1FB5C, "LML", "UMR", "LR"},
		{0x1FB47, 0x1FB5D, "LC", "LMR", "LR"},
		{0x1FB48, 0x1FB5E, "LL", "LMR", "LR"},
		{0x1FB49, 0x1FB5F, "LC", "UMR", "LR"},
		{0x1FB4A, 0x1FB60, "LL", "UMR", "LR"},
		{0x1FB4B, 0x1FB61, "LC", "UR", "LR"},
		{0x1FB4C, 0x1FB62, "UC", "UMR", "LL"},
		{0x1FB4D, 0x1FB63, "UL", "UMR", "LL"},
		{0x1FB4E, 0x1FB64, "UC", "LMR", "LL"},
		{0x1FB4F, 0x1FB65, "UL", "LMR", "LL"},
		{0x1FB50, 0x1FB66, "UC", "LR", "LL"},
		{0x1FB51, 0x1FB67, "UML", "LMR", "LL"},
	}

	for _, d := range diagonals {
		p1 := endpoints[d.p1]
		p2 := endpoints[d.p2]
		c := corners[d.corner]
		dx := p2.x - p1.x
		dy := p2.y - p1.y
		sc := (c.x-p1.x)*dy - (c.y-p1.y)*dx

		mask1 := evalMask(func(X, Y float64) bool {
			s := (X-p1.x)*dy - (Y-p1.y)*dx
			if sc >= 0 {
				return s >= 0
			}
			return s <= 0
		})

		shapesMap[d.r1] = mask1
		shapesMap[d.r2] = invertMask(mask1)
	}

	runes := []rune{
		' ', '█', '▀', '▄', '▌', '▐', '┃', 0x1FB0B, 0x1FB07, 0x1FB03,
		0x1FB82, 0x1FB85, '▂', '▆', '▎', '▊', 0x1FB87, 0x1FB8A,
		'▖', '▗', '▘', '▙', '▚', '▛', '▜', '▝', '▞', '▟',
		0x1FB9A, 0x1FB9B,
	}
	for r := rune(0x1FB3C); r <= 0x1FB6F; r++ {
		runes = append(runes, r)
	}

	shapes := make([]GlyphShape, 0, len(runes))
	for _, r := range runes {
		mask := shapesMap[r]
		shapes = append(shapes, GlyphShape{Glyph: r, Mask: mask})
	}
	return shapes
}

func generatedSextantShapes() []GlyphShape {
	const subsets = "1 2 12 3 13 23 123 4 14 24 124 34 134 234 1234 5 15 25 125 35 235 1235 45 145 245 1245 345 1345 2345 12345 6 16 26 126 36 136 236 1236 46 146 1246 346 1346 2346 12346 56 156 256 1256 356 1356 2356 12356 456 1456 2456 12456 3456 13456 23456"
	shapes := []GlyphShape{{Glyph: ' ', Mask: make([]bool, 6)}}
	for i, subset := range strings.Fields(subsets) {
		mask := make([]bool, 6)
		for _, digit := range subset {
			mask[int(digit-'1')] = true
		}
		shapes = append(shapes, GlyphShape{Glyph: rune(0x1fb00 + i), Mask: mask})
	}
	return shapes
}
func containsRune(rs []rune, r rune) bool {
	for _, x := range rs {
		if x == r {
			return true
		}
	}
	return false
}
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
func lcm(a, b int) int {
	if a <= 0 || b <= 0 {
		return 0
	}
	return a / gcd(a, b) * b
}

// ── Controls Spec ────────────────────────────────────────────────────────────

type ControlDef struct {
	Type   string   `yaml:"type"`
	Min    int      `yaml:"min"`
	Max    int      `yaml:"max"`
	Values []string `yaml:"values"`
	Set    string   `yaml:"set"`
	Get    string   `yaml:"get"`
}

type ControlsSpec struct {
	Schema   string                `yaml:"$schema"`
	Controls map[string]ControlDef `yaml:"controls"`
	Order    []string              `yaml:"-"`
}

func LoadControls() (ControlsSpec, error) {
	return LoadControlsFrom(FS)
}

func LoadControlsFrom(fsys fs.FS) (ControlsSpec, error) {
	var result ControlsSpec
	data, err := fs.ReadFile(fsys, "controls.yaml")
	if err != nil {
		return result, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return result, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return result, err
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return result, fmt.Errorf("controls.yaml must contain a mapping")
	}
	for i := 0; i+1 < len(root.Content[0].Content); i += 2 {
		if root.Content[0].Content[i].Value == "controls" {
			node := root.Content[0].Content[i+1]
			if node.Kind != yaml.MappingNode {
				return result, fmt.Errorf("controls must be a mapping")
			}
			for j := 0; j+1 < len(node.Content); j += 2 {
				result.Order = append(result.Order, node.Content[j].Value)
			}
		}
	}
	if len(result.Order) == 0 || len(result.Controls) != len(result.Order) {
		return result, fmt.Errorf("controls.yaml must define at least one control")
	}
	for _, key := range result.Order {
		c := result.Controls[key]
		switch c.Type {
		case "int":
			if c.Min >= c.Max {
				return result, fmt.Errorf("control %s must have min < max", key)
			}
		case "enum":
			if len(c.Values) < 2 {
				return result, fmt.Errorf("control %s needs at least two values", key)
			}
		case "bool":
		default:
			return result, fmt.Errorf("control %s has unsupported type %q", key, c.Type)
		}
		if c.Set == "" || c.Get == "" {
			return result, fmt.Errorf("control %s requires set and get bindings", key)
		}
	}
	return result, nil
}

// ── Yaml View Spec (about.yaml) ──────────────────────────────────────────────

type YamlView struct {
	Type     string   `yaml:"type"`
	Name     string   `yaml:"name"`
	Title    string   `yaml:"title"`
	Content  string   `yaml:"content"`
	Controls []string `yaml:"controls"`
}

func LoadYamlView(name string) (*YamlView, error) {
	return LoadYamlViewFrom(FS, name)
}

func LoadYamlViewFrom(fsys fs.FS, name string) (*YamlView, error) {
	var spec YamlView
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err = decoder.Decode(&spec); err != nil {
		return nil, err
	}
	if spec.Type != "view" || spec.Name == "" || spec.Title == "" {
		return nil, fmt.Errorf("%s must define type=view, name, and title", name)
	}
	return &spec, nil
}

// ── Config Spec ──────────────────────────────────────────────────────────────

type ConfigDef struct {
	PreviewHeight     int    `yaml:"preview_height"`
	ViewMode          string `yaml:"view_mode"`
	MaxJobs           int    `yaml:"max_jobs"`
	VideoFrames       int    `yaml:"video_frames"`
	PreviewVideos     bool   `yaml:"preview_videos"`
	VideoPreviewDelay int    `yaml:"video_preview_delay"`
}

type ConfigSpec struct {
	Schema string    `yaml:"$schema"`
	Config ConfigDef `yaml:"config"`
}

// LoadConfigDefaults reads default settings values.
func LoadConfigDefaults() (ConfigSpec, error) {
	return LoadConfigDefaultsFrom(FS)
}

// LoadConfigDefaultsFrom reads and validates settings defaults from fsys.
func LoadConfigDefaultsFrom(fsys fs.FS) (ConfigSpec, error) {
	var spec ConfigSpec
	data, err := fs.ReadFile(fsys, "config.yaml")
	if err != nil {
		return spec, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return spec, err
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return spec, err
	}
	config, ok := document["config"].(map[string]any)
	if !ok {
		return spec, fmt.Errorf("config.yaml must define a config mapping")
	}
	for _, key := range []string{"preview_height", "view_mode", "max_jobs", "video_frames", "preview_videos", "video_preview_delay"} {
		if _, ok := config[key]; !ok {
			return spec, fmt.Errorf("config.yaml is missing config.%s", key)
		}
	}
	c := spec.Config
	if c.PreviewHeight < 10 || c.PreviewHeight > 200 {
		return spec, fmt.Errorf("config.preview_height must be between 10 and 200")
	}
	if c.ViewMode != "grid" && c.ViewMode != "preview" {
		return spec, fmt.Errorf("config.view_mode must be grid or preview")
	}
	if c.MaxJobs < 1 || c.MaxJobs > 32 {
		return spec, fmt.Errorf("config.max_jobs must be between 1 and 32")
	}
	if c.VideoFrames < 1 || c.VideoFrames > 60 {
		return spec, fmt.Errorf("config.video_frames must be between 1 and 60")
	}
	if c.VideoPreviewDelay < 0 || c.VideoPreviewDelay > 5000 {
		return spec, fmt.Errorf("config.video_preview_delay must be between 0 and 5000")
	}
	return spec, nil
}
