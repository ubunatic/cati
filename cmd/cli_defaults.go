package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/cati/spec"
)

// cliSpecFS is the spec source; tests swap it to build trees without defaults.
var cliSpecFS fs.FS = spec.FS

// applyCLIDefaults sets flag defaults from spec/cli.yaml on root and all its
// subcommands. Flags are registered with zero values in Go; the spec is the
// only source of non-zero defaults. A missing cli.yaml leaves the zero values;
// an invalid one makes every command of root fail before it runs.
func applyCLIDefaults(root *cobra.Command) *cobra.Command {
	err := loadAndApplyCLIDefaults(root, "")
	if err == nil {
		return root
	}
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("spec/cli.yaml: %w", err)
	}
	return root
}

// applyCLIDefaultsUnder is applyCLIDefaults for a subtree that is built before
// it is attached to its binary root; parent is that root's command path
// (e.g. "cati" for modesCommand). Errors are left to the root's
// applyCLIDefaults, which loads the same spec.
func applyCLIDefaultsUnder(root *cobra.Command, parent string) *cobra.Command {
	_ = loadAndApplyCLIDefaults(root, parent)
	return root
}

func loadAndApplyCLIDefaults(root *cobra.Command, parent string) error {
	cli, err := spec.LoadCLIFrom(cliSpecFS)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return applyCLIDefaultsFrom(root, parent, cli.Defaults)
}

func applyCLIDefaultsFrom(root *cobra.Command, parent string, defaults map[string]map[string]spec.CLIFlagDefault) error {
	var firstErr error
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		path := strings.TrimSpace(parent + " " + c.CommandPath())
		for name, def := range defaults[path] {
			f := c.Flags().Lookup(name)
			if f == nil {
				continue // reported by TestSpecCLIDefaultsIntegrity
			}
			if def.Default != nil {
				if err := f.Value.Set(*def.Default); err != nil && firstErr == nil {
					firstErr = fmt.Errorf("default of %s --%s: %w", path, name, err)
					continue
				}
				f.DefValue = f.Value.String()
			}
			if def.NoOpt != "" {
				f.NoOptDefVal = def.NoOpt
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	return firstErr
}
