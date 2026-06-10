# Agent Ergonomics Improvements

Review of hangar from the perspective of an orchestrating agent (Claude Code, cabinet, openclaw,
Hermes) managing multiple Claude Code feature branches. Tasks ordered roughly by impact.

---

## Task 1 — Rename HARNESS → HANGAR throughout

All env vars, defaults, and references use the old `HARNESS` prefix. This is a breaking change
but should happen before any further work so subsequent tasks start clean.

**Scope:**
- `internal/config/config.go`: env var keys `HARNESS_*` → `HANGAR_*`; `defaultSessionPrefix`
  `"harness-"` → `"hangar-"`; validation error messages; `PrintSummary` output
- `internal/config/config_test.go`: any env var names in tests
- `cmd/root.go`: flag descriptions and any HARNESS references
- `README.md`: env var table, example configs, troubleshooting section
- `CLAUDE.md`: env var table

**Note:** The `sessionPrefix` default changing from `harness-` to `hangar-` means existing
running sessions with the old prefix will appear untracked on the next startup. Document this in
the commit message.

---

## Task 2 — Raise `maxWorkspaces` default and cap to 100

Agents doing aggressive fan-out need more than 10 concurrent workspaces.

**Changes:**
- `internal/config/config.go`: `defaultMaxWorkspaces = 10` → `100`; validation range
  `1–50` → `1–100`
- `README.md`: update the default and range in the config table

---

## Task 3 — `workspace_send`: sleep through rate limit instead of returning an error

Rate limit errors surface to the agent as tool failures, causing unnecessary retry logic with
backoff. Since the cooldown is at most 200 ms, the right behavior is to block briefly and then
send — the caller never needs to know the rate limit existed.

**Changes in `internal/tools/tools.go`:**
- In the `workspace_send` handler, when `rl.check` returns `ok=false`, sleep for
  `retryAfterMs` milliseconds then call `rl.check` again (or just call `mgr.SendKeys` directly
  after sleeping, updating the rate limiter's timestamp).
- Remove the `{"error":"rate limited","retry_after_ms":N}` error path entirely.
- The rate limiter's `check` method can be restructured to a `wait` method that blocks until
  the cooldown has elapsed and then records the new send time, keeping the mutex logic clean.

---

## Task 4 — `workspace_list`: replace boolean wait flags with a single `wait_for_idle` enum

Two mutually exclusive booleans (`wait_any_idle`, `wait_all_idle`) are an awkward API and
generate confusing tool descriptions for LLMs. A single string parameter is clearer.

**New parameter:** `wait_for_idle` (string, optional, default `"none"`)
- `"none"` — return immediately (current default behavior, no wait)
- `"any"` — block until at least one workspace is idle
- `"all"` — block until all workspaces are idle

**Changes:**
- `internal/tools/tools.go`: replace `mcp.WithBoolean("wait_any_idle", ...)` and
  `mcp.WithBoolean("wait_all_idle", ...)` with `mcp.WithString("wait_for_idle", ...)`;
  update handler logic to read the string and derive `waitAny`/`waitAll` booleans from it;
  return a validation error if the value is not `"none"`, `"any"`, or `"all"`
- `README.md`: update `workspace_list` parameter table
- Update the CLI client (`client/cmd_list.go`) if it exposes these flags

---

## Task 5 — `workspace_create`: add optional `prompt` parameter

An orchestrator's most common action is: create workspace → send first prompt. Requiring two
separate tool calls is extra latency and extra round trips. Adding a `prompt` string to
`workspace_create` lets an agent start a Claude session with a task in a single call.

**Behavior when `prompt` is set:**
1. Complete the normal create sequence (worktree + tmux + `claude` launch).
2. After launching `claude`, poll the pane until Claude's input prompt is visible (idle for
   at least `idleThresholdMs`), up to a 30-second startup timeout.
3. Send the prompt text (as if `workspace_send` were called), then return the workspace object.

**Changes:**
- `internal/workspace/workspace.go`: add `Prompt string` to `CreateOptions`; after the
  `SendKeys(claude)` step, if `Prompt != ""`, call `idle.WaitUntilIdle` with a 30 s timeout,
  then call `SendKeys(prompt)` on the session
- `internal/tools/tools.go`: add `mcp.WithString("prompt", ...)` to `workspace_create`;
  pass through to `CreateOptions`
- `README.md`: document the new parameter

**Side effect:** This also addresses the "no signal that Claude started" problem — the startup
wait in step 2 will surface a timeout error if `claude` fails to reach its input prompt within
30 seconds, which is better than the current silent 300 ms sleep.

---

## Task 6 — `workspace_delete`: add `delete_branch` parameter

