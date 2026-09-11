package cmd

import (
	"image"
	"math"

	"ubunatic.com/cati/internal/metrics"
	spec "ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/quadblock"
	"ubunatic.com/cati/v1/sextant"
	"ubunatic.com/cati/v1/sparkline"
)

type smartCandidate struct {
	width      int
	pixelWidth int
	score      float64
}

func smartCandidateWidths(target int) []int {
	policy, ok := loadSmartPolicy()
	if !ok || target <= 0 {
		return nil
	}
	lower := max(1, int(math.Ceil(float64(target)*(1-policy.MaxReduction))))
	widths := make([]int, 0, target-lower+1)
	for w := target; w >= lower; w-- {
		widths = append(widths, w)
	}
	return widths
}

func chooseSmartCandidate(candidates []smartCandidate) (smartCandidate, bool) {
	if len(candidates) == 0 {
		return smartCandidate{}, false
	}
	winner := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.score > winner.score || (candidate.score == winner.score && candidate.width > winner.width) {
			winner = candidate
		}
	}
	return winner, true
}

func loadSmartPolicy() (spec.SmartRenderPolicy, bool) {
	policy, err := spec.LoadRenderModes()
	if err != nil || policy.Smart.Metric != "psnr" || (policy.Smart.Step != "terminal-column" && policy.Smart.Step != "native") || policy.Smart.TieBreak != "widest" || policy.Smart.MaxReduction <= 0 || policy.Smart.MaxReduction > 1 {
		return spec.SmartRenderPolicy{}, false
	}
	return policy.Smart, true
}

func nativeSmartStep(rc renderCfg) bool {
	_, ok := nativeSmartStepSize(rc)
	return ok
}

func nativeSmartStepSize(rc renderCfg) (int, bool) {
	if rc.useGlyphs() {
		return 1, true
	}
	rm, err := spec.LoadRenderModes()
	if err != nil {
		return 0, false
	}
	name := rcModeName(rc)
	for _, mode := range rm.Modes {
		if mode.Name == name {
			return mode.NativeStep, mode.SmartStep == "native" && mode.NativeStep > 0
		}
	}
	return 0, false
}

// NativeStep is measured in render pixels; cell width converts it to a
// fractional terminal-column step (for example, 1/4 for a 4-pixel cell).
func nativeSmartTerminalStep(rc renderCfg) (int, int, bool) {
	pixelStep, ok := nativeSmartStepSize(rc)
	if !ok {
		return 0, 0, false
	}
	cellW, _ := rc.renderCellSize()
	return pixelStep, max(1, cellW), true
}

func smartCandidatePixelWidths(target, step int) []int {
	policy, ok := loadSmartPolicy()
	if !ok || target <= 0 || step <= 0 {
		return nil
	}
	lower := max(1, int(math.Ceil(float64(target)*(1-policy.MaxReduction))))
	widths := make([]int, 0, target-lower+1)
	for width := target; width >= lower; width -= step {
		widths = append(widths, width)
	}
	if widths[len(widths)-1] != lower {
		widths = append(widths, lower)
	}
	return widths
}

func smartReference(src image.Image, targetW, targetH int, rc renderCfg) image.Image {
	return resizeRenderedImage(src, targetW, targetH, rc)
}

// smartScoreCandidates scores candidate reconstructions against one fixed
// reference. Padding is deliberately outside this function and cannot affect
// selection.
func smartScoreCandidates(reference image.Image, candidates map[int]image.Image) (smartCandidate, bool) {
	scored := make([]smartCandidate, 0, len(candidates))
	for width, rendered := range candidates {
		normalized := rendered
		rb := reference.Bounds()
		if normalized.Bounds().Dx() != rb.Dx() || normalized.Bounds().Dy() != rb.Dy() {
			normalized = resizeRenderedImage(normalized, rb.Dx(), rb.Dy(), renderCfg{prescaler: prescalePyramid})
		}
		scored = append(scored, smartCandidate{width: width, pixelWidth: width, score: metrics.PSNR(reference, normalized)})
	}
	return chooseSmartCandidate(scored)
}

