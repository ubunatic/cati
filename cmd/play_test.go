//go:build catiplay

package cmd

import (
	"testing"
)

func TestPlay_NoFrames(t *testing.T) {
	err := play([]string{}, 0, 0, 0, renderCfg{}, TimeRange{}, cropSpec{}, "default", "once")
	if err == nil {
		t.Error("expected error for empty frame list, got nil")
	}
}

func TestPlay_MissingFile(t *testing.T) {
	err := play([]string{"nonexistent.png"}, 0, 0, 0, renderCfg{}, TimeRange{}, cropSpec{}, "default", "once")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestPlay_MissingVideoFile(t *testing.T) {
	err := play([]string{"nonexistent.mp4"}, 0, 0, 0, renderCfg{}, TimeRange{}, cropSpec{}, "default", "once")
	if err == nil {
		t.Error("expected error for missing video file, got nil")
	}
}

func TestPlay_MixedVideoAndImage(t *testing.T) {
	err := playVideos([]string{"nonexistent.mp4", "testdata/solid_red_4x4.png"}, 0, 0, 0, renderCfg{}, TimeRange{}, cropSpec{}, "default", "once")
	if err == nil {
		t.Error("expected error for mixed video+image paths, got nil")
	}
}

func TestParsePlayMode(t *testing.T) {
	tests := []struct {
		name     string
		playVal  string
		changed  bool
		args     []string
		wantMode string
		wantArgs []string
		wantErr  bool
	}{
		{
			name:     "unchanged empty",
			playVal:  "",
			changed:  false,
			args:     []string{"file.png"},
			wantMode: "",
			wantArgs: []string{"file.png"},
			wantErr:  false,
		},
		{
			name:     "flag set without arg",
			playVal:  "once",
			changed:  true,
			args:     []string{"file.png"},
			wantMode: "once",
			wantArgs: []string{"file.png"},
			wantErr:  false,
		},
		{
			name:     "explicit repeat",
			playVal:  "repeat",
			changed:  true,
			args:     []string{"file.png"},
			wantMode: "repeat",
			wantArgs: []string{"file.png"},
			wantErr:  false,
		},
		{
			name:     "explicit preview",
			playVal:  "preview",
			changed:  true,
			args:     []string{"file.png"},
			wantMode: "preview",
			wantArgs: []string{"file.png"},
			wantErr:  false,
		},
		{
			name:     "space separated preview arg",
			playVal:  "once",
			changed:  true,
			args:     []string{"file.png", "preview"},
			wantMode: "preview",
			wantArgs: []string{"file.png"},
			wantErr:  false,
		},
		{
			name:     "invalid mode",
			playVal:  "invalid",
			changed:  true,
			args:     []string{"file.png"},
			wantMode: "",
			wantArgs: []string{"file.png"},
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{}, tc.args...)
			mode, err := parsePlayMode(tc.playVal, tc.changed, &args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parsePlayMode error = %v, wantErr %v", err, tc.wantErr)
			}
			if mode != tc.wantMode {
				t.Errorf("got mode %q, want %q", mode, tc.wantMode)
			}
			if len(args) != len(tc.wantArgs) {
				t.Fatalf("got args %v, want %v", args, tc.wantArgs)
			}
			for i := range args {
				if args[i] != tc.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, args[i], tc.wantArgs[i])
				}
			}
		})
	}
}
