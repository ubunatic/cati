package spec

import (
	"testing"
	"testing/fstest"
)

func TestLoadControls_Success(t *testing.T) {
	spec, err := LoadControls()
	if err != nil {
		t.Fatalf("LoadControls() unexpected error: %v", err)
	}

	if len(spec.Controls) == 0 {
		t.Fatal("LoadControls() returned empty controls map")
	}

	expectedControls := []string{
		"preview_height",
		"view_mode",
		"preview_videos",
		"max_jobs",
		"video_frames",
		"video_preview_delay",
	}

	for _, name := range expectedControls {
		ctrl, ok := spec.Controls[name]
		if !ok {
			t.Errorf("LoadControls() missing control %q", name)
			continue
		}
		if ctrl.Type == "" {
			t.Errorf("LoadControls() control %q has empty Type", name)
		}
		if ctrl.Set == "" {
			t.Errorf("LoadControls() control %q has empty Set handler", name)
		}
		if ctrl.Get == "" {
			t.Errorf("LoadControls() control %q has empty Get handler", name)
		}
	}
}

func TestLoadControls_MissingFile(t *testing.T) {
	origFS := FS
	defer func() { FS = origFS }()

	FS = fstest.MapFS{}

	_, err := LoadControls()
	if err == nil {
		t.Error("LoadControls() expected error for missing controls.yaml, got nil")
	}
}

func TestLoadControls_InvalidYAML(t *testing.T) {
	origFS := FS
	defer func() { FS = origFS }()

	FS = fstest.MapFS{
		"controls.yaml": &fstest.MapFile{
			Data: []byte("controls: [invalid yaml"),
		},
	}

	_, err := LoadControls()
	if err == nil {
		t.Error("LoadControls() expected error for invalid YAML, got nil")
	}
}
