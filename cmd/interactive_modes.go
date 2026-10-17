package cmd

import (
	"slices"

	spec "ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/quadblock"
	"ubunatic.com/cati/v1/sextant"
	"ubunatic.com/cati/v1/sparkline"
)

type renderModeEntry struct {
	name         string
	aliases      []string
	definition   spec.RenderModeDef
	cfg          renderCfg
	registryOnly bool
}

var renderModes = loadRenderModeEntries()

var renderModeAliases = buildRenderModeAliases(renderModes)

var legacyRenderModes = loadLegacyRenderModeEntries()

var legacyRenderModeAliases = buildRenderModeAliases(legacyRenderModes)

func loadLegacyRenderModeEntries() []renderModeEntry {
	renderers := map[string]renderCfg{
		"halfblock_exact":      {id: 0},
		"sparkline_half_split": {id: 8, mode: modeHalfSplit, sparkMode: sparkline.HalfSplit},
		"quad_split_half":      {id: 1, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true, HalfblockThreshold: 2}},
		"sparkline_spark":      {id: 9, mode: modeSpark, sparkMode: sparkline.Spark},
		"sparkline_spark_quad": {id: 3, mode: modeSparkQuad, sparkMode: sparkline.Quad},
		"sextant_2x3":          {id: 6, mode: modeSextant, sextantMode: sextant.ModeSextant},
		"sparkline_six_half":   {id: 10, mode: modeSixHalf, sparkMode: sparkline.SixHalf},
		"sparkline_spark_six":  {id: 5, mode: modeSparkSix, sparkMode: sparkline.Best},
	}

	modeSpec, err := spec.LoadRenderModes()
	if err != nil {
		return nil
	}
	cycleSet := make(map[string]bool, len(modeSpec.Cycle))
	for _, name := range modeSpec.Cycle {
		cycleSet[name] = true
	}
	var entries []renderModeEntry
	for _, def := range modeSpec.Modes {
		if cycleSet[def.Name] {
			continue
		}
		cfg, ok := renderers[def.Renderer]
		if !ok {
			continue
		}
		cfg.name = def.Name
		entries = append(entries, renderModeEntry{name: def.Name, aliases: def.Aliases, definition: def, cfg: cfg})
	}
	return entries
}

func loadRenderModeEntries() []renderModeEntry {
	renderers := map[string]renderCfg{
		"halfblock_exact":      {id: 0},
		"sparkline_half_split": {id: 8, mode: modeHalfSplit, sparkMode: sparkline.HalfSplit},
		"quad_split_half":      {id: 1, mode: modeQuad, quadOpts: quadblock.Options{SplitHalf: true, HalfblockThreshold: 2}},
		"sparkline_spark":      {id: 9, mode: modeSpark, sparkMode: sparkline.Spark},
		"sparkline_spark_quad": {id: 3, mode: modeSparkQuad, sparkMode: sparkline.Quad},
		"sextant_2x3":          {id: 6, mode: modeSextant, sextantMode: sextant.ModeSextant},
		"sparkline_six_half":   {id: 10, mode: modeSixHalf, sparkMode: sparkline.SixHalf},
		"sparkline_spark_six":  {id: 5, mode: modeSparkSix, sparkMode: sparkline.Best},
	}

	modeSpec, err := spec.LoadRenderModes()
	if err != nil {
		return []renderModeEntry{{name: "half", aliases: []string{"h"}, cfg: renderCfg{id: 0}}}
	}
	defs := map[string]spec.RenderModeDef{}
	for _, def := range modeSpec.Modes {
		defs[def.Name] = def
	}
	entries := make([]renderModeEntry, 0, len(modeSpec.Cycle))
	for index, name := range modeSpec.Cycle {
		def, ok := defs[name]
		var cfg renderCfg
		if ok {
			cfg, ok = renderers[def.Renderer]
			if !ok {
				if def.Renderer == "glyph_union" {
					resolution, resolveErr := spec.ResolveGlyphSetExpression(name)
					if resolveErr == nil {
						cfg = renderCfg{id: 100 + index, glyph: &resolution}
						ok = true
					}
				}
				if !ok {
					continue
				}
			}
		} else {
			resolution, resolveErr := spec.ResolveGlyphSetExpression(name)
			if resolveErr != nil {
				continue
			}
			cfg = renderCfg{id: 100 + index, glyph: &resolution}
			def = spec.RenderModeDef{Name: name, Description: modeSpec.CompositionInfo[name], Renderer: "glyph_union", Cell: resolution.Geometry, Colorer: "fg_bg_sse", SmartStep: "native", NativeStep: 1}
		}
		cfg.name = name
		resolution, resolveErr := spec.ResolveGlyphSetExpression(name)
		_, registeredComposition := modeSpec.Compositions[name]
		if resolveErr == nil && registeredComposition {
			for alias, ids := range modeSpec.Compositions {
				if alias != name && slices.Equal(ids, resolution.IDs) {
					def.Aliases = append(def.Aliases, alias)
				}
			}
		}
		slices.Sort(def.Aliases)
		def.Aliases = slices.Compact(def.Aliases)
		entries = append(entries, renderModeEntry{name: def.Name, aliases: def.Aliases, definition: def, cfg: cfg})
	}
	if len(entries) == 0 {
		return []renderModeEntry{{name: "half", aliases: []string{"h"}, cfg: renderCfg{id: 0}}}
	}
	return entries
}

