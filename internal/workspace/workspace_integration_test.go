//go:build integration

package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/articulant/tmux-harness/internal/config"
	"github.com/articulant/tmux-harness/internal/store"
	"github.com/articulant/tmux-harness/internal/tmux"
	"github.com/articulant/tmux-harness/internal/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("HARNESS_INTEGRATION") != "1" {
		t.Skip("set HARNESS_INTEGRATION=1 to run integration tests")
	}
}

// setupTestRepo initialises a git repo with an initial commit.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	git := func(args ...string) {
		t.Helper()
		fullArgs := append([]string{"-C", dir}, args...)
		out, err := exec.Command("git", fullArgs...).CombinedOutput() //nolint:gosec
		require.NoError(t, err, "git %v: %s", args, out)
	}

	git("init")
	git("config", "user.email", "test@test.com")
	git("config", "user.name", "Test")
	readme := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("test"), 0o644))
	git("add", "README.md")
	git("commit", "-m", "init")

	return dir
}

func setupManager(t *testing.T, repoPath string) *Manager {
	t.Helper()
	wtRoot := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "workspaces.json")

	cfg := &config.Config{
		Repos: map[string]config.Repo{
			"default": {Path: repoPath, WorktreeRoot: wtRoot},
		},
		StorePath:       storePath,
		ClaudeCmd:       "echo", // use echo instead of real claude in tests
		IdleThresholdMs: 5000,
		SessionPrefix:   "harness-inttest-",
		MaxWorkspaces:   10,
	}

	s, err := store.NewStore(storePath)
	require.NoError(t, err)

	wtClients := map[string]*worktree.Client{
		"default": worktree.New(repoPath),
	}
	return New(tmux.New(), wtClients, s, cfg)
}

func TestIntegration_CreateAndArchive(t *testing.T) {
	skipIfNoIntegration(t)

	repoPath := setupTestRepo(t)
	m := setupManager(t, repoPath)
	ctx := context.Background()

	ws, err := m.Create(ctx, CreateOptions{Name: "inttest-basic"})
	require.NoError(t, err)
	assert.Equal(t, "inttest-basic", ws.Name)
	assert.Equal(t, store.StatusActive, ws.Status)
	assert.Equal(t, "default", ws.RepoAlias)

	t.Cleanup(func() {
		_, _ = m.Archive(ctx, ws.ID)
	})

	// Session should exist.
	exists, err := m.tmux.SessionExists(m.cfg.SessionPrefix, "inttest-basic")
	require.NoError(t, err)
	assert.True(t, exists)

	// Archive.
	archived, err := m.Archive(ctx, ws.ID)
	require.NoError(t, err)
	assert.Equal(t, store.StatusArchived, archived.Status)
	assert.NotNil(t, archived.ArchivedAt)

	// Session should be gone.
	exists, err = m.tmux.SessionExists(m.cfg.SessionPrefix, "inttest-basic")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestIntegration_Reconcile(t *testing.T) {
	skipIfNoIntegration(t)

	repoPath := setupTestRepo(t)
	m := setupManager(t, repoPath)
	ctx := context.Background()

	ws, err := m.Create(ctx, CreateOptions{Name: "inttest-orphan"})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = m.tmux.KillSession(ws.TmuxSession)
		_, _ = m.Archive(ctx, ws.ID)
	})

	// Kill session directly to simulate an orphan.
	require.NoError(t, m.tmux.KillSession(ws.TmuxSession))

	// Reconcile should mark it as orphaned.
	require.NoError(t, m.Reconcile(ctx))

	reloaded, err := m.store.Get(ws.ID)
	require.NoError(t, err)
	assert.Equal(t, store.StatusOrphaned, reloaded.Status)
}

