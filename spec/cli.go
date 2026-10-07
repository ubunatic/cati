package spec

import (
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

// CLIFlag documents a top-level flag and the Go handler that owns it.
type CLIFlag struct {
	Long        string `yaml:"long"`
	Description string `yaml:"description"`
	Handler     string `yaml:"handler"`
}

// CLIFlagDefault is the specced default of one flag. Default nil means the
// flag keeps its zero value; NoOpt is used when the flag is given without
// an argument.
type CLIFlagDefault struct {
	Default *string `yaml:"default"`
	NoOpt   string  `yaml:"no_opt"`
}

// CLISpec is the parsed cli.yaml.
type CLISpec struct {
	Schema string             `yaml:"$schema"`
	Flags  map[string]CLIFlag `yaml:"flags"`
	// Defaults maps a command path (e.g. "cati modes glyphs") to its flag defaults.
	Defaults map[string]map[string]CLIFlagDefault `yaml:"defaults"`
}

// LoadCLI reads the CLI flag spec.
func LoadCLI() (CLISpec, error) {
	return LoadCLIFrom(FS)
}

// LoadCLIFrom reads the CLI flag spec from fsys.
func LoadCLIFrom(fsys fs.FS) (CLISpec, error) {
	var spec CLISpec
	data, err := fs.ReadFile(fsys, "cli.yaml")
	if err != nil {
		return spec, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	err = decoder.Decode(&spec)
	return spec, err
}
