package core

import "testing"

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