func TestIntegration_CapacityLimit(t *testing.T) {
	skipIfNoIntegration(t)

	repoPath := setupTestRepo(t)
	wtRoot := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "ws.json")

	cfg := &config.Config{
		Repos: map[string]config.Repo{
			"default": {Path: repoPath, WorktreeRoot: wtRoot},
		},
		StorePath:       storePath,
		ClaudeCmd:       "echo",
		IdleThresholdMs: 5000,
		SessionPrefix:   "harness-cap-",
		MaxWorkspaces:   1,
	}

	s, err := store.NewStore(storePath)
	require.NoError(t, err)
	wtClients := map[string]*worktree.Client{
		"default": worktree.New(repoPath),
	}
	m := New(tmux.New(), wtClients, s, cfg)
	ctx := context.Background()

	ws, err := m.Create(ctx, CreateOptions{Name: "cap-first"})
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Archive(ctx, ws.ID) })

	_, err = m.Create(ctx, CreateOptions{Name: "cap-second"})
	assert.ErrorIs(t, err, ErrCapacityReached)
}

func TestIntegration_MultiRepo(t *testing.T) {
	skipIfNoIntegration(t)

	repo1 := setupTestRepo(t)
	repo2 := setupTestRepo(t)
	wt1 := t.TempDir()
	wt2 := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "ws.json")

	cfg := &config.Config{
		Repos: map[string]config.Repo{
			"alpha": {Path: repo1, WorktreeRoot: wt1},
			"beta":  {Path: repo2, WorktreeRoot: wt2},
		},
		StorePath:       storePath,
		ClaudeCmd:       "echo",
		IdleThresholdMs: 5000,
		SessionPrefix:   "harness-mr-",
		MaxWorkspaces:   10,
	}

	s, err := store.NewStore(storePath)
	require.NoError(t, err)
	wtClients := map[string]*worktree.Client{
		"alpha": worktree.New(repo1),
		"beta":  worktree.New(repo2),
	}
	m := New(tmux.New(), wtClients, s, cfg)
	ctx := context.Background()

	// Create a workspace in each repo — same name is allowed across repos.
	wsAlpha, err := m.Create(ctx, CreateOptions{Name: "feat-x", Repo: "alpha"})
	require.NoError(t, err)
	assert.Equal(t, "alpha", wsAlpha.RepoAlias)
	assert.True(t, filepath.HasPrefix(wsAlpha.WorktreePath, wt1))

	wsBeta, err := m.Create(ctx, CreateOptions{Name: "feat-x", Repo: "beta"})
	require.NoError(t, err)
	assert.Equal(t, "beta", wsBeta.RepoAlias)
	assert.True(t, filepath.HasPrefix(wsBeta.WorktreePath, wt2))

	t.Cleanup(func() {
		_, _ = m.Archive(ctx, wsAlpha.ID)
		_, _ = m.Archive(ctx, wsBeta.ID)
	})

	// workspace_list with repo filter returns only that repo's workspaces.
	alphaList := m.List(false, "alpha")
	require.Len(t, alphaList, 1)
	assert.Equal(t, wsAlpha.ID, alphaList[0].ID)

	betaList := m.List(false, "beta")
	require.Len(t, betaList, 1)
	assert.Equal(t, wsBeta.ID, betaList[0].ID)

	allList := m.List(false, "")
	assert.Len(t, allList, 2)

	// workspace_delete removes the branch from the correct repo.
	// Remove the t.Cleanup entry for wsAlpha by deleting it explicitly here.
	require.NoError(t, m.Delete(ctx, wsAlpha.ID, true))

	// Branch should be gone from repo1.
	out, _ := exec.Command("git", "-C", repo1, "branch", "--list", "feat-x").Output()
	assert.Empty(t, strings.TrimSpace(string(out)), "branch feat-x should be deleted from repo1")

	// Branch in repo2 (beta) must be untouched.
	out2, err := exec.Command("git", "-C", repo2, "branch", "--list", "feat-x").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out2), "feat-x", "branch feat-x should still exist in repo2")

	// wsAlpha should be gone from the store.
	_, err = m.store.Get(wsAlpha.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}
