package cmd

import (
	"fmt"
	"image"
	"sort"
	"strings"
	"time"

	"ubunatic.com/cati/spec"
)

type renderedSingleDemoItem struct {
	entry       renderModeEntry
	leftTitle   string
	normalLines []string
	normalStats modeDemoStats
	normalRaw   image.Image
	smartTitle  string
	smartLines  []string
	smartStats  modeDemoStats
	smartRaw    image.Image
	colW        int
}

func renderSingleImageItemRaw(src image.Image, width int, entry renderModeEntry) (*renderedSingleDemoItem, error) {
	// Normal
	normalCfg := entry.cfg
	normalCfg.smart = false
	start := time.Now()
	normalFitted, normalW, err := smartPrepareSelected(src, width, 0, normalCfg)
	if err != nil {
		return nil, fmt.Errorf("prepare %s: %w", entry.name, err)
	}
	normalLines, err := renderDemoLines(normalFitted, normalCfg)
	normalDur := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", entry.name, err)
	}

	// Smart
	smartCfg := entry.cfg
	smartCfg.smart = true
	start = time.Now()
	smartFitted, smartW, err := smartPrepareSelected(src, width, 0, smartCfg)
	if err != nil {
		return nil, fmt.Errorf("prepare smart %s: %w", entry.name, err)
	}
	smartLines, err := renderDemoLines(smartFitted, smartCfg)
	smartDur := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("render smart %s: %w", entry.name, err)
	}

	return &renderedSingleDemoItem{
		entry:       entry,
		normalLines: normalLines,
		normalStats: modeDemoStats{dur: normalDur, w: normalW},
		normalRaw:   normalFitted,
		smartLines:  smartLines,
		smartStats:  modeDemoStats{dur: smartDur, w: smartW},
		smartRaw:    smartFitted,
	}, nil
}

func computeSingleImageItemMetrics(src image.Image, item *renderedSingleDemoItem, width int) {
	normalCfg := item.entry.cfg
	normalCfg.smart = false
	item.normalStats.ssim = calcSingleImageSSIM(src, item.normalRaw, normalCfg, width)
	item.leftTitle = fmt.Sprintf("%s (%dms, w=%d, ssim=%.2f)", item.entry.name, item.normalStats.dur.Milliseconds(), item.normalStats.w, item.normalStats.ssim)

	smartCfg := item.entry.cfg
	smartCfg.smart = true
	item.smartStats.ssim = calcSingleImageSSIM(src, item.smartRaw, smartCfg, width)
	item.smartTitle = fmt.Sprintf("+smart (%dms, w=%d, ssim=%.2f)", item.smartStats.dur.Milliseconds(), item.smartStats.w, item.smartStats.ssim)

	colW := ansiLinesWidth(item.normalLines) + 4
	if len(item.leftTitle)+4 > colW {
		colW = len(item.leftTitle) + 4
	}
	item.colW = colW
}

