package core

import (
	"os"
	"runtime"
	"strconv"
)

// maxWorkers caps parallel render workers. It is read once from
// CATI_MAX_WORKERS; unset, invalid, or <1 means runtime.NumCPU().
var maxWorkers = func() int {
	n := runtime.NumCPU()
	if v, err := strconv.Atoi(os.Getenv("CATI_MAX_WORKERS")); err == nil && v > 0 {
		n = min(n, v)
	}
	return n
}()

// MaxWorkers returns the worker ceiling for parallel renderers.
func MaxWorkers() int { return maxWorkers }