// fitNativeWidthChecked keeps the source aspect while allowing a candidate
// between whole terminal-cell widths. Renderers still decide how that partial
// cell is represented; final terminal-column padding happens after selection.
func fitNativeWidthChecked(img image.Image, pixelWidth, termRows int, rc renderCfg) (image.Image, error) {
	if pixelWidth < 1 {
		return nil, nil
	}
	cellW, _ := rc.renderCellSize()
	cols := max(1, (pixelWidth+cellW-1)/cellW)
	fit, err := fitRenderedImageChecked(img, cols, termRows, rc)
	if err != nil {
		return nil, err
	}
	height := fit.Bounds().Dy()
	return resizeRenderedImage(img, pixelWidth, height, rc), nil
}

func renderReconstruction(img image.Image, rc renderCfg) image.Image {
	b := img.Bounds()
	switch {
	case rc.useGlyphs():
		cellW, cellH := rc.renderCellSize()
		result, err := sparkline.RenderToImageWithOptions(img, max(1, ceilDiv(b.Dx(), cellW)), max(1, ceilDiv(b.Dy(), cellH)), rc.glyphOptions())
		if err == nil {
			return result
		}
		return img
	case rc.useSextant():
		return sextant.RenderToImage(img, rc.sextantMode)
	case rc.useQuad():
		return quadblock.RenderToImage(img, rc.quadOpts)
	case rc.useSpark():
		spec := rc.viewSpec()
		return sparkline.RenderToImage(img, max(1, b.Dx()/spec.CellW), max(1, b.Dy()/spec.CellH), rc.sparkMode)
	default:
		return halfblock.RenderToImage(img)
	}
}

func padSmartImage(img image.Image, targetCols int, rc renderCfg) image.Image {
	cellW, _ := rc.renderCellSize()
	actual := renderedCellSize(img, rc).Cols
	if targetCols <= actual || cellW <= 0 {
		return img
	}
	left := (targetCols - actual) / 2
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, targetCols*cellW, b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			out.Set(left*cellW+x, y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}

func smartPrepare(orig image.Image, termCols, termRows int, rc renderCfg) (image.Image, error) {
	if !rc.smart {
		return fitRenderedImageChecked(orig, termCols, termRows, rc)
	}
	if rc.gray {
		orig = quadblock.ReduceColors(orig, rc.grayColors)
	}
	if termCols <= 0 {
		derived, err := fitRenderedImageChecked(orig, termCols, termRows, rc)
		if err != nil {
			return nil, err
		}
		termCols = renderedCellSize(derived, rc).Cols
		if termCols <= 0 {
			return derived, nil
		}
	}
	base, err := fitRenderedImageChecked(orig, termCols, termRows, rc)
	if err != nil {
		return nil, err
	}
	if base.Bounds().Dx() <= 0 || base.Bounds().Dy() <= 0 {
		return base, nil
	}
	ref := smartReference(orig, base.Bounds().Dx(), base.Bounds().Dy(), rc)
	candidates := make(map[int]image.Image)
	if nativeSmartStep(rc) {
		step, _, _ := nativeSmartTerminalStep(rc)
		for _, pixelWidth := range smartCandidatePixelWidths(base.Bounds().Dx(), step) {
			candidate, err := fitNativeWidthChecked(orig, pixelWidth, termRows, rc)
			if err != nil || candidate == nil {
				continue
			}
			candidates[pixelWidth] = renderReconstruction(candidate, rc)
		}
	} else {
		for _, width := range smartCandidateWidths(termCols) {
			candidate, err := fitRenderedImageChecked(orig, width, termRows, rc)
			if err != nil {
				continue
			}
			candidates[width] = renderReconstruction(candidate, rc)
		}
	}
	winner, ok := smartScoreCandidates(ref, candidates)
	if !ok {
		return base, nil
	}
	selected, err := fitRenderedImageChecked(orig, winner.width, termRows, rc)
	if nativeSmartStep(rc) {
		selected, err = fitNativeWidthChecked(orig, winner.width, termRows, rc)
	}
	if err != nil {
		return base, nil
	}
	return padSmartImage(selected, termCols, rc), nil
}
