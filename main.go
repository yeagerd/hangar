package main

import (
	"fmt"
	"os"

	"github.com/yeagerd/hangar/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "hangar:", err)
		os.Exit(1)
	}
}
