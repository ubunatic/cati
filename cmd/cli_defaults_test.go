package cmd

import (
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"ubunatic.com/cati/spec"
)

// envDefaultFlags take their default from the environment, not the spec.
var envDefaultFlags = map[string]bool{"fastpath": true}

func cliRoots() []*cobra.Command {
	return []*cobra.Command{New(), NewPlay(), NewBrowse()}
}

func walkFlags(c *cobra.Command, fn func(*cobra.Command, *pflag.Flag)) {
	c.LocalFlags().VisitAll(func(f *pflag.Flag) { fn(c, f) })
	for _, sub := range c.Commands() {
		walkFlags(sub, fn)
	}
}

func isZeroDefault(f *pflag.Flag) bool {
	switch f.DefValue {
	case "", "false", "0", "0s", "[]":
		return true
	}
	return false
}

// TestCLIDefaultsNotHardcoded builds the command trees without cli.yaml and
// requires every flag to start at its zero value: non-zero defaults belong in
// spec/cli.yaml. This also covers the missing-spec degradation path.
func TestCLIDefaultsNotHardcoded(t *testing.T) {
	cliSpecFS = fstest.MapFS{}
	defer func() { cliSpecFS = spec.FS }()
	for _, root := range cliRoots() {
		walkFlags(root, func(c *cobra.Command, f *pflag.Flag) {
			if envDefaultFlags[f.Name] {
				return
			}
			if !isZeroDefault(f) {
				t.Errorf("%s --%s has hardcoded default %q; move it to spec/cli.yaml defaults", c.CommandPath(), f.Name, f.DefValue)
			}
			if f.NoOptDefVal != "" && f.Value.Type() != "bool" {
				t.Errorf("%s --%s has hardcoded no-opt value %q; move it to spec/cli.yaml defaults", c.CommandPath(), f.Name, f.NoOptDefVal)
			}
		})
	}
}

// TestSpecCLIDefaultsIntegrity requires every spec default to name an
// existing command and flag, to parse, and to show up in the flag.
func TestSpecCLIDefaultsIntegrity(t *testing.T) {
	cli, err := spec.LoadCLI()
	if err != nil {
		t.Fatalf("LoadCLI: %v", err)
	}
	if len(cli.Defaults) == 0 {
		t.Fatal("cli.yaml defines no defaults")
	}
	used := map[string]bool{}
	for _, root := range cliRoots() {
		walkFlags(root, func(c *cobra.Command, f *pflag.Flag) {
			def, ok := cli.Defaults[c.CommandPath()][f.Name]
			if !ok {
				return
			}
			used[c.CommandPath()+" --"+f.Name] = true
			if def.Default != nil && (f.DefValue != *def.Default || f.Value.String() != *def.Default) {
				t.Errorf("%s --%s: default %q, value %q, want %q from cli.yaml", c.CommandPath(), f.Name, f.DefValue, f.Value.String(), *def.Default)
			}
			if def.Default == nil && def.NoOpt == "" {
				t.Errorf("%s --%s: empty spec default", c.CommandPath(), f.Name)
			}
			if def.NoOpt != "" && f.NoOptDefVal != def.NoOpt {
				t.Errorf("%s --%s: no-opt %q, want %q", c.CommandPath(), f.Name, f.NoOptDefVal, def.NoOpt)
			}
		})
	}
	for path, flags := range cli.Defaults {
		for name := range flags {
			if !used[path+" --"+name] {
				t.Errorf("cli.yaml defaults %q.%q names no existing flag", path, name)
			}
		}
	}
	if err := applyCLIDefaultsFrom(New(), "", cli.Defaults); err != nil {
		t.Errorf("applying spec defaults: %v", err)
	}
}

// TestCLIDefaultsInvalidSpecFails covers the loader contract: a present but
// invalid cli.yaml is an error, never silently replaced by zero defaults.
func TestCLIDefaultsInvalidSpecFails(t *testing.T) {
	cliSpecFS = fstest.MapFS{"cli.yaml": {Data: []byte("defaults:\n  cati modes glyphs:\n    grid: { default: eight }\n")}}
	defer func() { cliSpecFS = spec.FS }()
	root := New()
	root.SetArgs([]string{"modes", "glyphs"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "spec/cli.yaml") {
		t.Fatalf("Execute with invalid cli.yaml = %v, want spec/cli.yaml error", err)
	}
}
