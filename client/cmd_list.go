package main

import (
	"context"
	"fmt"
	"os"
)

func cmdList(opts globalOpts, args []string) error {
	ctx := context.Background()
	c, cleanup, err := connect(ctx, opts.binaryPath, opts.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness-client list: %v\n", err)
		return err
	}
	defer cleanup()

	raw, err := callTool(ctx, c, "workspace_list", map[string]any{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness-client list: %v\n", err)
		return err
	}
	_ = raw
	return nil
}
