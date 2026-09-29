package main

import (
	"image"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"ubunatic.com/cati/internal/imgutil"
	"ubunatic.com/cati/internal/metrics"
	"ubunatic.com/cati/spec"
	"ubunatic.com/cati/v1/core"
	"ubunatic.com/cati/v1/halfblock"
	"ubunatic.com/cati/v1/sextant"
	"ubunatic.com/cati/v1/sparkline"
)

// renderMode selects which block renderer is used.
type renderMode int

const (
	modeSix   renderMode = iota // sextant 2×3
	modeHalf                    // halfblock 1×2
	modeQuadP                   // quad+ composite 2×4 (quads + split-middle & quarter bars)
	modeBars                    // bars composite 4×4 (quads + fractional bars)
	modeAll                     // all composite 6×6 (quads + sextants + 3×3 ninelikes)
)

var modeNames = [...]string{"six", "half", "quad+", "bars", "all"}

func (m renderMode) String() string {
	if int(m) < len(modeNames) {
		return modeNames[m]
	}
	return "?"
}

func (m renderMode) Next() renderMode { return (m + 1) % (modeAll + 1) }

func (m renderMode) Prev() renderMode {
	if m == 0 {
		return modeAll
	}
	return m - 1
}

// renderReq is a single pending render job.
type renderReq struct {
	path string
	cols int
	rows int
	mode renderMode
	seq  uint64 // monotonically increasing; stale jobs are discarded
}

type renderStats struct {
	fileSize   int64
	origW      int
	origH      int
	cellW      int
	cellH      int
	subW       int
	subH       int
	scaleRatio float64
	gridW      int
	gridH      int
	dur        time.Duration
	ssim       float64
}

// renderRes is the result of a completed job.
type renderRes struct {
	path  string
	cols  int
	rows  int
	mode  renderMode
	seq   uint64
	grid  *core.Grid
	msg   string // non-empty on error / non-image
	dur   time.Duration
	ssim  float64
	stats renderStats
}

// renderer runs a single worker goroutine. Submitting a new request
// while one is pending replaces it; the in-flight render cannot be
// interrupted mid-execution but its result is discarded if superseded.
type renderer struct {
	seq  atomic.Uint64
	reqC chan renderReq
	resC chan renderRes
}

func newRenderer() *renderer {
	r := &renderer{
		reqC: make(chan renderReq, 1),
		resC: make(chan renderRes, 1),
	}
	go r.work()
	return r
}

// submit enqueues a render request, dropping any pending (not yet started) job.
func (r *renderer) submit(path string, cols, rows int, mode renderMode) {
	seq := r.seq.Add(1)
	req := renderReq{path: path, cols: cols, rows: rows, mode: mode, seq: seq}
	// Replace stale pending request.
	select {
	case <-r.reqC:
	default:
	}
	r.reqC <- req
}

// poll returns a completed result without blocking, or nil if none is ready.
func (r *renderer) poll() *renderRes {
	select {
	case res := <-r.resC:
		return &res
	default:
		return nil
	}
}

func (r *renderer) work() {
	for req := range r.reqC {
		// Skip if already superseded before we even start.
		if r.seq.Load() != req.seq {
			continue
		}
		start := time.Now()
		grid, ssim, stats, msg := doRender(req.path, req.cols, req.rows, req.mode)
		dur := time.Since(start)
		stats.dur = dur
		stats.ssim = ssim
		// Discard result if a newer request arrived while we rendered.
		if r.seq.Load() != req.seq {
			continue
		}
		res := renderRes{
			path: req.path, cols: req.cols, rows: req.rows, mode: req.mode, seq: req.seq,
			grid: grid, msg: msg, dur: dur, ssim: ssim, stats: stats,
		}
		// Replace stale result.
		select {
		case <-r.resC:
		default:
		}
		r.resC <- res
	}
}

type modeSpecCache struct {
	shapes []sparkline.Shape
	cellW  int
	cellH  int
}

var (
	cachedModesMu sync.RWMutex
	cachedModes   = make(map[string]modeSpecCache)
)

