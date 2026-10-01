package halfblock

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"

	"ubunatic.com/cati/v1/core"
)

// LoadResult is the outcome of LoadImageAsync.
type LoadResult struct {
	Image image.Image
	Err   error
}

// LoadImageAsync starts an image load and returns a buffered channel that
// receives exactly one result and is then closed. Progress callbacks run on
// the loading goroutine.
func LoadImageAsync(ctx context.Context, path string, onProgress func(core.Progress)) <-chan LoadResult {
	result := make(chan LoadResult, 1)
	go func() {
		img, err := LoadImageContext(ctx, path, onProgress)
		result <- LoadResult{Image: img, Err: err}
		close(result)
	}()
	return result
}

// LoadImageContext loads an image while reporting progress. Raster decoding
// observes cancellation between reads. SVG and video loaders report their
// start and terminal states; their external decoders do not expose byte-level
// progress.
func LoadImageContext(ctx context.Context, path string, onProgress func(core.Progress)) (image.Image, error) {
	if ctx == nil {
		return nil, fmt.Errorf("load %s: nil context", path)
	}
	if err := ctx.Err(); err != nil {
		return nil, reportLoadError(onProgress, path, err)
	}
	reportProgress(onProgress, core.Progress{Stage: core.StageLoading, Message: path})
	if IsVideo(path) || IsSVG(path) {
		var img image.Image
		var err error
		if IsVideo(path) {
			img, err = loadVideoFrameContext(ctx, path)
		} else {
			img, err = rasterizeSVGContext(ctx, path)
		}
		if err != nil {
			return nil, reportLoadError(onProgress, path, err)
		}
		if err := ctx.Err(); err != nil {
			return nil, reportLoadError(onProgress, path, err)
		}
		reportProgress(onProgress, core.Progress{Stage: core.StageComplete, Ratio: 1, Current: 1, Total: 1, Message: path})
		return img, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, reportLoadError(onProgress, path, fmt.Errorf("open %s: %w", path, err))
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, reportLoadError(onProgress, path, fmt.Errorf("stat %s: %w", path, err))
	}
	total := info.Size()
	reader := &progressReader{ctx: ctx, r: f, total: total, callback: onProgress, path: path}
	reportProgress(onProgress, core.Progress{Stage: core.StageDecoding, Message: path, Total: int(total)})
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, reportLoadError(onProgress, path, fmt.Errorf("decode %s: %w", path, err))
	}
	if err := ctx.Err(); err != nil {
		return nil, reportLoadError(onProgress, path, err)
	}
	reportProgress(onProgress, core.Progress{Stage: core.StageComplete, Ratio: 1, Current: int(total), Total: int(total), Message: path})
	return img, nil
}

type progressReader struct {
	ctx      context.Context
	r        io.Reader
	total    int64
	read     int64
	callback func(core.Progress)
	path     string
}

func (r *progressReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	r.read += int64(n)
	if n > 0 && r.total > 0 {
		ratio := float64(r.read) / float64(r.total)
		if ratio > 1 {
			ratio = 1
		}
		reportProgress(r.callback, core.Progress{Stage: core.StageDecoding, Ratio: ratio, Current: int(r.read), Total: int(r.total), Message: r.path})
	}
	return n, err
}

func reportProgress(callback func(core.Progress), progress core.Progress) {
	if callback != nil {
		callback(progress)
	}
}

func reportLoadError(callback func(core.Progress), path string, err error) error {
	reportProgress(callback, core.Progress{Stage: core.StageError, Message: err.Error()})
	return err
}
