# Task List: `workspace_wait_idle` MCP Tool

Adds a blocking `workspace_wait_idle` tool that polls internally until the
workspace becomes idle (or a timeout elapses), so callers make a single tool
call instead of repeatedly invoking `workspace_idle`.

Default timeout: **10 minutes** (600 000 ms). Client may override via
`timeout_ms`.

---

## 1. `idle` package — add `WaitUntilIdle`

- [x] Add `WaitUntilIdle(ctx context.Context, ws store.Workspace, capture PaneCapture, updater WorkspaceUpdater, thresholdMs, timeoutMs, pollIntervalMs int64) (IdleStatus, error)` to `internal/idle/idle.go`.
  - Use `time.NewTicker` with `pollIntervalMs` (default 500 ms when ≤ 0).
  - On each tick call `Check(...)` exactly as `workspace_idle` does.
  - Return as soon as `IdleStatus.Idle == true`.
  - Return `context.DeadlineExceeded`-wrapped error when the internal deadline (derived from `timeoutMs`) elapses; include elapsed ms in the error message.
  - Respect `ctx` cancellation in addition to the internal deadline — whichever fires first.
  - Do **not** inline the poll loop into the tool handler; keep it in the `idle` package so it is testable in isolation.

- [x] Add unit tests for `WaitUntilIdle` in `internal/idle/idle_test.go`:
  - Already-idle workspace returns immediately with `Idle: true`.
  - Workspace becomes idle after N ticks; verify it takes ≥ N×pollInterval.
  - Timeout fires before idle; verify error wraps `context.DeadlineExceeded` and `Idle` is false.
  - Parent `ctx` cancellation fires before timeout; verify same error shape.
  - Use a fake `PaneCapture` that toggles hash on demand (same pattern as existing tests).

---

## 2. `tools` package — register `workspace_wait_idle`

- [x] In `internal/tools/tools.go`, inside `Register(...)`, add a new `workspace_wait_idle` tool after `workspace_idle`:

  ```go
  s.AddTool(mcp.NewTool("workspace_wait_idle",
      mcp.WithDescription("Block until the workspace is idle or the timeout elapses. "+
          "Polls pane output internally; returns the same shape as workspace_idle plus a timed_out flag."),
      mcp.WithString("id",
          mcp.Required(),
          mcp.Description("Workspace ID"),
      ),
      mcp.WithNumber("timeout_ms",
          mcp.Description("Maximum time to wait in milliseconds (default 600000 = 10 min)"),
      ),
      mcp.WithNumber("threshold_ms",
          mcp.Description("Idle-stability threshold override in milliseconds"),
      ),
      mcp.WithNumber("poll_interval_ms",
          mcp.Description("How often to sample the pane (default 500 ms)"),
      ),
  ), ...)
  ```

- [x] Handler logic:
  - Resolve `timeout_ms` → default `600_000`.
  - Resolve `threshold_ms` → default `defaultThresholdMs` (passed into `Register`).
  - Resolve `poll_interval_ms` → default `500`.
  - Look up workspace via `mgr.Get(id)`; return tool error if not found or not active.
  - Call `idle.WaitUntilIdle(ctx, ws, capture, storeUpd, thresholdMs, timeoutMs, pollIntervalMs)`.
  - On success: return `IdleStatus` JSON with an added `"timed_out": false` field.
  - On timeout/cancellation error: return `IdleStatus`-shaped JSON with `"idle": false, "timed_out": true` — **not** a tool error, so the client can distinguish timeout from a hard failure. Hard failures (pane capture error, store error) still return `mcp.NewToolResultError`.

- [x] Define a `waitIdleResult` struct (or embed `idle.IdleStatus` + `TimedOut bool`) to produce consistent JSON — do not inline `map[string]any`.

- [x] Add handler-level unit tests in `internal/tools/tools_test.go`:
  - Happy path: workspace already idle → returns `{idle: true, timed_out: false}`.
  - Timeout path: fake capture never changes hash, short `timeout_ms` → returns `{idle: false, timed_out: true}`.
  - Unknown workspace ID → tool error.
  - Non-active workspace status → tool error.

---

## 3. Smoke test

- [x] In `cmd/smoke/main.go`, add a `workspace_wait_idle` call after `workspace_send` (if the smoke test exercises a send). Pass `timeout_ms: 30000` so the smoke test doesn't hang. Log whether the result was idle or timed out. Do **not** fail the smoke test on timeout — just log it.

---

## 4. Documentation

- [x] Update `README.md` tool table to include `workspace_wait_idle` with its parameters and return shape.
- [x] Add an example JSON response block showing both the success (`timed_out: false`) and timeout (`timed_out: true`) cases.

---

## Return shape reference

**Success (idle):**
```json
{
  "Idle": true,
  "TimedOut": false,
  "LastChangedAt": "2026-06-05T11:30:00Z",
  "ElapsedMs": 5200,
  "ThresholdMs": 5000
}
```

**Timeout:**
```json
{
  "Idle": false,
  "TimedOut": true,
  "LastChangedAt": "2026-06-05T11:29:55Z",
  "ElapsedMs": 600000,
  "ThresholdMs": 5000
}
```

**Hard error** (tool error string, not JSON):
```
capture failed: exit status 1
```

---

## Implementation order

1 → 2 → 3 → 4. Do not register the tool until `WaitUntilIdle` exists and its
tests pass. One commit per numbered section.
