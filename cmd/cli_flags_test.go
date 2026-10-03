package cmd

import (
	"testing"
)

func TestValidateCommonFlags(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		aspect      string
		initialZoom string
		pad         string
		wantErr     bool
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
			name:    "negative width",
			width:   -1,
			wantErr: true,
		},
		{
			name:    "negative height",
			height:  -1,
			wantErr: true,
		},
		{
			name:    "valid aspect default",
			aspect:  "default",
			wantErr: false,
		},
		{
			name:    "valid aspect aligned",
			aspect:  "aligned",
			wantErr: false,
		},
		{
			name:    "invalid aspect",
			aspect:  "bogus",
			wantErr: true,
		},
		{
			name:        "valid zoom 0",
			initialZoom: "0",
			wantErr:     false,
		},
		{
			name:        "valid zoom 1",
			initialZoom: "1",
			wantErr:     false,
		},
		{
			name:        "valid zoom 100%",
			initialZoom: "100%",
			wantErr:     false,
		},
		{
			name:        "valid zoom 1:1",
			initialZoom: "1:1",
			wantErr:     false,
		},
		{
			name:        "valid zoom w",
			initialZoom: "w",
			wantErr:     false,
		},
		{
			name:        "valid zoom h",
			initialZoom: "h",
			wantErr:     false,
		},
		{
			name:        "invalid zoom",
			initialZoom: "bogus",
			wantErr:     true,
		},
		{
			name:    "valid pad",
			pad:     "0,1",
			wantErr: false,
		},
		{
			name:    "invalid pad",
			pad:     "bad_pad",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCommonFlags(tc.width, tc.height, tc.aspect, tc.initialZoom, tc.pad)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateCommonFlags(%d, %d, %q, %q, %q) err = %v, wantErr = %v",
					tc.width, tc.height, tc.aspect, tc.initialZoom, tc.pad, err, tc.wantErr)
			}
		})
	}
}

func TestCLICommandsRejectInvalidFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "negative width",
			args: []string{"testdata/solid_red_4x4.png", "-W", "-1"},
		},
		{
			name: "negative height",
			args: []string{"testdata/solid_red_4x4.png", "-H", "-1"},
		},
		{
			name: "invalid aspect",
			args: []string{"testdata/solid_red_4x4.png", "--aspect", "invalid_mode"},
		},
		{
			name: "invalid zoom",
			args: []string{"testdata/solid_red_4x4.png", "--zoom", "invalid_zoom"},
		},
		{
			name: "invalid pad",
			args: []string{"testdata/solid_red_4x4.png", "--pad", "bad_format"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := New()
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for args %v, got nil", tc.args)
			}
		})
	}
}
