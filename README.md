# tmux-harness

A Go MCP server that lets orchestrators (Claude Code, Hermes, cabinet) create and manage isolated Claude Code sessions. Each "workspace" is a git worktree in a dedicated directory, opened inside a named tmux session with a running Claude Code interactive shell.

---

## Prerequisites

| Tool | Minimum version |
|------|----------------|
| Go | 1.22 |
| tmux | 3.2 |
| git | 2.35 |
| claude (Claude Code CLI) | any |

---

## Build

```sh
make build           # produces ./tmux-harness
# or
go build -o tmux-harness .
```

---

## Configuration

Configuration can be supplied via a JSON file and/or environment variables. Environment variables always take priority.

### Multi-repo config (recommended)

Use the `repos` object to manage worktrees across multiple git repositories from a single binary instance:

```json
{
  "repos": {
    "articulant": {
      "path": "/home/alice/github/articulant/main",
      "worktreeRoot": "/home/alice/github/articulant/worktrees"
    },
    "client-app": {
      "path": "/home/alice/github/client-app"
    }
  },
  "storePath": "~/.config/tmux-harness/workspaces.json",
  "claudeCmd": "claude",
  "maxWorkspaces": 20
}
```

The `worktreeRoot` field inside each repo entry is optional and defaults to `<path>/../worktrees`.

### Single-repo config (legacy)

For a single repository the legacy `repoPath` / `worktreeRoot` top-level fields still work:

```json
{
  "repoPath": "/home/alice/myproject",
  "maxWorkspaces": 5,
  "idleThresholdMs": 8000
}
```

### Config fields

| Field | Env var | Default | Description |
|-------|---------|---------|-------------|
| `repos` | `HARNESS_REPOS` | — | Map of alias → repo entry (`path` + optional `worktreeRoot`) |
| `repoPath` *(deprecated)* | `HARNESS_REPO_PATH` | — | Single-repo path; synthesises a `"default"` repos entry |
| `worktreeRoot` *(deprecated)* | `HARNESS_WORKTREE_ROOT` | `<repoPath>/../worktrees` | Single-repo worktree root |
| `storePath` | `HARNESS_STORE_PATH` | `~/.config/tmux-harness/workspaces.json` | Path to the JSON workspace registry |
| `claudeCmd` | `HARNESS_CLAUDE_CMD` | `claude` | Command to launch Claude Code |
| `idleThresholdMs` | `HARNESS_IDLE_THRESHOLD_MS` | `5000` | Milliseconds of pane inactivity before a session is "idle" |
| `sessionPrefix` | `HARNESS_SESSION_PREFIX` | `harness-` | Prefix for tmux session names |
| `maxWorkspaces` | `HARNESS_MAX_WORKSPACES` | `10` | Hard cap on concurrent active workspaces (1–50) |

### `HARNESS_REPOS` env var format

`HARNESS_REPOS` accepts a comma-separated list of `alias=path` or `alias=path:worktreeRoot` pairs:

```
HARNESS_REPOS=articulant=/home/alice/github/articulant/main,client-app=/home/alice/github/client-app:/custom/worktrees
```

Entries set via `HARNESS_REPOS` override entries of the same alias from the config file.

---

## Registering with Claude Code

Add the server to your Claude Code MCP config (typically `~/.claude/mcp.json` or a per-project `.mcp.json`):

```json
{
  "mcpServers": {
    "tmux-harness": {
      "command": "/usr/local/bin/tmux-harness",
      "args": ["--config", "/home/alice/.config/tmux-harness/config.json"],
      "env": {
        "HARNESS_REPO_PATH": "/home/alice/myproject"
      }
    }
  }
}
```

The orchestrator Claude Code instance will then have access to all workspace tools.

---

## Registering with Hermes / cabinet

Generic stdio MCP entry (adjust `command` and `env` as needed):

```json
{
  "servers": [
    {
      "name": "tmux-harness",
      "transport": "stdio",
      "command": "/usr/local/bin/tmux-harness",
      "args": ["--config", "/home/alice/.config/tmux-harness/config.json"],
      "env": {
        "HARNESS_REPO_PATH": "/home/alice/myproject"
      }
    }
  ]
}
```

---

## Tool Reference

### `workspace_list`
List all workspaces. By default excludes archived and orphaned ones.

**Inputs:**
- `include_archived` (bool, optional, default false)
- `repo` (string, optional) — filter by repo alias; omit to list workspaces for all repos

**Output:** JSON array: `[{id, name, status, branch, tmuxSession, createdAt, worktreePath, repoAlias, repoPath}]`

---

### `workspace_create`
Create a new workspace: git worktree + tmux session + Claude Code instance.

**Inputs:**
- `name` (string, required) — lowercase alphanumeric and hyphens, 1–40 chars
- `branch` (string, optional) — git branch to create; defaults to `name`
- `repo` (string, optional) — alias of the repo to create the workspace in; defaults to the only configured repo when there is exactly one, otherwise required
- `meta` (object, optional) — freeform string key-value metadata

**Output:** Full workspace object as JSON (includes `repoAlias` and `repoPath`).

---

### `workspace_archive`
Gracefully shut down a workspace. Exits Claude Code, removes the worktree, retains the git branch.

**Inputs:** `id` (string, required)

**Output:** Updated workspace object with `status: "archived"`.

---

### `workspace_delete`
Permanently delete a workspace and its git branch. **Irreversible.**

**Inputs:**
- `id` (string, required)
- `confirm` (bool, required) — must be `true`

**Output:** `{"deleted": true, "id": "<id>"}`

