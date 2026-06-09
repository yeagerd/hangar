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
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "BRANCH")
	assert.Contains(t, out, "IDLE")
	assert.NotContains(t, out, "STATUS")
	assert.NotContains(t, out, "REPO")
}

func TestPrintTable_SingleEntry(t *testing.T) {
	var buf bytes.Buffer
	isIdle := true
	ws := []workspaceSummary{
		{
			ID:         "8e9691bc-0c72-4942-aba1-b301fef763e4",
			Name:       "my-workspace",
			Branch:     "feature-branch",
			IdleStatus: &isIdle,
			CreatedAt:  time.Date(2026, 6, 5, 13, 41, 0, 0, time.UTC),
		},
	}
	printTable(ws, &buf)
	out := buf.String()
	assert.Contains(t, out, "8e9691bc")
	assert.Contains(t, out, "my-workspace")
	assert.Contains(t, out, "feature-branch")
	assert.Contains(t, out, "yes")
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
	assert.Contains(t, out, "…")
	assert.NotContains(t, out, "this-is-a-very-long-workspace-name-that-exceeds-the-column-width")
}

func TestPrintTable_ArchivedWorkspace(t *testing.T) {
	var buf bytes.Buffer
	ws := []workspaceSummary{
		{
			ID:   "deadbeef-0000-0000-0000-000000000000",
			Name: "old-workspace",
		},
	}
	printTable(ws, &buf)
	out := buf.String()
	// Status is not shown in the table; the entry should still be listed by name.
	assert.Contains(t, out, "old-workspace")
}

func TestPrintTable_IdleColumn(t *testing.T) {
	var buf bytes.Buffer
	yes := true
	no := false
	ws := []workspaceSummary{
		{ID: "aaaa", Name: "idle-ws", IdleStatus: &yes},
		{ID: "bbbb", Name: "busy-ws", IdleStatus: &no},
		{ID: "cccc", Name: "unknown-ws"},
	}
	printTable(ws, &buf)
	out := buf.String()
	assert.Contains(t, out, "yes")
	assert.Contains(t, out, "no")
	assert.Contains(t, out, "-")
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

func TestPrintTable_NilIdleShowsDash(t *testing.T) {
	var buf bytes.Buffer
	ws := []workspaceSummary{
		{ID: "aabbccdd", Name: "test"},
	}
	printTable(ws, &buf)
	out := buf.String()
	lines := strings.Split(out, "\n")
	require.Greater(t, len(lines), 1)
	// nil IdleStatus and zero CreatedAt → "-" placeholders in the data row.
	assert.Contains(t, lines[1], "-")
}
