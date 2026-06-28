package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/ubunatic/cati/internal/quadblock"
)

// ── isImageFile ───────────────────────────────────────────────────────────────

func TestIsImageFile(t *testing.T) {
	t.Helper()
	cases := []struct {
		path string
		want bool
	}{
		{"photo.png", true},
		{"photo.PNG", true},
		{"photo.jpg", true},
		{"photo.JPEG", true},
		{"photo.jpeg", true},
		{"readme.txt", false},
		{"noext", false},
		{"archive.tar.gz", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isImageFile(tc.path); got != tc.want {
				t.Errorf("isImageFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// ── expandArgs ────────────────────────────────────────────────────────────────

func TestExpandArgs_Files(t *testing.T) {
	// Explicit file paths are returned as-is (stat must succeed → use real files).
	files := []string{
		"testdata/solid_red_4x4.png",
		"testdata/checkerboard_4x4.png",
	}
	got, err := expandArgs(files, false)
	if err != nil {
		t.Fatalf("expandArgs: %v", err)
	}
	if len(got) != len(files) {
		t.Errorf("got %d paths, want %d: %v", len(got), len(files), got)
	}
}

func TestExpandArgs_Directory_Flat(t *testing.T) {
	got, err := expandArgs([]string{"testdata"}, false)
	if err != nil {
		t.Fatalf("expandArgs: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one image from testdata/")
	}
	for _, p := range got {
		if !isImageFile(p) {
			t.Errorf("non-image file in result: %s", p)
		}
	}
}

func TestExpandArgs_Directory_Recursive(t *testing.T) {
	// Create a temp tree: root/sub/deep.png
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// Copy a real PNG into both levels.
	copyFile := func(src, dst string) {
		t.Helper()
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", dst, err)
		}
	}
	copyFile("testdata/solid_red_4x4.png", filepath.Join(root, "top.png"))
	copyFile("testdata/solid_red_4x4.png", filepath.Join(sub, "deep.png"))

	// Flat: should only return root-level file.
	flat, err := expandArgs([]string{root}, false)
	if err != nil {
		t.Fatalf("flat expandArgs: %v", err)
	}
	if len(flat) != 1 {
		t.Errorf("flat: expected 1 file, got %d: %v", len(flat), flat)
	}

	// Recursive: should return both.
	rec, err := expandArgs([]string{root}, true)
	if err != nil {
		t.Fatalf("recursive expandArgs: %v", err)
	}
	if len(rec) != 2 {
		t.Errorf("recursive: expected 2 files, got %d: %v", len(rec), rec)
	}
}

func TestExpandArgs_Deduplication(t *testing.T) {
	path := "testdata/solid_red_4x4.png"
	got, err := expandArgs([]string{path, path}, false)
	if err != nil {
		t.Fatalf("expandArgs: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected deduplication to 1 entry, got %d: %v", len(got), got)
	}
}

func TestExpandArgs_MissingFile(t *testing.T) {
	_, err := expandArgs([]string{"nonexistent.png"}, false)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestZoomFlagShorthand(t *testing.T) {
	cmd := New()
	// Test --zoom
	err := cmd.ParseFlags([]string{"--zoom", "w"})
	if err != nil {
		t.Fatalf("failed to parse --zoom: %v", err)
	}
	zoomVal, err := cmd.Flags().GetString("zoom")
	if err != nil {
		t.Fatalf("failed to get zoom flag: %v", err)
	}
	if zoomVal != "w" {
		t.Errorf("expected zoom flag to be 'w', got '%s'", zoomVal)
	}

	// Reset flags and test -z
	cmd = New()
	err = cmd.ParseFlags([]string{"-z", "h"})
	if err != nil {
		t.Fatalf("failed to parse -z: %v", err)
	}
	zoomVal, err = cmd.Flags().GetString("zoom")
	if err != nil {
		t.Fatalf("failed to get zoom flag: %v", err)
	}
	if zoomVal != "h" {
		t.Errorf("expected zoom flag to be 'h', got '%s'", zoomVal)
	}
}

func TestParseQuadMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		useQuad bool
		check   func(t *testing.T, opts quadblock.Options)
	}{
		{"off zero", "0", false, nil},
		{"default empty is splithalf", "", true, func(t *testing.T, opts quadblock.Options) {
			if !opts.SplitHalf {
				t.Fatal("empty quad mode should enable SplitHalf")
			}
		}},
		{"edge snap", "edge-snap", true, func(t *testing.T, opts quadblock.Options) {
			if !opts.EdgeSnap {
				t.Fatal("edge-snap mode should enable EdgeSnap")
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts, useQuad, err := parseQuadMode(tc.mode)
			if err != nil {
				t.Fatalf("parseQuadMode(%q): %v", tc.mode, err)
			}
			if useQuad != tc.useQuad {
				t.Fatalf("useQuad = %v, want %v", useQuad, tc.useQuad)
			}
			if tc.check != nil {
				tc.check(t, opts)
			}
		})
	}
}