---

### `workspace_send`
Send text to the Claude Code session in a workspace.

**Inputs:**
- `id` (string, required)
- `text` (string, required) — must not contain ASCII control characters (except `\n` and `\t`)
- `press_enter` (bool, optional, default true)

**Output:** `{"sent": true}`

**Guards:** Workspace must be active. Rate limited to 1 call per 200 ms per workspace.

---

### `workspace_read`
Capture recent terminal output from a workspace's tmux pane.

**Inputs:**
- `id` (string, required)
- `lines` (int, optional, default 200, max 2000)

**Output:** `{"content": "...", "captured_at": "<ISO 8601>"}`

---

### `workspace_idle`
Check whether a workspace is busy or idle.

**Inputs:**
- `id` (string, required)
- `threshold_ms` (int, optional) — override the configured default

**Output:** `{"Idle": bool, "LastChangedAt": "...", "ElapsedMs": N, "ThresholdMs": N}`

---

### `workspace_wait_idle`
Block until the workspace becomes idle or the timeout elapses. Polls pane output internally so the caller makes a single tool call instead of polling repeatedly.

**Inputs:**
- `id` (string, required)
- `timeout_ms` (int, optional, default `600000` = 10 min) — maximum time to wait
- `threshold_ms` (int, optional) — idle-stability threshold override
- `poll_interval_ms` (int, optional, default `500`) — how often to sample the pane

**Output (success — workspace became idle):**
```json
{
  "idle": true,
  "timed_out": false,
  "last_changed_at": "2026-06-05T11:30:00Z",
  "elapsed_ms": 5200,
  "threshold_ms": 5000
}
```

**Output (timeout elapsed before idle):**
```json
{
  "idle": false,
  "timed_out": true,
  "last_changed_at": "2026-06-05T11:29:55Z",
  "elapsed_ms": 600000,
  "threshold_ms": 5000
}
```

**Hard errors** (workspace not found, not active, or pane capture failure) are returned as MCP tool errors, not JSON.

---

### `workspace_attach_hint`
Return the shell command to attach to a workspace's tmux session.

**Inputs:** `id` (string, required)

**Output:** `{"command": "tmux attach-session -t harness-<name>"}`

---

## Busy/Idle Detection

The idle detector does **not** parse Claude Code's internal state. Instead:

1. Capture the last 200 lines of the tmux pane via `tmux capture-pane`.
2. SHA-256 hash the output.
3. If the hash changed since the last check → **busy** (hash + timestamp stored in the workspace registry).
4. If the hash is unchanged, compute elapsed ms since last change:
   - elapsed ≥ `threshold_ms` → **idle**
   - elapsed < `threshold_ms` → **busy**

**Tuning:** Increase `idleThresholdMs` if your Claude Code sessions take a long time to produce output between steps. Decrease it for faster polling in interactive use.

---

## Attaching to a Session Manually

Any workspace can be inspected or interacted with by a human at any time:

```sh
# Get the attach command via MCP:
workspace_attach_hint id=<workspace-id>

# Or directly (if you know the name):
tmux attach-session -t harness-<name>
```

The orchestrator and human can both interact with the session simultaneously.

---

## Two-Claude Setup

```
Orchestrator Claude Code (has tmux-harness MCP registered)
        │
        │  workspace_create / workspace_send / workspace_idle / workspace_read / workspace_archive
        ▼
  tmux-harness binary
        │
        ├── git worktrees (one per workspace)
        └── tmux sessions (one per workspace, named harness-<name>)
                └── Worker Claude Code instances (one per session)

Human ──► tmux attach-session -t harness-<name>  (at any time)
```

**Typical orchestrator flow:**

1. Call `workspace_create {name: "feat-foo"}` → workspace created, Claude Code launches.
2. Call `workspace_send {id: ..., text: "Implement feature X"}` → prompt sent.
3. Call `workspace_wait_idle {id: ..., timeout_ms: 600000}` → blocks until Claude Code finishes (or times out).
4. Call `workspace_read {id: ..., lines: 500}` to retrieve output.
5. Optionally attach (`tmux attach-session -t harness-feat-foo`) to verify.
6. Call `workspace_archive {id: ...}` when done.

---

## Startup Reconciliation

On startup, `tmux-harness` reconciles the workspace registry against live tmux sessions:

- **Active workspace, session missing** → status set to `orphaned`, logged to stderr. The branch and registry entry are preserved.
- **tmux session with harness prefix, not in registry** → warning logged (not auto-deleted; may be manually created).

---

## Known Limitations

- No authentication on the stdio transport. Secure your process environment.
- Idle detection is heuristic (hash-based); a session that produces the same output repeatedly may appear idle prematurely.
- `maxWorkspaces` is a global cap across all repos, not a per-repo limit.
- tmux session names are globally prefixed (`harness-<name>`); if two repos have a workspace with the same name, they would share a session name. Plan accordingly or use distinct names per repo.

---

## Troubleshooting

**"Claude Code didn't launch in tmux"**
Check `HARNESS_CLAUDE_CMD` points to a valid binary. The session is still created — attach to it manually and launch `claude` to inspect.

**"worktree already exists"**
A previous run left a stale worktree. Run `git worktree prune` in the repo, or use `workspace_delete` to clean up via the MCP interface.

**"store is out of sync"**
Delete `~/.config/tmux-harness/workspaces.json` and restart. Existing tmux sessions will show as untracked warnings at next startup.

**"session shows busy indefinitely"**
Increase `idleThresholdMs`. Or attach to the session manually to check whether Claude Code is actually stuck.
