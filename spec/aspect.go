package spec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"

	"gopkg.in/yaml.v3"
)

// PixelAspectPolicy owns the fractional limits for integer pixel snapping.
type PixelAspectPolicy struct {
	MaxDistortion float64
	MaxPadding    float64
}

// LoadPixelAspectPolicy loads the embedded policy. A missing file disables
// optional snapping; an existing but malformed policy returns an error.
func LoadPixelAspectPolicy() (PixelAspectPolicy, error) {
	return loadPixelAspectPolicy(FS)
}

func loadPixelAspectPolicy(files fs.FS) (PixelAspectPolicy, error) {
	data, err := fs.ReadFile(files, "aspect.yaml")
	if errors.Is(err, fs.ErrNotExist) {
		return PixelAspectPolicy{}, nil
	}
	if err != nil {
		return PixelAspectPolicy{}, fmt.Errorf("aspect policy: %w", err)
	}
	var document struct {
		Schema string `yaml:"$schema"`
		Pixel  struct {
			MaxDistortion *float64 `yaml:"max_distortion"`
			MaxPadding    *float64 `yaml:"max_padding"`
		} `yaml:"pixel"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return PixelAspectPolicy{}, fmt.Errorf("aspect policy: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return PixelAspectPolicy{}, fmt.Errorf("aspect policy: expected a single YAML document")
	}
	if document.Pixel.MaxDistortion == nil || document.Pixel.MaxPadding == nil {
		return PixelAspectPolicy{}, fmt.Errorf("aspect policy: pixel.max_distortion and pixel.max_padding are required")
	}
	policy := PixelAspectPolicy{MaxDistortion: *document.Pixel.MaxDistortion, MaxPadding: *document.Pixel.MaxPadding}
	if math.IsNaN(policy.MaxDistortion) || math.IsInf(policy.MaxDistortion, 0) || policy.MaxDistortion < 0 || policy.MaxDistortion > 1 || math.IsNaN(policy.MaxPadding) || math.IsInf(policy.MaxPadding, 0) || policy.MaxPadding < 0 || policy.MaxPadding >= 1 {
		return PixelAspectPolicy{}, fmt.Errorf("aspect policy: max_distortion must be in [0,1] and max_padding in [0,1)")
	}
	return policy, nil
}