func sortSingleImageDemoItems(items []*renderedSingleDemoItem, sortKey string) error {
	switch strings.ToLower(strings.TrimSpace(sortKey)) {
	case "ssim", "psnr", "best", "quality":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].normalStats.ssim != items[j].normalStats.ssim {
				return items[i].normalStats.ssim > items[j].normalStats.ssim
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "-ssim", "-psnr", "worst":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].normalStats.ssim != items[j].normalStats.ssim {
				return items[i].normalStats.ssim < items[j].normalStats.ssim
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "time", "dur", "fastest":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].normalStats.dur != items[j].normalStats.dur {
				return items[i].normalStats.dur < items[j].normalStats.dur
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "-time", "-dur", "slowest":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].normalStats.dur != items[j].normalStats.dur {
				return items[i].normalStats.dur > items[j].normalStats.dur
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "eff", "efficiency":
		sort.SliceStable(items, func(i, j int) bool {
			effI := (1.0 - items[i].normalStats.ssim) * float64(max(1, items[i].normalStats.dur.Microseconds()))
			effJ := (1.0 - items[j].normalStats.ssim) * float64(max(1, items[j].normalStats.dur.Microseconds()))
			if effI != effJ {
				return effI < effJ
			}
			return items[i].entry.name < items[j].entry.name
		})
	case "-eff", "-efficiency":
		sort.SliceStable(items, func(i, j int) bool {
			effI := (1.0 - items[i].normalStats.ssim) * float64(max(1, items[i].normalStats.dur.Microseconds()))
			effJ := (1.0 - items[j].normalStats.ssim) * float64(max(1, items[j].normalStats.dur.Microseconds()))
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

func renderSingleDemoItemRaw(leftImg, rightImg image.Image, width int, smart bool, entry renderModeEntry, termRows int) (*renderedDemoItem, error) {
	normalRaw, err := renderModePairRaw(leftImg, rightImg, width, entry.cfg, false, entry.name, termRows)
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
		smartRaw, err := renderModePairRaw(leftImg, rightImg, width, entry.cfg, true, entry.name, termRows)
		if err != nil {
			return nil, err
		}
		item.smartLines = smartRaw.pairLines
		item.smartStats = modeDemoStats{dur: smartRaw.dur, w: smartRaw.contentW}
		item.smartRaw = smartRaw
	}
	return item, nil
}

func computeDemoItemMetrics(leftImg, rightImg image.Image, item *renderedDemoItem, width int, smart bool) {
	item.normalStats.ssim = calcModePairSSIM(leftImg, rightImg, item.normalRaw, width)
	item.leftTitle = fmt.Sprintf("%s (%dms, w=%d, ssim=%.2f)", item.entry.name, item.normalStats.dur.Milliseconds(), item.normalStats.w, item.normalStats.ssim)
	if smart {
		item.smartStats.ssim = calcModePairSSIM(leftImg, rightImg, item.smartRaw, width)
		item.smartTitle = fmt.Sprintf("+smart (%dms, w=%d, ssim=%.2f)", item.smartStats.dur.Milliseconds(), item.smartStats.w, item.smartStats.ssim)
		colW := ansiLinesWidth(item.normalLines) + 4
		if len(item.leftTitle)+4 > colW {
			colW = len(item.leftTitle) + 4
		}
		item.colW = colW
	}
}

func singleImageCardLines(item *renderedSingleDemoItem, info bool, modeSpec spec.RenderModesSpec) []string {
	lines := make([]string, 0, 1+max(len(item.normalLines), len(item.smartLines))+5)
	lines = append(lines, fmt.Sprintf("%-*s%s", item.colW, item.leftTitle, item.smartTitle))
	for i := 0; i < max(len(item.normalLines), len(item.smartLines)); i++ {
		var left, right string
		if i < len(item.normalLines) {
			left = item.normalLines[i]
		}
		if i < len(item.smartLines) {
			right = item.smartLines[i]
		}
		lines = append(lines, fmt.Sprintf("%s    %s", padANSILine(left, item.colW-4), right))
	}
	if info {
		lines = append(lines, modeInfoLines(item.entry, modeSpec)...)
	}
	return lines
}

func pairDemoCardLines(item *renderedDemoItem, smart, info bool, modeSpec spec.RenderModesSpec) []string {
	lines := make([]string, 0, 1+max(len(item.normalLines), len(item.smartLines))+5)
	if !smart {
		lines = append(lines, item.leftTitle)
		lines = append(lines, item.normalLines...)
	} else {
		lines = append(lines, fmt.Sprintf("%-*s%s", item.colW, item.leftTitle, item.smartTitle))
		for i := 0; i < max(len(item.normalLines), len(item.smartLines)); i++ {
			var left, right string
			if i < len(item.normalLines) {
				left = item.normalLines[i]
			}
			if i < len(item.smartLines) {
				right = item.smartLines[i]
			}
			lines = append(lines, fmt.Sprintf("%s    %s", padANSILine(left, item.colW-4), right))
		}
	}
	if info {
		lines = append(lines, modeInfoLines(item.entry, modeSpec)...)
	}
	return lines
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
