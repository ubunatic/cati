package core

import "sync/atomic"

// ProgressReporter reports completed rendering work. Construct it only when a
// callback is configured so the nil-callback path has no atomic accounting.
type ProgressReporter struct {
	callback func(Progress)
	total    int64
	current  atomic.Int64
}

// NewProgressReporter creates a reporter, or returns nil when callback is nil.
func NewProgressReporter(callback func(Progress), total int) *ProgressReporter {
	if callback == nil {
		return nil
	}
	if total < 0 {
		total = 0
	}
	return &ProgressReporter{callback: callback, total: int64(total)}
}

// Done reports one completed unit of work. Callbacks may run concurrently.
func (r *ProgressReporter) Done() {
	current := r.current.Add(1)
	total := r.total
	if total > 0 && current > total {
		current = total
	}
	if total > 0 && current > total {
		current = total
	}
	ratio := float64(1)
	if total > 0 {
		ratio = float64(current) / float64(total)
		if ratio < 0 {
			ratio = 0
		} else if ratio > 1 {
			ratio = 1
		}
	}
	r.callback(Progress{Stage: StageRendering, Current: int(current), Total: int(total), Ratio: ratio})
}
