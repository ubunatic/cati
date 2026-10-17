// Package testutil provides shared prerequisites for media integration tests.
package testutil

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"
)

// SkipInContainer skips tests that require host devices unavailable to containers.
func SkipInContainer(t testing.TB, requirement string) {
	t.Helper()
	if os.Getenv("CATI_CONTAINER_TEST") == "1" {
		t.Skipf("container test skips %s", requirement)
	}
}

// RequireMedia skips tests whose media or external tools are unavailable.
// Other read errors fail the test; malformed media still reaches the decoder.
func RequireMedia(t testing.TB, path string, commands ...string) {
	t.Helper()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		t.Skipf("media fixture %s is missing", path)
	}
	if err != nil {
		t.Fatalf("open media fixture %s: %v", path, err)
	}
	defer f.Close()
	const signature = "version https://git-lfs.github.com/spec/v1"
	prefix := make([]byte, len(signature))
	n, err := io.ReadFull(f, prefix)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		t.Fatalf("read media fixture %s: %v", path, err)
	}
	if bytes.Equal(prefix[:n], []byte(signature)) {
		t.Skipf("media fixture %s is a Git LFS pointer; run git lfs pull", path)
	}
	for _, command := range commands {
		if _, err := exec.LookPath(command); err != nil {
			t.Skipf("media test requires %s: %v", command, err)
		}
	}
}
