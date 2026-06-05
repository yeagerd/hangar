package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, defaultClaudeCmd, cfg.ClaudeCmd)
	assert.Equal(t, defaultIdleThresholdMs, cfg.IdleThresholdMs)
	assert.Equal(t, defaultSessionPrefix, cfg.SessionPrefix)
	assert.Equal(t, defaultMaxWorkspaces, cfg.MaxWorkspaces)
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.json")
	require.NoError(t, err)
	assert.Equal(t, defaultClaudeCmd, cfg.ClaudeCmd)
}

func TestLoad_FromFile(t *testing.T) {
	// Neutralize any ambient env overrides so file values are observable.
	t.Setenv("HARNESS_REPO_PATH", "")
	t.Setenv("HARNESS_CLAUDE_CMD", "")
	t.Setenv("HARNESS_MAX_WORKSPACES", "")
	t.Setenv("HARNESS_SESSION_PREFIX", "")
	t.Setenv("HARNESS_IDLE_THRESHOLD_MS", "")

	tmp := t.TempDir()
	cfgData := map[string]interface{}{
		"repoPath":        "/some/repo",
		"claudeCmd":       "myclaude",
		"maxWorkspaces":   5,
		"sessionPrefix":   "test-",
		"idleThresholdMs": 3000,
	}
	data, err := json.Marshal(cfgData)
	require.NoError(t, err)
	cfgFile := filepath.Join(tmp, "config.json")
	require.NoError(t, os.WriteFile(cfgFile, data, 0o600))

	cfg, err := Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "/some/repo", cfg.RepoPath)
	assert.Equal(t, "myclaude", cfg.ClaudeCmd)
	assert.Equal(t, 5, cfg.MaxWorkspaces)
	assert.Equal(t, "test-", cfg.SessionPrefix)
	assert.Equal(t, 3000, cfg.IdleThresholdMs)
}

func TestLoad_ReposFromFile(t *testing.T) {
	tmp := t.TempDir()
	cfgData := map[string]interface{}{
		"repos": map[string]interface{}{
			"articulant": map[string]interface{}{
				"path":         "/some/repo",
				"worktreeRoot": "/some/worktrees",
			},
			"client-app": map[string]interface{}{
				"path": "/other/repo",
			},
		},
		"claudeCmd": "myclaude",
	}
	data, err := json.Marshal(cfgData)
	require.NoError(t, err)
	cfgFile := filepath.Join(tmp, "config.json")
	require.NoError(t, os.WriteFile(cfgFile, data, 0o600))

	cfg, err := Load(cfgFile)
	require.NoError(t, err)
	require.Len(t, cfg.Repos, 2)
	assert.Equal(t, "/some/repo", cfg.Repos["articulant"].Path)
	assert.Equal(t, "/some/worktrees", cfg.Repos["articulant"].WorktreeRoot)
	assert.Equal(t, "/other/repo", cfg.Repos["client-app"].Path)
	// WorktreeRoot derived from path since it was omitted.
	assert.Equal(t, filepath.Join(filepath.Dir("/other/repo"), "worktrees"), cfg.Repos["client-app"].WorktreeRoot)
	assert.Equal(t, "myclaude", cfg.ClaudeCmd)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("HARNESS_REPO_PATH", "/env/repo")
	t.Setenv("HARNESS_CLAUDE_CMD", "env-claude")
	t.Setenv("HARNESS_MAX_WORKSPACES", "7")
	t.Setenv("HARNESS_SESSION_PREFIX", "env-")
	t.Setenv("HARNESS_IDLE_THRESHOLD_MS", "9000")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "/env/repo", cfg.RepoPath)
	assert.Equal(t, "env-claude", cfg.ClaudeCmd)
	assert.Equal(t, 7, cfg.MaxWorkspaces)
	assert.Equal(t, "env-", cfg.SessionPrefix)
	assert.Equal(t, 9000, cfg.IdleThresholdMs)
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	tmp := t.TempDir()
	cfgData := map[string]interface{}{"claudeCmd": "file-claude"}
	data, _ := json.Marshal(cfgData)
	cfgFile := filepath.Join(tmp, "config.json")
	require.NoError(t, os.WriteFile(cfgFile, data, 0o600))

	t.Setenv("HARNESS_CLAUDE_CMD", "env-claude")

	cfg, err := Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "env-claude", cfg.ClaudeCmd)
}

func TestLoad_HarnessReposEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HARNESS_REPOS", "myrepo="+tmp+"/repo1,other="+tmp+"/repo2:/custom/wt")

	cfg, err := Load("")
	require.NoError(t, err)
	require.Len(t, cfg.Repos, 2)
	assert.Equal(t, tmp+"/repo1", cfg.Repos["myrepo"].Path)
	assert.Equal(t, filepath.Join(filepath.Dir(tmp+"/repo1"), "worktrees"), cfg.Repos["myrepo"].WorktreeRoot)
	assert.Equal(t, tmp+"/repo2", cfg.Repos["other"].Path)
	assert.Equal(t, "/custom/wt", cfg.Repos["other"].WorktreeRoot)
}

