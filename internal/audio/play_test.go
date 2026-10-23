package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/cati/internal/testutil"
)

// videoExts lists extensions treated as potential audio sources.
var videoExts = map[string]bool{
	".mp4":  true,
	".webm": true,
	".mkv":  true,
	".mov":  true,
	".avi":  true,
}

// isVideo reports whether path has a recognised video extension.
func isVideo(path string) bool {
	return videoExts[strings.ToLower(filepath.Ext(path))]
}

// TestProbeAudio scans asset directories for video files and prints a table of
// their audio stream info.  No playback — safe to run without -v.
func TestProbeAudio(t *testing.T) {
	dirs := []string{"../../assets", "../../assets/samples", "../../testdata"}

	found := false
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Logf("skip %s: %v", dir, err)
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if !isVideo(path) {
				continue
			}
			found = true
			t.Run(path, func(t *testing.T) {
				t.Parallel()
				testutil.RequireMedia(t, path, "ffprobe")
				info, err := Probe(path)
				if err != nil {
					t.Fatalf("%s: probe error: %v", path, err)
				}
				if info == nil {
					t.Logf("%-50s  no audio", path)
				} else {
					t.Logf("%-50s  %s  %d Hz  %dch",
						path, info.Codec, info.SampleRate, info.Channels)
				}
			})
		}
	}
	if !found {
		t.Log("no video files found in asset directories — probe test skipped")
	}
}

// TestPlayAudio finds one video file with audio and plays a short snippet.
// Runs only with -v; skipped otherwise to avoid blocking CI.
//
// Usage: go test ./internal/audio/ -v -run TestPlayAudio
func TestPlayAudio(t *testing.T) {
	if testing.Short() {
		t.Skip("real audio playback")
	}
	testutil.SkipInContainer(t, "audio playback")
	if !testing.Verbose() {
		t.Skip("only runs with -v")
	}

	if err := checkDeps(); err != nil {
		t.Skip(err)
	}

	dirs := []string{"../../assets", "../../assets/samples"}
	const snippetDuration = 250 * time.Millisecond

	found := false
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Logf("skip %s: %v", dir, err)
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if !isVideo(path) {
				continue
			}

			info, err := Probe(path)
			if err != nil {
				t.Logf("%s: probe error: %v", path, err)
				continue
			}
			if info == nil {
				t.Logf("%-40s  (no audio — skip)", filepath.Base(path))
				continue
			}

			found = true
			fmt.Printf("\n── %s ──\n", path)
			fmt.Printf("   codec=%s  rate=%d Hz  channels=%d\n",
				info.Codec, info.SampleRate, info.Channels)
			fmt.Printf("   playing %.2fs snippet …\n", snippetDuration.Seconds())

			ctx, cancel := context.WithTimeout(context.Background(), snippetDuration)
			player, err := Open(ctx, path, 0, 0)
			if err != nil {
				cancel()
				t.Errorf("%s: open: %v", path, err)
				continue
			}

			select {
			case <-player.Done():
			case <-ctx.Done():
				player.Stop()
			}
			cancel()

			if err := player.Err(); err != nil {
				t.Logf("   playback error: %v", err)
			} else {
				fmt.Printf("   done\n")
			}
			return
		}
	}

	if !found {
		t.Log("no video files with audio found — playback test skipped")
	}
}
