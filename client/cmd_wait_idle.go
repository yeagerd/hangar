package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// waitIdleIDs implements flag.Value for repeated --id flags.
type waitIdleIDs []string

func (w *waitIdleIDs) String() string { return strings.Join(*w, ",") }
func (w *waitIdleIDs) Set(val string) error {
	*w = append(*w, val)
	return nil
}

func cmdWaitIdle(opts globalOpts, args []string) error {
	fs := flag.NewFlagSet("harness-client wait-idle", flag.ContinueOnError)
	var ids waitIdleIDs
	var mode string
	var timeoutMs int64
	fs.Var(&ids, "id", "workspace ID to watch (repeatable; at least one required)")
	fs.StringVar(&mode, "mode", "all", `wait mode: "all" (every workspace idle) or "any" (at least one idle)`)
	fs.Int64Var(&timeoutMs, "timeout-ms", 0, "maximum wait time in milliseconds (0 = server default of 10 min)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Positional args also treated as IDs for convenience.
	for _, a := range fs.Args() {
		ids = append(ids, a)
	}

	if len(ids) == 0 {
		fmt.Fprintf(os.Stderr, "harness-client wait-idle: at least one --id (or positional arg) is required\n")
		fs.Usage()
		os.Exit(1)
	}
	if mode != "all" && mode != "any" {
		fmt.Fprintf(os.Stderr, "harness-client wait-idle: mode must be \"all\" or \"any\"\n")
		os.Exit(1)
	}

	ctx := context.Background()
	c, cleanup, err := connect(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness-client wait-idle: %v\n", err)
		return err
	}
	defer cleanup()

	idsAny := make([]any, len(ids))
	for i, id := range ids {
		idsAny[i] = id
	}
	toolArgs := map[string]any{
		"ids":  idsAny,
		"mode": mode,
	}
	if timeoutMs > 0 {
		toolArgs["timeout_ms"] = timeoutMs
	}

	raw, err := callTool(ctx, c, "workspace_wait_idle", toolArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness-client wait-idle: %v\n", err)
		return err
	}

	if opts.jsonOut {
		return prettyPrint(raw)
	}

	var result struct {
		TimedOut bool                              `json:"timed_out"`
		Results  map[string]struct{ Idle bool `json:"idle"` } `json:"results"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		fmt.Fprintf(os.Stderr, "harness-client wait-idle: parsing response: %v\n", err)
		return err
	}

	for id, entry := range result.Results {
		status := "busy"
		if entry.Idle {
			status = "idle"
		}
		fmt.Printf("%s: %s\n", id, status)
	}
	if result.TimedOut {
		fmt.Fprintln(os.Stderr, "timed out")
		os.Exit(2)
	}
	return nil
}
