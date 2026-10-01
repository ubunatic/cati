package core

import (
	"math"
	"sync"
	"testing"
)

func TestProgressStages(t *testing.T) {
	stages := []string{StageLoading, StageDecoding, StageRendering, StageComplete, StageError}
	want := []string{"loading", "decoding", "rendering", "complete", "error"}
	for i := range stages {
		if stages[i] != want[i] {
			t.Errorf("stage %d = %q, want %q", i, stages[i], want[i])
		}
	}
	p := Progress{Stage: StageLoading, Ratio: 0.25, Current: 1, Total: 4, Message: "loading"}
	if p.Stage != StageLoading || p.Ratio != 0.25 || p.Current != 1 || p.Total != 4 || p.Message != "loading" {
		t.Fatalf("Progress fields were not retained: %+v", p)
	}
}

func TestProgressReporterClampsRatioAndCurrent(t *testing.T) {
	var mu sync.Mutex
	var events []Progress
	reporter := NewProgressReporter(func(p Progress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	}, 1)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); reporter.Done() }()
	}
	wg.Wait()
	if len(events) != 16 {
		t.Fatalf("got %d callback events, want 16", len(events))
	}
	for _, p := range events {
		if p.Current != 1 || p.Total != 1 || p.Ratio != 1 || math.IsNaN(p.Ratio) {
			t.Errorf("progress not clamped: %+v", p)
		}
	}
}

func TestNewProgressReporterNilCallback(t *testing.T) {
	if reporter := NewProgressReporter(nil, 4); reporter != nil {
		t.Fatalf("nil callback reporter = %#v, want nil", reporter)
	}
}
