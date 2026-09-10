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
	width int
	score float64
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
	if err != nil || policy.Smart.Metric != "psnr" || policy.Smart.Step != "terminal-column" || policy.Smart.TieBreak != "widest" || policy.Smart.MaxReduction <= 0 || policy.Smart.MaxReduction > 1 {
		return spec.SmartRenderPolicy{}, false
	}
	return policy.Smart, true
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
		scored = append(scored, smartCandidate{width: width, score: metrics.PSNR(reference, normalized)})
	}
	return chooseSmartCandidate(scored)
}

func renderReconstruction(img image.Image, rc renderCfg) image.Image {
	b := img.Bounds()
	switch {
	case rc.mode.useSextant():
		return sextant.RenderToImage(img, rc.sextantMode)
	case rc.mode.useQuad():
		return quadblock.RenderToImage(img, rc.quadOpts)
	case rc.mode.useSpark():
		spec := rc.mode.viewSpec()
		return sparkline.RenderToImage(img, max(1, b.Dx()/spec.CellW), max(1, b.Dy()/spec.CellH), rc.sparkMode)
	default:
		return halfblock.RenderToImage(img)
	}
}

func padSmartImage(img image.Image, targetCols int, rc renderCfg) image.Image {
	cellW, _ := rc.mode.renderCellSize()
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
	for _, width := range smartCandidateWidths(termCols) {
		candidate, err := fitRenderedImageChecked(orig, width, termRows, rc)
		if err != nil {
			continue
		}
		candidates[width] = renderReconstruction(candidate, rc)
	}
	winner, ok := smartScoreCandidates(ref, candidates)
	if !ok {
		return base, nil
	}
	selected, err := fitRenderedImageChecked(orig, winner.width, termRows, rc)
	if err != nil {
		return base, nil
	}
	return padSmartImage(selected, termCols, rc), nil
}
