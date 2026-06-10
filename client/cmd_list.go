package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func cmdList(opts globalOpts, args []string) error {
	fs := flag.NewFlagSet("harness-client list", flag.ContinueOnError)
	var waitForIdle string
	var timeoutMs int64
	fs.StringVar(&waitForIdle, "wait-for-idle", "none", `block before returning: "none" (default), "any" (at least one idle), "all" (all idle)`)
	fs.Int64Var(&timeoutMs, "timeout-ms", 0, "max wait in milliseconds when --wait-for-idle is set (0 = server default of 10 min)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	c, cleanup, err := connect(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness-client list: %v\n", err)
		return err
	}
	defer cleanup()

	toolArgs := map[string]any{}
	if waitForIdle != "" && waitForIdle != "none" {
		toolArgs["wait_for_idle"] = waitForIdle
	}
	if timeoutMs > 0 {
		toolArgs["timeout_ms"] = timeoutMs
	}

	raw, err := callTool(ctx, c, "workspace_list", toolArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness-client list: %v\n", err)
		return err
	}

	if opts.jsonOut {
		return prettyPrint(raw)
	}

	var result listResult
	if err := json.Unmarshal(raw, &result); err != nil {
		fmt.Fprintf(os.Stderr, "harness-client list: parsing response: %v\n", err)
		return err
	}
	if result.TimedOut {
		fmt.Fprintf(os.Stderr, "harness-client list: warning: timed out waiting for idle\n")
	}
	printTable(result.Workspaces, os.Stdout)
	return nil
}
