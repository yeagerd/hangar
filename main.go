package main

import (
	"fmt"
	"os"

	"github.com/articulant/tmux-harness/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "tmux-harness:", err)
		os.Exit(1)
	}
}