func getModeShapes(name string) ([]sparkline.Shape, int, int, error) {
	cachedModesMu.RLock()
	if c, ok := cachedModes[name]; ok {
		cachedModesMu.RUnlock()
		return c.shapes, c.cellW, c.cellH, nil
	}
	cachedModesMu.RUnlock()

	cachedModesMu.Lock()
	defer cachedModesMu.Unlock()
	if c, ok := cachedModes[name]; ok {
		return c.shapes, c.cellW, c.cellH, nil
	}

	res, err := spec.ResolveGlyphSetExpression(name)
	if err != nil {
		return nil, 0, 0, err
	}
	shapes := make([]sparkline.Shape, len(res.Shapes))
	for i, s := range res.Shapes {
		shapes[i] = sparkline.Shape{
			Ch:     s.Glyph,
			Width:  res.Geometry.W,
			Height: res.Geometry.H,
			Mask:   s.Mask,
		}
	}
	cachedModes[name] = modeSpecCache{
		shapes: shapes,
		cellW:  res.Geometry.W,
		cellH:  res.Geometry.H,
	}
	return shapes, res.Geometry.W, res.Geometry.H, nil
}

func computeSSIM(src, rec image.Image, cols, rows int) float64 {
	if src == nil || rec == nil {
		return 0
	}
	const cellSubW = 12
	const cellSubH = 24
	canonW := cols * cellSubW
	canonH := rows * cellSubH
	if canonW <= 0 || canonH <= 0 {
		return 0
	}
	ref := metrics.PyramidDownscale(src, canonW, canonH)
	upscaled := imgutil.ScaleNN(rec, canonW, canonH)
	return metrics.SSIMLuminance(ref, upscaled)
}

// renderFrame converts a single decoded image frame to *core.Grid without computing SSIM/stats.
func renderFrame(img image.Image, cols, rows int, mode renderMode) (*core.Grid, error) {
	jobs := runtime.NumCPU()
	switch mode {
	case modeHalf:
		return halfblock.RenderToGrid(img, cols, halfblock.Options{Jobs: jobs, Rows: rows})
	case modeSix:
		return sextant.RenderToGrid(img, cols, sextant.Options{Jobs: jobs, Rows: rows})
	case modeQuadP:
		shapes, cw, ch, sErr := getModeShapes("quad+")
		if sErr != nil {
			return nil, sErr
		}
		opts := sparkline.Options{CellW: cw, CellH: ch, Shapes: shapes, Jobs: jobs, Rows: rows}
		return sparkline.RenderToGrid(img, cols, opts)
	case modeBars:
		shapes, cw, ch, sErr := getModeShapes("bars")
		if sErr != nil {
			return nil, sErr
		}
		opts := sparkline.Options{CellW: cw, CellH: ch, Shapes: shapes, Jobs: jobs, Rows: rows}
		return sparkline.RenderToGrid(img, cols, opts)
	case modeAll:
		shapes, cw, ch, sErr := getModeShapes("all")
		if sErr != nil {
			return nil, sErr
		}
		opts := sparkline.Options{CellW: cw, CellH: ch, Shapes: shapes, Jobs: jobs, Rows: rows}
		return sparkline.RenderToGrid(img, cols, opts)
	}
	return halfblock.RenderToGrid(img, cols, halfblock.Options{Jobs: jobs, Rows: rows})
}

