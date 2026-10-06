package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"ubunatic.com/cati/v1/core"
)

func TestFastpathFlag(t *testing.T) {
	previous := core.Fastpath
	t.Cleanup(func() { core.Fastpath = previous })
	for _, factory := range []struct {
		name string
		new  func() *cobra.Command
	}{{"cati", New}, {"catiplay", NewPlay}, {"catibrowse", NewBrowse}} {
		for _, initial := range []bool{false, true} {
			for _, tc := range []struct {
				name string
				args []string
				want bool
			}{
				{"omitted", nil, initial},
				{"enable", []string{"--fastpath=true"}, true},
				{"disable", []string{"--fastpath=false"}, false},
				{"bare", []string{"--fastpath"}, true},
			} {
				t.Run(factory.name+"/"+tc.name+"/"+map[bool]string{false: "off", true: "on"}[initial], func(t *testing.T) {
					core.Fastpath = initial
					command := factory.new()
					command.SetOut(io.Discard)
					command.SetErr(io.Discard)
					command.SetArgs(tc.args)
					command.RunE = func(*cobra.Command, []string) error {
						if core.Fastpath != tc.want {
							t.Errorf("fastpath = %v, want %v before rendering", core.Fastpath, tc.want)
						}
						return nil
					}
					if err := command.Execute(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestFastpathFlagOnModesSubcommand(t *testing.T) {
	previous := core.Fastpath
	t.Cleanup(func() { core.Fastpath = previous })
	core.Fastpath = true
	root := New()
	command, _, err := root.Find([]string{"modes"})
	if err != nil {
		t.Fatal(err)
	}
	command.RunE = func(*cobra.Command, []string) error {
		if core.Fastpath {
			t.Error("inherited flag did not disable fastpath")
		}
		return nil
	}
	root.SetArgs([]string{"modes", "--fastpath=false"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestForwardCommandInheritsFastpath(t *testing.T) {
	previous := core.Fastpath
	t.Cleanup(func() { core.Fastpath = previous })
	dir := t.TempDir()
	output := filepath.Join(dir, "fastpath")
	program := filepath.Join(dir, "probe")
	if err := os.WriteFile(program, []byte("#!/bin/sh\nprintf '%s' \"$CATI_FASTPATH\" > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		core.Fastpath = enabled
		want, inherited := "0", "1"
		if enabled {
			want, inherited = "1", "0"
		}
		t.Setenv("CATI_FASTPATH", inherited)
		if err := forwardCommand(program, []string{output}); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("child fastpath = %q, want %q", got, want)
		}
	}
}
