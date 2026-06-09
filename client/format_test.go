package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	printTable(nil, &buf)
	out := buf.String()
	// Header should still be printed.
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "BRANCH")
}

func TestPrintTable_SingleEntry(t *testing.T) {
	var buf bytes.Buffer
	ws := []workspaceSummary{
		{
			ID:        "8e9691bc-0c72-4942-aba1-b301fef763e4",
			Name:      "my-workspace",
			Branch:    "feature-branch",
			RepoAlias: "default",
			CreatedAt: time.Date(2026, 6, 5, 13, 41, 0, 0, time.UTC),
		},
	}
	printTable(ws, &buf)
	out := buf.String()
	assert.Contains(t, out, "8e9691bc")
	assert.Contains(t, out, "my-workspace")
	assert.Contains(t, out, "feature-branch")
	assert.Contains(t, out, "default")
}

func TestPrintTable_LongNameTruncation(t *testing.T) {
	var buf bytes.Buffer
	ws := []workspaceSummary{
		{
			ID:   "abcdef01-1234-5678-9abc-def012345678",
			Name: "this-is-a-very-long-workspace-name-that-exceeds-the-column-width",
		},
	}
	printTable(ws, &buf)
	out := buf.String()
	// The name column should be truncated with "…".
	assert.Contains(t, out, "…")
	// Full name should NOT appear.
	assert.NotContains(t, out, "this-is-a-very-long-workspace-name-that-exceeds-the-column-width")
}

func TestPrintWorkspace(t *testing.T) {
	var buf bytes.Buffer
	ws := workspaceSummary{
		ID:           "8e9691bc-0c72-4942-aba1-b301fef763e4",
		Name:         "multi-repo-support",
		Branch:       "multi-repo-support",
		TmuxSession:  "harness-multi-repo-support",
		WorktreePath: "/Users/yeagerd/github/articulant/worktrees/multi-repo-support",
		CreatedAt:    time.Date(2026, 6, 5, 13, 41, 55, 0, time.UTC),
	}
	printWorkspace(ws, &buf)
	out := buf.String()
	require.Contains(t, out, "id:")
	require.Contains(t, out, ws.ID)
	require.Contains(t, out, "name:")
	require.Contains(t, out, ws.Name)
	require.Contains(t, out, "branch:")
	require.Contains(t, out, ws.Branch)
	require.Contains(t, out, "session:")
	require.Contains(t, out, ws.TmuxSession)
	require.Contains(t, out, "worktree:")
	require.Contains(t, out, ws.WorktreePath)
	require.Contains(t, out, "created:")
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hello", truncate("hello", 5))
	assert.Equal(t, "hell…", truncate("hello!", 5))
	assert.Equal(t, "…", truncate("ab", 1))
}

func TestPrintTable_NoRepo(t *testing.T) {
	var buf bytes.Buffer
	ws := []workspaceSummary{
		{ID: "aabbccdd", Name: "test"},
	}
	printTable(ws, &buf)
	out := buf.String()
	// No repo alias → should show "-".
	lines := strings.Split(out, "\n")
	require.Greater(t, len(lines), 1)
	assert.Contains(t, lines[1], "-")
}
