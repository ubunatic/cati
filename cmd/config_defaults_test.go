package cmd

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	spec "ubunatic.com/cati/spec"
)

func TestConfigDefaultsFollowSpecAndUserOverridesWin(t *testing.T) {
	data, err := fs.ReadFile(fs.FS(spec.FS), "config.yaml")
	if err != nil {
		t.Fatalf("read embedded config spec: %v", err)
	}
	changed := strings.Replace(string(data), "preview_height: 40", "preview_height: 57", 1)
	if changed == string(data) {
		t.Fatal("could not find preview_height default to modify")
	}
	defaults, err := spec.LoadConfigDefaultsFrom(fstest.MapFS{"config.yaml": {Data: []byte(changed)}})
	if err != nil {
		t.Fatalf("load changed config defaults: %v", err)
	}
	settings := settingsFromConfigDefaults(defaults.Config)
	if settings.MaxPreviewHeight != 57 {
		t.Fatalf("MaxPreviewHeight = %d, want changed spec value 57", settings.MaxPreviewHeight)
	}

	overridden := applyConfigOverrides(settings, []byte("max_preview_height=73\npreview_videos=false\n"))
	if overridden.MaxPreviewHeight != 73 || overridden.PreviewVideos {
		t.Fatalf("user overrides not applied: %+v", overridden)
	}
	if overridden.MaxJobs != defaults.Config.MaxJobs || overridden.VideoFrames != defaults.Config.VideoFrames {
		t.Fatalf("unmodified defaults changed while applying overrides: %+v", overridden)
	}
}

func TestConfigDefaultsRejectMissingAndInvalidSpecValues(t *testing.T) {
	data, err := fs.ReadFile(fs.FS(spec.FS), "config.yaml")
	if err != nil {
		t.Fatalf("read embedded config spec: %v", err)
	}
	missing := strings.Replace(string(data), "  video_frames: 8\n", "", 1)
	if _, err := spec.LoadConfigDefaultsFrom(fstest.MapFS{"config.yaml": {Data: []byte(missing)}}); err == nil {
		t.Fatal("config loader accepted missing video_frames")
	}
	invalid := strings.Replace(string(data), "  max_jobs: 4\n", "  max_jobs: 0\n", 1)
	if _, err := spec.LoadConfigDefaultsFrom(fstest.MapFS{"config.yaml": {Data: []byte(invalid)}}); err == nil {
		t.Fatal("config loader accepted out-of-range max_jobs")
	}
}
