//go:build !race

package testutil

// RaceFactor scales deadlines for external processes under race instrumentation.
const RaceFactor = 1
