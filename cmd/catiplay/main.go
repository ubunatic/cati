package main

import (
	"fmt"
	"os"

	"ubunatic.com/cati/cmd"
)

func main() {
	if err := cmd.NewPlay().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
