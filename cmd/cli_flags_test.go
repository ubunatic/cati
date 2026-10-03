package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateCommonFlags(t *testing.T) {
	tests := []struct {
		name           string
		width          int
		height         int
		aspect         string
		initialZoom    string
		pad            string
		wantErr        bool
		wantErrSubstr  string
		wantAspectNorm string
		wantZoomNorm   string
	}{
		{
			name:    "defaults",
			wantErr: false,
		},
		{
			name:    "valid dimensions",
			width:   80,
			height:  24,
			wantErr: false,
		},
		{
			name:          "negative width",
			width:         -1,
			wantErr:       true,
			wantErrSubstr: "--width",
		},
		{
			name:          "negative height",
			height:        -1,
			wantErr:       true,
			wantErrSubstr: "--height",
		},
		{
			name:           "valid aspect default with casing and whitespace",
			aspect:         "  DeFaUlT  ",
			wantErr:        false,
			wantAspectNorm: "default",
		},
		{
			name:           "valid aspect aligned with casing",
			aspect:         "ALIGNED",
			wantErr:        false,
			wantAspectNorm: "aligned",
		},
		{
			name:           "valid aspect contain",
			aspect:         "contain",
			wantErr:        false,
			wantAspectNorm: "contain",
		},
		{
			name:           "valid aspect fit",
			aspect:         "fit",
			wantErr:        false,
			wantAspectNorm: "fit",
		},
		{
			name:          "invalid aspect",
			aspect:        "bogus",
			wantErr:       true,
			wantErrSubstr: "--aspect",
		},
		{
			name:         "valid zoom 0",
			initialZoom:  "0",
			wantErr:      false,
			wantZoomNorm: "0",
		},
		{
			name:         "valid zoom 1",
			initialZoom:  "1",
			wantErr:      false,
			wantZoomNorm: "1",
		},
		{
			name:         "valid zoom 100%",
			initialZoom:  "100%",
			wantErr:      false,
			wantZoomNorm: "100%",
		},
		{
			name:         "valid zoom 1:1",
			initialZoom:  "1:1",
			wantErr:      false,
			wantZoomNorm: "1:1",
		},
		{
			name:         "valid zoom w uppercase with whitespace",
			initialZoom:  "  W  ",
			wantErr:      false,
			wantZoomNorm: "w",
		},
		{
			name:         "valid zoom h uppercase",
			initialZoom:  "H",
			wantErr:      false,
			wantZoomNorm: "h",
		},
		{
			name:          "invalid zoom string",
			initialZoom:   "bogus",
			wantErr:       true,
			wantErrSubstr: "--zoom",
		},
		{
			name:          "invalid zoom Inf",
			initialZoom:   "Inf",
			wantErr:       true,
			wantErrSubstr: "--zoom",
		},
		{
			name:          "invalid zoom Inf%",
			initialZoom:   "Inf%",
			wantErr:       true,
			wantErrSubstr: "--zoom",
		},
		{
			name:          "invalid zoom Inf:Inf",
			initialZoom:   "Inf:Inf",
			wantErr:       true,
			wantErrSubstr: "--zoom",
		},
		{
			name:          "invalid zoom zero denominator",
			initialZoom:   "1:0",
			wantErr:       true,
			wantErrSubstr: "--zoom",
		},
		{
			name:          "invalid zoom negative",
			initialZoom:   "-2",
			wantErr:       true,
			wantErrSubstr: "--zoom",
		},
		{
			name:          "invalid zoom overflow reciprocal",
			initialZoom:   "1e-320%",
			wantErr:       true,
			wantErrSubstr: "--zoom",
		},
		{
			name:    "valid pad",
			pad:     "0,1",
			wantErr: false,
		},
		{
			name:          "invalid pad format",
			pad:           "bad_pad",
			wantErr:       true,
			wantErrSubstr: "pad",
		},
		{
			name:          "invalid pad negative",
			pad:           "-1,0",
			wantErr:       true,
			wantErrSubstr: "pad",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			aspect := tc.aspect
			zoom := tc.initialZoom
			pad := tc.pad
			err := validateCommonFlags(tc.width, tc.height, &aspect, &zoom, &pad)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateCommonFlags(%d, %d, %q, %q, %q) err = %v, wantErr = %v",
					tc.width, tc.height, tc.aspect, tc.initialZoom, tc.pad, err, tc.wantErr)
			}
			if tc.wantErr && tc.wantErrSubstr != "" {
				if !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErrSubstr, err.Error())
				}
			}
			if !tc.wantErr {
				if tc.wantAspectNorm != "" && aspect != tc.wantAspectNorm {
					t.Fatalf("normalized aspect = %q, want %q", aspect, tc.wantAspectNorm)
				}
				if tc.wantZoomNorm != "" && zoom != tc.wantZoomNorm {
					t.Fatalf("normalized zoom = %q, want %q", zoom, tc.wantZoomNorm)
				}
			}
		})
	}
}

func TestCLICommandsRejectInvalidFlags(t *testing.T) {
	factories := []struct {
		name string
		fn   func() *cobra.Command
	}{
		{name: "cati", fn: New},
		{name: "catiplay", fn: NewPlay},
		{name: "catibrowse", fn: NewBrowse},
	}

	flagCases := []struct {
		name          string
		args          []string
		wantErrSubstr string
	}{
		{
			name:          "negative width",
			args:          []string{"testdata/solid_red_4x4.png", "-W", "-1"},
			wantErrSubstr: "--width",
		},
		{
			name:          "negative height",
			args:          []string{"testdata/solid_red_4x4.png", "-H", "-1"},
			wantErrSubstr: "--height",
		},
		{
			name:          "invalid aspect",
			args:          []string{"testdata/solid_red_4x4.png", "--aspect", "invalid_mode"},
			wantErrSubstr: "--aspect",
		},
		{
			name:          "invalid zoom",
			args:          []string{"testdata/solid_red_4x4.png", "--zoom", "invalid_zoom"},
			wantErrSubstr: "--zoom",
		},
		{
			name:          "invalid zoom Inf",
			args:          []string{"testdata/solid_red_4x4.png", "--zoom", "Inf"},
			wantErrSubstr: "--zoom",
		},
		{
			name:          "invalid pad",
			args:          []string{"testdata/solid_red_4x4.png", "--pad", "bad_format"},
			wantErrSubstr: "pad",
		},
	}

	for _, factory := range factories {
		for _, tc := range flagCases {
			testName := factory.name + "/" + tc.name
			t.Run(testName, func(t *testing.T) {
				cmd := factory.fn()
				cmd.SetArgs(tc.args)
				err := cmd.Execute()
				if err == nil {
					t.Fatalf("%s: expected error for args %v, got nil", testName, tc.args)
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("%s: expected error containing %q, got %q", testName, tc.wantErrSubstr, err.Error())
				}
			})
		}
	}
}

func TestBenchmarkModeRejectsInvalidCommonFlags(t *testing.T) {
	cmd := New()
	cmd.SetArgs([]string{"testdata/solid_red_4x4.png", "--bench", "--bench-budget", "1ms", "-W", "-1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for --bench with -W -1, got nil")
	}
	if !strings.Contains(err.Error(), "--width") {
		t.Fatalf("expected error containing '--width', got %q", err.Error())
	}
}
