# Task: Make HARNESS_REPO_PATH Optional

**Goal:** When `HARNESS_REPO_PATH` (and the `repoPath` JSON field) are not set, default to the
output of `git rev-parse --show-toplevel` run from the process working directory.
No legacy shims — clean defaulting logic only.

**Files touched:** `internal/config/config.go`, `internal/config/config_test.go`, `cmd/smoke/main.go`

---

## Checklist

### 1. [x] Add `detectRepoPath()` helper — `internal/config/config.go`

Add a private function that calls `exec.Command("git", "rev-parse", "--show-toplevel")` with
binary and args as separate strings (no shell interpolation), trims whitespace from the output,
and returns `("", err)` on failure. Commit: `feat(config): add detectRepoPath helper`.

### 2. [x] Wire auto-detection into `config.Load()` — `internal/config/config.go`

After env-var overrides are applied, check `if cfg.RepoPath == ""`; if so, call
`detectRepoPath()` and assign the result on success (silently ignore the error — an empty
`RepoPath` at this point is valid until `Validate()` runs). Commit: `feat(config): auto-detect repoPath via git rev-parse --show-toplevel`.

### 3. [x] Re-apply derived defaults after auto-detection — `internal/config/config.go`

Move (or add a second pass of) the `WorktreeRoot` and `StorePath` defaults block to run
**after** the auto-detection step so they pick up the auto-detected `RepoPath`. The
`StorePath` default should be `<repoPath>/.hangar/workspaces.json` when `RepoPath` is known,
falling back to `~/.config/hangar/workspaces.json` otherwise. Commit: `feat(config): default storePath to <repoPath>/.hangar/workspaces.json`.

### 4. [x] Update `Validate()` error message — `internal/config/config.go`

Replace the old "repoPath is required" error with:
`"repoPath is not set and could not be auto-detected; set HARNESS_REPO_PATH or run from within a git repository"`.
Keep the subsequent `os.Stat` and `.git` existence checks unchanged. Commit can be folded into step 2.

### 5. [x] Add happy-path auto-detection test — `internal/config/config_test.go`

`TestLoad_AutoDetectRepoPath`: set `HARNESS_REPO_PATH=""`, call `Load("")`, assert
`cfg.RepoPath != ""` (git detects the repo the test binary runs from) and
`cfg.StorePath` contains `.hangar`. Commit can be folded into step 2.

### 6. [x] Add failure-path auto-detection test — `internal/config/config_test.go`

`TestLoad_AutoDetectFails_RepoPathEmpty`: make `detectRepoPath` injectable (export it as a
package-level `var detectRepoPathFn = detectRepoPath` and call through the variable in
`Load()`), then override it in the test to return `("", errors.New("not a repo"))`. Call
`Load("")` with `HARNESS_REPO_PATH` unset; assert `cfg.RepoPath == ""`. Then call
`Validate(cfg)` and assert the error contains `"auto-detected"`. This is the only case not
covered by existing tests. Commit: `test(config): cover auto-detect failure path`.

### 7. [x] Update smoke test to make `--repo` optional — `cmd/smoke/main.go`

The smoke test currently fatalf-exits when `--repo ""`. Since the binary now auto-detects,
remove the hard requirement: if `*repoPath == ""`, do not inject `HARNESS_REPO_PATH` into the
env (let the server auto-detect). Update the usage string to mark `--repo` as optional.
Commit: `test(smoke): make --repo flag optional via auto-detection`.

---

## What "done" means

- `go build ./...` clean
- `go vet ./...` clean  
- `go test -race ./...` passes
- `golangci-lint run` passes
- All checkboxes above marked `[x]`
