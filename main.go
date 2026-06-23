package main

import (
	"fmt"
	"os"

	"codeberg.org/ubunatic/cati/cmd"
)

func main() {
	if err := cmd.New().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
