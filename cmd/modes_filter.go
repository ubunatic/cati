package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
)

type modesFilter struct {
	list        bool
	all         bool
	composed    bool
	legacy      bool
	exact       bool
	approx      bool
	optimized   bool
	unoptimized bool
	setIDs      []int
	maxGeo      *spec.RenderModeGeometry
	sort        string
	bySSIM      bool
	cols        int
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
			emojig = cati
		}
		// Phase 1: Sequential isolated measurement
		items := make([]*renderedDemoItem, 0, len(entries))
		for _, entry := range entries {
			raw, err := renderModePairRaw(cati, emojig, 12, entry.cfg, false, entry.name, 7)
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
			raw, err := renderModePairRaw(cati, emojig, 12, entry.cfg, false, entry.name, 7)
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

func isModeOptimized(entry renderModeEntry) bool {
	if entry.cfg.useGlyphs() || entry.registryOnly {
		res, err := spec.ResolveGlyphSetExpression(entry.name)
		if err == nil {
			return res.Optimized
		}
	}
	if entry.definition.Name != "" {
		return entry.definition.Optimized
	}
	return true
}

func applyModesFilter(entries []renderModeEntry, filter modesFilter) []renderModeEntry {
	if !filter.composed && !filter.legacy && !filter.exact && !filter.approx && !filter.optimized && !filter.unoptimized && len(filter.setIDs) == 0 && filter.maxGeo == nil {
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
		if filter.optimized && !isModeOptimized(entry) {
			continue
		}
		if filter.unoptimized && isModeOptimized(entry) {
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
