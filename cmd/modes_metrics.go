package cmd

import (
	"fmt"
	"image"
	"time"

	"ubunatic.com/cati/internal/metrics"
)

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

func renderModePairRaw(leftSrc, rightSrc image.Image, width int, cfg renderCfg, smart bool, name string, termRows int) (modePairRawResult, error) {
	start := time.Now()
	cfg.smart = smart
	left, _, err := smartPrepareSelected(leftSrc, width, termRows, cfg)
	if err != nil {
		return modePairRawResult{}, fmt.Errorf("fit %s left logo: %w", name, err)
	}
	right, rightW, err := smartPrepareSelected(rightSrc, width, termRows, cfg)
	if err != nil {
		return modePairRawResult{}, fmt.Errorf("fit %s right logo: %w", name, err)
	}
	leftLines, err := renderDemoLines(left, cfg)
	if err != nil {
		return modePairRawResult{}, fmt.Errorf("render %s left logo: %w", name, err)
	}
	rightLines, err := renderDemoLines(right, cfg)
	if err != nil {
		return modePairRawResult{}, fmt.Errorf("render %s right logo: %w", name, err)
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

// ssimTrimTransparentPadding controls whether SSIM evaluation crops out trailing
// transparent extension rows (appended by FitDims to snap to terminal cell boundaries).
//
// Why this option exists:
// Terminal render modes often have multi-pixel vertical cells (e.g. halfblock 1x2,
// quadblock 2x2, sextant 2x3, sparkline 4x8). When a source image height results in a
// fractional character row (e.g. 26.5 rows), FitDims snaps to a valid half-cell and appends
// extH transparent rows so the renderer can emit a partial block character (e.g. ▀).
//
// If SSIM evaluation scales the source reference across the full cell bounds including extH,
// the reference image contains genuine content at the bottom where the reconstruction is
// transparent/black. This incurs an artificial boundary penalty on the bottom row, unfairly
// suppressing the SSIM score of multi-pixel modes (like halfblock) relative to 1x1 modes
// (like fulls) which have extH=0.
//
// Trimming the trailing transparent extension rows aligns the reference downscale exactly
// with the active rendered content, giving a fair, distortion-free quality comparison.
var ssimTrimTransparentPadding = true

func countTrailingTransparentRows(img image.Image) int {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return 0
	}
	transpRows := 0
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		allTransp := true
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0 {
				allTransp = false
				break
			}
		}
		if !allTransp {
			break
		}
		transpRows++
	}
	return transpRows
}

func cropImageHeight(img image.Image, h int) image.Image {
	b := img.Bounds()
	if b.Dy() <= h {
		return img
	}
	if sub, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return sub.SubImage(image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+h))
	}
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), h))
	for y := 0; y < h; y++ {
		for x := 0; x < b.Dx(); x++ {
			out.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}

func calcSingleImageSSIM(src image.Image, prepared image.Image, cfg renderCfg, targetWidth int) float64 {
	rec := renderReconstruction(prepared, cfg)
	const cellSubW = 12
	const cellSubH = 24

	cols := renderedCellSize(prepared, cfg).Cols
	if cols <= 0 {
		cols = targetWidth
	}
	spec := cfg.viewSpec()
	numRows := max(1, prepared.Bounds().Dy()/spec.CellH)
	canonW := cols * cellSubW
	canonH := numRows * cellSubH

	if ssimTrimTransparentPadding {
		extRows := countTrailingTransparentRows(prepared)
		contentH := prepared.Bounds().Dy() - extRows
		if contentH > 0 && extRows > 0 {
			kY := cellSubH / spec.CellH
			if kY <= 0 {
				kY = 1
			}
			canonContentH := contentH * kY
			recContent := cropImageHeight(rec, contentH)
			ref := metrics.PyramidDownscale(src, canonW, canonContentH)
			upscaled := nnUpscale(recContent, canonW, canonContentH)
			return metrics.SSIMLuminance(ref, upscaled)
		}
	}

	ref := metrics.PyramidDownscale(src, canonW, canonH)
	upscaled := nnUpscale(rec, canonW, canonH)
	return metrics.SSIMLuminance(ref, upscaled)
}

func calcModePairSSIM(leftSrc, rightSrc image.Image, raw modePairRawResult, width int) float64 {
	ssimLeft := calcSingleImageSSIM(leftSrc, raw.left, raw.cfg, width)
	ssimRight := calcSingleImageSSIM(rightSrc, raw.right, raw.cfg, raw.contentW)
	return (ssimLeft + ssimRight) / 2.0
}

func renderModePair(cati, emojig image.Image, width int, cfg renderCfg, smart bool, name string) ([]string, modeDemoStats, error) {
	raw, err := renderModePairRaw(cati, emojig, width, cfg, smart, name, 7)
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
