// Package spec embeds all spec YAML files and their JSON schemas into the binary.
// Import this package to access spec files at runtime without filesystem access.
package spec

import (
	"embed"
	"io/fs"
)

//go:embed *.yaml schemas
var root embed.FS

// FS is the embedded spec filesystem. Paths are relative to the spec/ directory,
// e.g. fs.ReadFile(spec.FS, "buttons.yaml").
var FS fs.FS = root