func buildRenderModeAliases(entries []renderModeEntry) map[string]string {
	aliases := map[string]string{"": "half"}
	for _, entry := range entries {
		aliases[entry.name] = entry.name
		for _, alias := range entry.aliases {
			aliases[alias] = entry.name
		}
	}
	return aliases
}

func canonicalRenderCfg(rc renderCfg) renderCfg {
	for _, m := range renderModes {
		if sameRenderMode(rc, m.cfg) {
			canon := m.cfg
			canon.prescaler = rc.prescaler
			canon.jobs = rc.jobs
			canon.gray = rc.gray
			canon.grayColors = rc.grayColors
			canon.smart = rc.smart
			return canon
		}
	}
	return rc
}

func sameRenderMode(a, b renderCfg) bool {
	if a.useGlyphs() || b.useGlyphs() {
		return a.useGlyphs() && b.useGlyphs() && a.name == b.name
	}
	if a.mode != b.mode || a.sparkMode != b.sparkMode {
		return false
	}
	if a.sextantMode != b.sextantMode {
		return false
	}
	if (a.preScale == nil) != (b.preScale == nil) {
		return false
	}
	if a.quadOpts.SplitHalf || b.quadOpts.SplitHalf {
		// Threshold is a quality policy, not a distinct cycle mode. This also
		// keeps older callers' zero-threshold startup config canonicalizable.
		a.quadOpts.HalfblockThreshold = 0
		b.quadOpts.HalfblockThreshold = 0
	}
	// Progress reporting is an observer and does not change rendering mode.
	a.quadOpts.OnProgress = nil
	b.quadOpts.OnProgress = nil
	return a.quadOpts.NoLinePrefix == b.quadOpts.NoLinePrefix &&
		a.quadOpts.HalfblockThreshold == b.quadOpts.HalfblockThreshold &&
		a.quadOpts.Blend == b.quadOpts.Blend &&
		a.quadOpts.SplitHalf == b.quadOpts.SplitHalf &&
		a.quadOpts.SplitHalfNeighbors == b.quadOpts.SplitHalfNeighbors &&
		a.quadOpts.LumSplit == b.quadOpts.LumSplit &&
		a.quadOpts.PCA2 == b.quadOpts.PCA2 &&
		a.quadOpts.Diameter == b.quadOpts.Diameter &&
		a.quadOpts.KMeans == b.quadOpts.KMeans &&
		a.quadOpts.EdgeSnap == b.quadOpts.EdgeSnap &&
		a.quadOpts.Rows == b.quadOpts.Rows &&
		a.quadOpts.Jobs == b.quadOpts.Jobs
}

// cycleRenderCfg returns the next renderCfg in the cycle and its display name.
// The gray state is carried over from the current cfg.
func cycleRenderCfg(rc renderCfg) (renderCfg, string) {
	for i, m := range renderModes {
		if m.cfg.id == rc.id {
			next := renderModes[(i+1)%len(renderModes)]
			next.cfg.prescaler = rc.prescaler
			next.cfg.jobs = rc.jobs
			next.cfg.gray = rc.gray
			next.cfg.grayColors = rc.grayColors
			next.cfg.smart = rc.smart
			return next.cfg, next.name
		}
	}
	next := renderModes[0]
	next.cfg.prescaler = rc.prescaler
	next.cfg.jobs = rc.jobs
	next.cfg.gray = rc.gray
	next.cfg.grayColors = rc.grayColors
	next.cfg.smart = rc.smart
	return next.cfg, next.name
}

// cycleRenderCfgPrev returns the previous renderCfg in the cycle and its display name.
// The gray state is carried over from the current cfg.
func cycleRenderCfgPrev(rc renderCfg) (renderCfg, string) {
	n := len(renderModes)
	for i, m := range renderModes {
		if m.cfg.id == rc.id {
			prev := renderModes[(i+n-1)%n]
			prev.cfg.prescaler = rc.prescaler
			prev.cfg.jobs = rc.jobs
			prev.cfg.gray = rc.gray
			prev.cfg.grayColors = rc.grayColors
			prev.cfg.smart = rc.smart
			return prev.cfg, prev.name
		}
	}
	prev := renderModes[n-1]
	prev.cfg.prescaler = rc.prescaler
	prev.cfg.jobs = rc.jobs
	prev.cfg.gray = rc.gray
	prev.cfg.grayColors = rc.grayColors
	prev.cfg.smart = rc.smart
	return prev.cfg, prev.name
}

// findRenderModeByID looks up a render mode by id in renderModes.
func findRenderModeByID(id int) (renderCfg, string, bool) {
	for _, m := range renderModes {
		if m.cfg.id == id {
			return m.cfg, m.name, true
		}
	}
	return renderCfg{}, "", false
}

// ── zoom levels ──────────────────────────────────────────────────────────────

// maxZoom returns the zoom level at which each terminal cell covers exactly
// 1 source pixel column × 2 source pixel rows (1:1 pixel-perfect).
