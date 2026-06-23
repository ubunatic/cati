package cmd

import (
	"testing"
)

func TestPlay_NoFrames(t *testing.T) {
	err := play([]string{}, 15)
	if err == nil {
		t.Error("expected error for empty frame list, got nil")
	}
}

func TestPlay_BadFPS(t *testing.T) {
	err := play([]string{"../testdata/solid_red_4x4.png"}, 0)
	if err == nil {
		t.Error("expected error for fps=0, got nil")
	}

	err = play([]string{"../testdata/solid_red_4x4.png"}, -1)
	if err == nil {
		t.Error("expected error for fps=-1, got nil")
	}
}

func TestPlay_MissingFile(t *testing.T) {
	err := play([]string{"nonexistent.png"}, 15)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
