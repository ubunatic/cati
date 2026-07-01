package cmd

import (
	"context"
	"image"
	"runtime"
	"sync"

	"ubunatic.com/cati/v1/halfblock"
)

type thumbKey struct {
	path string
	w, h int
}

type thumbResult struct {
	key    thumbKey
	frames []image.Image
}

type thumbJob struct {
	key     thumbKey
	isVideo bool
}

type thumbQueue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	jobs    []thumbJob
	pending map[thumbKey]bool
	done    bool
}

func newThumbQueue() *thumbQueue {
	q := &thumbQueue{pending: make(map[thumbKey]bool)}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *thumbQueue) submit(job thumbJob) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.done || q.pending[job.key] {
		return
	}
	q.pending[job.key] = true
	q.jobs = append(q.jobs, job)
	q.cond.Signal()
}

// prioritize moves jobs whose keys are in the given set to the front of the
// queue so they are processed next. Jobs already being processed are unaffected.
func (q *thumbQueue) prioritize(keys map[thumbKey]bool) {
	if len(keys) == 0 {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	var front, back []thumbJob
	for _, j := range q.jobs {
		if keys[j.key] {
			front = append(front, j)
		} else {
			back = append(back, j)
		}
	}
	q.jobs = append(front, back...)
}

func (q *thumbQueue) next() (thumbJob, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.jobs) == 0 && !q.done {
		q.cond.Wait()
	}
	if len(q.jobs) == 0 {
		return thumbJob{}, false
	}
	j := q.jobs[0]
	q.jobs = q.jobs[1:]
	return j, true
}

func (q *thumbQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

func (q *thumbQueue) stop() {
	q.mu.Lock()
	q.done = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

func startThumbWorkers(ctx context.Context, n int, q *thumbQueue, previewVideos bool, nVideoFrames int, rc renderCfg, results chan<- thumbResult) {
	if n <= 0 {
		n = max(1, runtime.NumCPU()/2)
	}
	for range n {
		go thumbWorker(ctx, q, previewVideos, nVideoFrames, rc, results)
	}
}

func resolveWorkerCount(override, configured int) int {
	if override > 0 {
		return override
	}
	if configured > 0 {
		return configured
	}
	return max(1, runtime.NumCPU()/2)
}

func thumbWorker(ctx context.Context, q *thumbQueue, previewVideos bool, nVideoFrames int, rc renderCfg, results chan<- thumbResult) {
	for {
		job, ok := q.next()
		if !ok {
			return
		}
		var frames []image.Image
		if job.isVideo {
			if previewVideos {
				frames = loadVideoThumbs(job.key.path, nVideoFrames, job.key.w, job.key.h, rc)
			}
		} else {
			img, err := halfblock.LoadImage(job.key.path)
			if err == nil && img != nil {
				frames = []image.Image{rc.scaleToFit(img, job.key.w, job.key.h)}
			}
		}
		if len(frames) > 0 {
			select {
			case results <- thumbResult{key: job.key, frames: frames}:
			case <-ctx.Done():
				return
			}
		}
	}
}

func loadVideoThumbs(path string, n, w, h int, rc renderCfg) []image.Image {
	dur, err := halfblock.ProbeVideoDuration(path)
	if err != nil || dur <= 0 || n <= 1 {
		if dur > 0 {
			// Use a frame 25% into the video — the first frame is often black.
			img, err := halfblock.LoadVideoFrameAt(path, dur*0.25)
			if err == nil {
				return []image.Image{rc.scaleToFit(img, w, h)}
			}
		}
		// Fallback: try the first frame anyway.
		img, err := halfblock.LoadVideoFrame(path)
		if err != nil {
			return nil
		}
		return []image.Image{rc.scaleToFit(img, w, h)}
	}
	var frames []image.Image
	for i := range n {
		// Offset from zero to avoid the first frame (often black).
		t := dur * float64(i+1) / float64(n+1)
		img, err := halfblock.LoadVideoFrameAt(path, t)
		if err != nil {
			continue
		}
		frames = append(frames, rc.scaleToFit(img, w, h))
	}
	return frames
}
