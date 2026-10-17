package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRequireMedia(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name     string
		data     string
		missing  bool
		commands []string
		wantSkip bool
	}{
		{name: "missing", missing: true, wantSkip: true},
		{name: "pointer", data: "version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 123\n", wantSkip: true},
		{name: "binary", data: "\x1a\x45\xdf\xa3\x00\xff"},
		{name: "malformed media must reach decoder", data: "broken video"},
		{name: "empty media must reach decoder"},
		{name: "missing tool", data: "video", commands: []string{"cati-nonexistent-media-test-tool"}, wantSkip: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name)
			if !tc.missing {
				if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var check *testing.T
			continued := false
			t.Run("prerequisites", func(t *testing.T) {
				check = t
				RequireMedia(t, path, tc.commands...)
				continued = true
			})
			if check.Skipped() != tc.wantSkip || continued == tc.wantSkip {
				t.Fatalf("skipped=%v continued=%v, want skip=%v", check.Skipped(), continued, tc.wantSkip)
			}
		})
	}
}

func TestSkipInContainer(t *testing.T) {
	t.Setenv("CATI_CONTAINER_TEST", "1")
	var check *testing.T
	t.Run("device-dependent test", func(t *testing.T) {
		check = t
		SkipInContainer(t, "host audio playback")
	})
	if !check.Skipped() {
		t.Fatal("SkipInContainer did not skip a container-only test")
	}
}
