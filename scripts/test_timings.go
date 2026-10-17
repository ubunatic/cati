//go:build ignore

// Summarize go test -json output: go run scripts/test_timings.go results.json.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

type event struct {
	Action  string
	Package string
	Test    string
	Elapsed float64
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run scripts/test_timings.go results.json")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	var events []event
	for {
		var e event
		if err := decoder.Decode(&e); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if e.Action == "pass" || e.Action == "fail" {
			events = append(events, e)
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Elapsed > events[j].Elapsed })
	for _, e := range events[:min(25, len(events))] {
		name := e.Test
		if name == "" {
			name = "(package)"
		}
		fmt.Printf("%7.3fs  %-4s  %s  %s\n", e.Elapsed, e.Action, e.Package, name)
	}
}
