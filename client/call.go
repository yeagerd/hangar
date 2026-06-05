package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// callTool invokes a named MCP tool and returns the raw JSON text of the first
// text content item in the response. Returns a non-nil error if the server
// reports an error or no text content is found.
func callTool(ctx context.Context, c *mcpclient.Client, name string, args map[string]any) (json.RawMessage, error) {
	result, err := c.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	if result.IsError {
		msg := extractText(result)
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("%s: server error: %s", name, msg)
	}

	text := extractText(result)
	if text == "" {
		return nil, fmt.Errorf("%s: empty response from server", name)
	}
	return json.RawMessage(text), nil
}

// extractText returns the concatenated text from all TextContent items in result.
func extractText(result *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
