package v1_test

import (
	"testing"

	"ubunatic.com/cati/v1/core"
)

func assertRenderProgress(t *testing.T, events []core.Progress, total int) {
	t.Helper()
	if len(events) != total {
		t.Fatalf("got %d events, want %d", len(events), total)
	}
	maxCurrent := 0
	for _, p := range events {
		if p.Stage != core.StageRendering {
			t.Errorf("stage = %q, want %q", p.Stage, core.StageRendering)
		}
		if p.Total != total {
			t.Errorf("total = %d, want %d", p.Total, total)
		}
		if p.Ratio < 0 || p.Ratio > 1 {
			t.Errorf("ratio out of bounds: %v", p.Ratio)
		}
		if p.Current < 1 || p.Current > total {
			t.Errorf("current out of bounds: %d", p.Current)
		}
		if p.Current > maxCurrent {
			maxCurrent = p.Current
		}
		wantRatio := float64(p.Current) / float64(total)
		if p.Ratio != wantRatio {
			t.Errorf("ratio = %v, want current/total %v", p.Ratio, wantRatio)
		}
	}
	if maxCurrent != total {
		t.Errorf("maximum current = %d, want %d", maxCurrent, total)
	}
}