Today `Delete` always removes the git branch. An agent cleaning up a session after work is done
usually wants the branch to survive (for a PR, for review). Deleting the branch should be
opt-in.

**New parameter:** `delete_branch` (bool, optional, default `false`)

**Changes:**
- `internal/workspace/workspace.go`: wrap the `git branch -d/-D` block in
  `if opts.DeleteBranch { ... }`; thread `DeleteBranch bool` through a new struct or an
  additional parameter on `Delete`
- `internal/tools/tools.go`: add `mcp.WithBoolean("delete_branch", ...)` to
  `workspace_delete`; pass through to `Manager.Delete`
- `README.md`: update `workspace_delete` parameter table; note that the default changed
  (branch is now preserved by default)

---

## Task 7 — `workspace_create`: support existing branches and a `base_branch` option

Agents managing real feature workflows often need to:
- Create a workspace on a branch that already exists (e.g. pick up stalled work)
- Branch from a specific base (not HEAD)

Currently `worktree.Add` is always called with `createBranch: true`, so neither is possible.

**New parameters on `workspace_create`:**
- `base_branch` (string, optional) — the branch or commit to branch from; defaults to HEAD
- `create_branch` (bool, optional, default `true`) — if `false`, check out `branch` as an
  existing branch rather than creating a new one

**Changes:**
- `internal/worktree/worktree.go`: `Add` already accepts `createBranch bool`; when
  `createBranch=true` and `baseBranch != ""` pass it as the third positional arg to
  `git worktree add <path> -b <branch> <base>`
- `internal/workspace/workspace.go`: add `BaseBranch string` and `CreateBranch bool` (default
  `true`) to `CreateOptions`; pass through to `worktree.Add`
- `internal/tools/tools.go`: add `mcp.WithString("base_branch", ...)` and
  `mcp.WithBoolean("create_branch", ...)` to `workspace_create`
- `README.md`: update `workspace_create` parameter table

---

## Task 8 — Expose capacity in `workspace_list` response

Agents discover the workspace cap only by hitting it. The list response should include
headroom so agents can plan fan-out without trial and error.

**Change the plain-array response to an envelope:**
```json
{
  "max_workspaces": 100,
  "active_count": 3,
  "workspaces": [ ... ]
}
```

The wait-flag path already returns a `listWaitResult` struct; unify both paths to always
return the envelope (add `MaxWorkspaces` and `ActiveCount` fields to `listWaitResult`, or
rename it to `listResult` and use it for both paths).

**Changes:**
- `internal/tools/tools.go`: update `listWaitResult` (or introduce `listResult`) with
  `MaxWorkspaces int` and `ActiveCount int`; populate from `cfg.MaxWorkspaces` and
  `len(workspaces)`; thread `cfg` into `Register` or close over it
- `README.md`: update `workspace_list` output description

---

## Task 9 — Structured error responses

Agents that want to handle specific failures programmatically (workspace not found vs at
capacity vs validation error) must today string-match error messages. Structured errors let
agents branch on a `code` field.

**Proposed error shape:**
```json
{"error": true, "code": "not_found", "message": "workspace not found: foo"}
```

**Error codes** mapping to existing sentinel errors:
- `not_found` — `ErrNotFound`
- `invalid_name` — `ErrInvalidName`
- `capacity_reached` — `ErrCapacityReached`
- `delete_not_confirmed` — `ErrDeleteNotConfirmed`
- `ambiguous` — `ErrAmbiguous`
- `rate_limited` — (if rate limit is ever surfaced as data rather than a block; see Task 3)
- `internal` — everything else

**Changes:**
- `internal/tools/tools.go`: add a `toolError(code, message string)` helper that returns a
  `mcp.NewToolResultText` with the JSON above (not `mcp.NewToolResultError`, so agents see it
  as data); replace all `mcp.NewToolResultError(err.Error())` call sites, using
  `errors.Is` to map sentinel errors to codes

---

## Task 10 — `workspace_read`: add a line-offset cursor

`workspace_read` always returns the last N lines. For long-running Claude tasks, agents doing
a second or third read can't efficiently retrieve only what changed.

**New parameter:** `since_line` (int, optional) — if set, return only lines after this offset
from the start of the captured buffer.

**Response addition:** include `total_lines int` in the output so the agent can pass
`since_line=<previous total_lines>` on the next call.

**Note:** tmux's `capture-pane` buffer is bounded and scrolls, so `since_line` is a
best-effort cursor, not a guaranteed log offset. Document this limitation.

**Changes:**
- `internal/tools/tools.go`: add `since_line` parameter; after capturing, split on `\n`,
  slice from `since_line`, rejoin; include `total_lines` and `returned_lines` in
  `readResult`
- `README.md`: update `workspace_read` parameter and output tables