func TestLoad_HarnessReposEnvInvalidEntry(t *testing.T) {
	t.Setenv("HARNESS_REPOS", "noequals")
	_, err := Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HARNESS_REPOS")
}

func TestValidate_Valid(t *testing.T) {
	tmp := t.TempDir()
	repoPath := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755))

	cfg := &Config{
		RepoPath:      repoPath,
		WorktreeRoot:  filepath.Join(tmp, "worktrees"),
		StorePath:     filepath.Join(tmp, "store", "ws.json"),
		ClaudeCmd:     "claude",
		MaxWorkspaces: 10,
		SessionPrefix: "harness-",
	}
	require.NoError(t, Validate(cfg))
	_, err := os.Stat(cfg.WorktreeRoot)
	assert.NoError(t, err, "worktreeRoot should be created")
}

func TestValidate_MultiRepo(t *testing.T) {
	tmp := t.TempDir()
	repo1 := filepath.Join(tmp, "repo1")
	repo2 := filepath.Join(tmp, "repo2")
	require.NoError(t, os.MkdirAll(filepath.Join(repo1, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repo2, ".git"), 0o755))

	cfg := &Config{
		Repos: map[string]Repo{
			"one": {Path: repo1, WorktreeRoot: filepath.Join(tmp, "wt1")},
			"two": {Path: repo2, WorktreeRoot: filepath.Join(tmp, "wt2")},
		},
		StorePath:     filepath.Join(tmp, "store", "ws.json"),
		MaxWorkspaces: 10,
		SessionPrefix: "harness-",
	}
	require.NoError(t, Validate(cfg))
	_, err := os.Stat(filepath.Join(tmp, "wt1"))
	assert.NoError(t, err, "worktreeRoot for repo one should be created")
	_, err = os.Stat(filepath.Join(tmp, "wt2"))
	assert.NoError(t, err, "worktreeRoot for repo two should be created")
}

func TestValidate_BothReposAndRepoPath(t *testing.T) {
	tmp := t.TempDir()
	repoPath := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755))

	cfg := &Config{
		RepoPath: repoPath,
		Repos: map[string]Repo{
			"x": {Path: repoPath, WorktreeRoot: filepath.Join(tmp, "wt")},
		},
		StorePath:     filepath.Join(tmp, "ws.json"),
		MaxWorkspaces: 10,
	}
	err := Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repos and repoPath cannot both be set")
}

func TestValidate_InvalidRepoPath(t *testing.T) {
	cfg := &Config{
		RepoPath:      "/nonexistent/repo",
		WorktreeRoot:  "/tmp/wt",
		StorePath:     "/tmp/store/ws.json",
		MaxWorkspaces: 10,
	}
	err := Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestValidate_MissingGit(t *testing.T) {
	tmp := t.TempDir()
	repoPath := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoPath, 0o755))

	cfg := &Config{
		RepoPath:      repoPath,
		WorktreeRoot:  filepath.Join(tmp, "wt"),
		StorePath:     filepath.Join(tmp, "store", "ws.json"),
		MaxWorkspaces: 10,
	}
	err := Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), ".git")
}

func TestValidate_MaxWorkspacesBounds(t *testing.T) {
	tmp := t.TempDir()
	repoPath := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755))

	base := &Config{
		RepoPath:      repoPath,
		WorktreeRoot:  filepath.Join(tmp, "wt"),
		StorePath:     filepath.Join(tmp, "store", "ws.json"),
		SessionPrefix: "h-",
	}

	for _, bad := range []int{0, 51} {
		cfg := *base
		cfg.MaxWorkspaces = bad
		cfg.Repos = nil // reset synthesized repos between iterations
		assert.Error(t, Validate(&cfg))
	}
	for _, good := range []int{1, 25, 50} {
		cfg := *base
		cfg.MaxWorkspaces = good
		cfg.Repos = nil // reset synthesized repos between iterations
		assert.NoError(t, Validate(&cfg))
	}
}

func TestValidate_NoReposConfigured(t *testing.T) {
	cfg := &Config{MaxWorkspaces: 10}
	err := Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one repo must be configured")
}

// TestLegacyConfigShim verifies that a config with only repoPath synthesises a "default" entry.
func TestLegacyConfigShim(t *testing.T) {
	tmp := t.TempDir()
	repoPath := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755))

	cfg := &Config{
		RepoPath:      repoPath,
		WorktreeRoot:  filepath.Join(tmp, "worktrees"),
		StorePath:     filepath.Join(tmp, "store", "ws.json"),
		MaxWorkspaces: 10,
		SessionPrefix: "harness-",
	}
	require.NoError(t, Validate(cfg))
	require.NotNil(t, cfg.Repos)
	assert.Equal(t, repoPath, cfg.Repos["default"].Path)
	assert.Equal(t, filepath.Join(tmp, "worktrees"), cfg.Repos["default"].WorktreeRoot)
}
