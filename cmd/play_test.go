package cmd

import (
	"testing"
)

func TestPlay_NoFrames(t *testing.T) {
	err := play([]string{}, 0, 0, 0)
	if err == nil {
		t.Error("expected error for empty frame list, got nil")
	}
}

func TestPlay_MissingFile(t *testing.T) {
	err := play([]string{"nonexistent.png"}, 0, 0, 0)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestPlay_MissingVideoFile(t *testing.T) {
	err := play([]string{"nonexistent.mp4"}, 0, 0, 0)
	if err == nil {
		t.Error("expected error for missing video file, got nil")
	}
}

func TestPlay_MixedVideoAndImage(t *testing.T) {
	err := playVideos([]string{"nonexistent.mp4", "testdata/solid_red_4x4.png"}, 0, 0, 0)
	if err == nil {
		t.Error("expected error for mixed video+image paths, got nil")
	}
}
