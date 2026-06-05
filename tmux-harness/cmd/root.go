package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	configPath  string
	showVersion bool
)

const version = "0.1.0"

func Execute() error {
	flag.StringVar(&configPath, "config", "", "path to config JSON file")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Fprintf(os.Stderr, "tmux-harness %s\n", version)
		return nil
	}

	return run(configPath)
}

func run(_ string) error {
	s := server.NewMCPServer(
		"tmux-harness",
		version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	pingTool := mcp.NewTool("ping",
		mcp.WithDescription("No-op health check tool"),
	)
	s.AddTool(pingTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("pong"), nil
	})

	fmt.Fprintln(os.Stderr, "tmux-harness: starting MCP server over stdio")
	return server.ServeStdio(s)
}
