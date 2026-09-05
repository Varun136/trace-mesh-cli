# Tracemesh test suite

## Scope

`integration_test.go` exercises the `tm` CLI as a user would run it. Each test creates an isolated temporary Git repository, builds the CLI once, runs commands as subprocesses, and verifies command output plus repository state. The suite covers initialization, agent integration, task creation, active-task pointers, notes, task listing, history, switching, synchronization, archiving, prompts, and invalid-context behavior.

The tests intentionally avoid checks unrelated to Tracemesh functionality.

## Prerequisites

- Go 1.22 or newer
- Git available on `PATH`
- A clean working tree is not required; tests use temporary repositories

## Run the complete suite

From the repository root:

```bash
go test ./test -v
```

Expected result: every test passes and Go reports `PASS`.

## Run with the race detector

```bash
go test -race ./test -v
```

Expected result: the same tests pass with no race reports.

## Run a specific suite

```bash
go test ./test -run TestInit -v
go test ./test -run TestStart -v
go test ./test -run TestStatusShowAndNote -v
go test ./test -run TestListAndHistory -v
go test ./test -run TestSwitch -v
go test ./test -run TestSync -v
go test ./test -run TestFinish -v
go test ./test -run TestPrompt -v
go test ./test -run TestCommandsRequireValidContext -v
```

## Test detail

### `TestInitCreatesStateAndCheckoutHook`

**Run:** `go test ./test -run TestInitCreatesStateAndCheckoutHook -v`

**Expected:** `tm init` succeeds in a Git repository, creates `.tracemesh/config.json`, task and archive directories, installs `.git/hooks/post-checkout`, includes `tm sync` in the hook, and makes the hook executable.

### `TestInitRequiresGitRepository`

**Run:** `go test ./test -run TestInitRequiresGitRepository -v`

**Expected:** `tm init` fails outside Git and reports that Tracemesh requires a Git repository.

### `TestAddDetectsAndAddsAgentRules`

**Run:** `go test ./test -run TestAddDetectsAndAddsAgentRules -v`

**Expected:** an existing `CLAUDE.md` receives the context protocol during initialization; `tm add cursor` creates `.cursorrules`; repeated additions do not duplicate the protocol.

### `TestAddFallbackAndRequiresInit`

**Run:** `go test ./test -run TestAddFallbackAndRequiresInit -v`

**Expected:** adding an agent before initialization fails with the initialization guidance. After initialization, an unknown agent uses `AGENTS.md` as the fallback instruction file.

### `TestStartCreatesDescriptionAndActiveTask`

**Run:** `go test ./test -run TestStartCreatesDescriptionAndActiveTask -v`

**Expected:** `tm start` creates `TM-001`, preserves the entered description, creates the required Markdown sections, and points `.tracemesh/active.md` to the task.

### `TestStartUsesTitleWhenDescriptionIsBlankAndPreventsSecondActiveTask`

**Run:** `go test ./test -run TestStartUsesTitleWhenDescriptionIsBlankAndPreventsSecondActiveTask -v`

**Expected:** a blank description falls back to the title, and starting another task while one is active fails without replacing the active task.

### `TestStatusShowAndNote`

**Run:** `go test ./test -run TestStatusShowAndNote -v`

**Expected:** `status` reports the branch and active task, `show` prints the complete active Markdown document, and `note` appends the supplied text to the decision log.

### `TestListAndHistoryShowActiveAndArchivedTasks`

**Run:** `go test ./test -run TestListAndHistoryShowActiveAndArchivedTasks -v`

**Expected:** `list` separates active and archived tasks, while `history` displays the current branch's task associations and titles.

### `TestSwitchChangesActiveTaskAndWarnsOnOtherBranch`

**Run:** `go test ./test -run TestSwitchChangesActiveTaskAndWarnsOnOtherBranch -v`

**Expected:** `switch` activates an existing task and updates the active symlink to that task. The test also uses a second branch to ensure task context is branch-aware.

### `TestSyncSelectsLatestBranchTaskAndClearsWhenNone`

**Run:** `go test ./test -run TestSyncSelectsLatestBranchTaskAndClearsWhenNone -v`

**Expected:** `sync` clears active context on a branch with no configured tasks and restores the latest valid task when returning to the configured branch.

### `TestFinishArchivesAndClearsActiveTask`

**Run:** `go test ./test -run TestFinishArchivesAndClearsActiveTask -v`

**Expected:** `finish` moves the task from `tasks` to `archive`, removes the active pointer, and reports the completed task and branch.

### `TestPromptPrintsAgentProtocol`

**Run:** `go test ./test -run TestPromptPrintsAgentProtocol -v`

**Expected:** `tm-prompt` prints the protocol, including the requirement to read `.tracemesh/active.md` and maintain the decision log.

### `TestCommandsRequireValidContext`

**Run:** `go test ./test -run TestCommandsRequireValidContext -v`

**Expected:** commands requiring initialized Tracemesh state fail before initialization, and commands requiring an active task fail when no active task exists.

## Troubleshooting

- If the build fails, verify the Go version with `go version` and run `go mod download`.
- If Git setup fails, verify `git --version` and that Git can initialize a repository.
- If a test fails, rerun only that test with `-v`; the failure output includes the temporary command output and expected repository state.
- The tests build `./cmd/tm` automatically and do not use the checked-in `tm` binary.