// doRender dispatches to the correct renderer package with multicore parallelism
// and computes the reconstruction SSIM score against the source image.
func doRender(path string, cols, rows int, mode renderMode) (*core.Grid, float64, renderStats, string) {
	var stats renderStats
	if fi, err := os.Stat(path); err == nil {
		stats.fileSize = fi.Size()
	}

	maxPixelW := cols * 2
	maxPixelH := cols * 4
	if rows > 0 && rows*4 > maxPixelH {
		maxPixelH = rows * 4
	}
	img, err := halfblock.LoadImageWithTarget(path, maxPixelW, maxPixelH)
	if err != nil {
		return nil, 0, stats, "Not an image: " + err.Error()
	}

	origW, origH := img.Bounds().Dx(), img.Bounds().Dy()
	stats.origW = origW
	stats.origH = origH

	jobs := runtime.NumCPU()
	var grid *core.Grid
	var rec image.Image

	switch mode {
	case modeHalf:
		stats.cellW, stats.cellH = 1, 2
		grid, err = halfblock.RenderToGrid(img, cols, halfblock.Options{Jobs: jobs, Rows: rows})
		scaled := halfblock.ScaleToFit(img, cols, rows)
		rec = halfblock.RenderToImageJ(scaled, jobs)
		if scaled != nil {
			stats.subW, stats.subH = scaled.Bounds().Dx(), scaled.Bounds().Dy()
		}
	case modeSix:
		stats.cellW, stats.cellH = 2, 3
		grid, err = sextant.RenderToGrid(img, cols, sextant.Options{Jobs: jobs, Rows: rows})
		scaled := sextant.ScaleToFit(img, cols, rows)
		rec = sextant.RenderToImageJ(scaled, sextant.ModeSextant, jobs)
		if scaled != nil {
			stats.subW, stats.subH = scaled.Bounds().Dx(), scaled.Bounds().Dy()
		}
	case modeQuadP:
		shapes, cw, ch, sErr := getModeShapes("quad+")
		if sErr != nil {
			return nil, 0, stats, "Resolve quad+ mode: " + sErr.Error()
		}
		stats.cellW, stats.cellH = cw, ch
		opts := sparkline.Options{
			CellW:  cw,
			CellH:  ch,
			Shapes: shapes,
			Jobs:   jobs,
			Rows:   rows,
		}
		grid, err = sparkline.RenderToGrid(img, cols, opts)
		if grid != nil {
			targetW, targetH, extH := imgutil.FitDims(img.Bounds().Dx(), img.Bounds().Dy(), cw, ch, 2*cw/ch, cols, rows)
			stats.subW, stats.subH = targetW, targetH
			scaled := imgutil.ScaleNN(img, targetW, targetH)
			if extH > 0 {
				scaled = imgutil.AppendTransparentRows(scaled, extH)
			}
			rec, _ = sparkline.RenderToImageWithOptionsJ(scaled, grid.Width, grid.Height, opts, jobs)
		}
	case modeBars:
		shapes, cw, ch, sErr := getModeShapes("bars")
		if sErr != nil {
			return nil, 0, stats, "Resolve bars mode: " + sErr.Error()
		}
		stats.cellW, stats.cellH = cw, ch
		opts := sparkline.Options{
			CellW:  cw,
			CellH:  ch,
			Shapes: shapes,
			Jobs:   jobs,
			Rows:   rows,
		}
		grid, err = sparkline.RenderToGrid(img, cols, opts)
		if grid != nil {
			targetW, targetH, extH := imgutil.FitDims(img.Bounds().Dx(), img.Bounds().Dy(), cw, ch, 2*cw/ch, cols, rows)
			stats.subW, stats.subH = targetW, targetH
			scaled := imgutil.ScaleNN(img, targetW, targetH)
			if extH > 0 {
				scaled = imgutil.AppendTransparentRows(scaled, extH)
			}
			rec, _ = sparkline.RenderToImageWithOptionsJ(scaled, grid.Width, grid.Height, opts, jobs)
		}
	case modeAll:
		shapes, cw, ch, sErr := getModeShapes("all")
		if sErr != nil {
			return nil, 0, stats, "Resolve all mode: " + sErr.Error()
		}
		stats.cellW, stats.cellH = cw, ch
		opts := sparkline.Options{
			CellW:  cw,
			CellH:  ch,
			Shapes: shapes,
			Jobs:   jobs,
			Rows:   rows,
		}
		grid, err = sparkline.RenderToGrid(img, cols, opts)
		if grid != nil {
			targetW, targetH, extH := imgutil.FitDims(img.Bounds().Dx(), img.Bounds().Dy(), cw, ch, 2*cw/ch, cols, rows)
			stats.subW, stats.subH = targetW, targetH
			scaled := imgutil.ScaleNN(img, targetW, targetH)
			if extH > 0 {
				scaled = imgutil.AppendTransparentRows(scaled, extH)
			}
			rec, _ = sparkline.RenderToImageWithOptionsJ(scaled, grid.Width, grid.Height, opts, jobs)
		}
	}

	if err != nil {
		return nil, 0, stats, "Render failed: " + err.Error()
	}
	ssim := 0.0
	if grid != nil && rec != nil {
		ssim = computeSSIM(img, rec, grid.Width, grid.Height)
		stats.gridW = grid.Width
		stats.gridH = grid.Height
	}
	if stats.subW > 0 && stats.origW > 0 {
		stats.scaleRatio = float64(stats.subW) / float64(stats.origW)
	}
	return grid, ssim, stats, ""
}
