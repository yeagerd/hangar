package main

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractText_SingleTextContent(t *testing.T) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: `{"key":"value"}`},
		},
	}
	got := extractText(result)
	assert.Equal(t, `{"key":"value"}`, got)
}

func TestExtractText_MultipleTextContent(t *testing.T) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "hello "},
			mcp.TextContent{Type: "text", Text: "world"},
		},
	}
	got := extractText(result)
	assert.Equal(t, "hello world", got)
}

func TestExtractText_EmptyContent(t *testing.T) {
	result := &mcp.CallToolResult{}
	got := extractText(result)
	assert.Equal(t, "", got)
}

func TestExtractText_NonTextContent(t *testing.T) {
	// Non-text content items should be ignored.
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.ImageContent{Type: "image", Data: "abc", MIMEType: "image/png"},
		},
	}
	got := extractText(result)
	assert.Equal(t, "", got)
}

func TestExtractText_ErrorResponse(t *testing.T) {
	// Simulate a server-side error response.
	result := &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "workspace not found"},
		},
	}
	// extractText should still return the message (caller decides what to do with IsError).
	got := extractText(result)
	require.Equal(t, "workspace not found", got)
}
