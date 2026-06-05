package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// connect spawns tmux-harness as a subprocess and returns an initialized MCP client.
// cleanup must be called when done to kill the subprocess and release resources.
func connect(ctx context.Context, binary, configPath string) (*mcpclient.Client, func(), error) {
	binary, err := resolveBinary(binary)
	if err != nil {
		return nil, nil, err
	}

	args := []string{}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}

	c, err := mcpclient.NewStdioMCPClient(binary, nil, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("starting tmux-harness: %w", err)
	}

	// Forward subprocess stderr to our stderr.
	if r, ok := mcpclient.GetStderr(c); ok {
		go func() {
			_, _ = io.Copy(os.Stderr, r)
		}()
	}

	cleanup := func() {
		_ = c.Close()
	}

	_, err = c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "harness-client",
				Version: clientVersion,
			},
		},
	})
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("initializing MCP client: %w", err)
	}

	return c, cleanup, nil
}

// resolveBinary finds the tmux-harness binary. Prefers the given path, then
// bin/tmux-harness relative to the running executable, then $PATH.
func resolveBinary(path string) (string, error) {
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("binary not found: %s", path)
		}
		return path, nil
	}

	// Try bin/tmux-harness relative to the running executable.
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "tmux-harness")
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Fall back to $PATH.
	found, err := exec.LookPath("tmux-harness")
	if err != nil {
		return "", fmt.Errorf("tmux-harness binary not found in PATH or next to harness-client; use --binary to specify")
	}
	return found, nil
}
