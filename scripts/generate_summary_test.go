//go:build ignore

package main

import "testing"

const sampleMarkdown = `---
title: "Sample Page"
---

# My Sample Title

Some body text here.
`

func TestExtractH1(t *testing.T) {
	got := extractH1(sampleMarkdown)
	want := "My Sample Title"
	if got != want {
		t.Errorf("extractH1() = %q, want %q", got, want)
	}
}

func BenchmarkExtractH1(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = extractH1(sampleMarkdown)
	}
}
