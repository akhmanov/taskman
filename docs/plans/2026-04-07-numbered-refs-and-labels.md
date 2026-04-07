# Numbered Refs And Labels Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add numbered project/task refs, explicit rename, first-class label operations, and machine-readable read output while keeping taskman append-only and filesystem-backed.

**Architecture:** Preserve append-only manifests and event journals, but separate internal identity from human-facing refs. The store becomes responsible for canonical ref resolution, numbering allocation, and physical directory moves; lifecycle owns rename/label use cases; CLI only parses refs, renders output, and exposes the new commands/flags.

**Tech Stack:** Go, urfave/cli v3, filesystem-backed store, JSON/YAML manifests, Go test.

---

### Task 1: Save ref model and numbering tests

**Files:**
- Modify: `internal/store/v3_store_test.go`
- Modify: `internal/cli/v3_cli_test.go`

**Step 1: Write the failing tests**

- Add store tests that expect project manifests and task manifests to carry `number` and canonical directory names.
- Add CLI tests that expect list/show output to use canonical refs and number-first ordering.
- Add CLI tests that resolve refs by composite ref, short number, and bare slug.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store ./internal/cli`

**Step 3: Write the minimal implementation**

- Extend manifest data model with `number` and canonical ref helpers.
- Update store path helpers and list/load logic to support canonical directory names.
- Update CLI rendering to print canonical refs.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/store ./internal/cli`

### Task 2: Add resolver and sequence allocation

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/layout.go`
- Modify: `internal/model/entity_v3.go`
- Modify: `internal/model/status.go`
- Create: `internal/store/refs.go`
- Create: `internal/store/refs_test.go`

**Step 1: Write the failing tests**

- Add resolver tests for:
  - composite project/task refs
  - numeric project/task refs
  - bare slug refs
  - ambiguous / not found errors
- Add sequence tests for global project numbering and per-project task numbering.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store`

**Step 3: Write the minimal implementation**

- Add explicit ref parsing and resolution helpers.
- Add a simple persisted sequence state under the runtime root / project root.
- Make store create/load/list operations use resolved locators instead of raw slug-only paths.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/store`

### Task 3: Add rename use cases

**Files:**
- Modify: `internal/lifecycle/project.go`
- Modify: `internal/lifecycle/task.go`
- Modify: `internal/store/store.go`
- Modify: `internal/cli/app.go`
- Modify: `internal/cli/v3_cli_test.go`

**Step 1: Write the failing tests**

- Add CLI tests for `project rename` and `task rename`.
- Add store / lifecycle tests proving project rename moves the whole subtree and task rename only moves the task directory.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/lifecycle ./internal/cli`

**Step 3: Write the minimal implementation**

- Add explicit rename methods in lifecycle.
- Add store move helpers that rewrite manifests after directory rename.
- Wire new CLI commands.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/lifecycle ./internal/cli`

### Task 4: Promote labels to first-class CLI behavior

**Files:**
- Modify: `internal/cli/app.go`
- Modify: `internal/lifecycle/project.go`
- Modify: `internal/lifecycle/task.go`
- Modify: `internal/model/projection_v3.go`
- Modify: `internal/cli/v3_cli_test.go`

**Step 1: Write the failing tests**

- Add tests for label normalization and dedupe.
- Add CLI tests for `label add`, `label remove`, project/task show label rendering, and `list --label` filtering.
- Keep an explicit test that `update --label` still replaces the full label set.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli ./internal/lifecycle`

**Step 3: Write the minimal implementation**

- Normalize labels before storage.
- Add lifecycle helpers for incremental add/remove.
- Add list-time filtering by any matching label.
- Render project labels in `project show`.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli ./internal/lifecycle`

### Task 5: Add JSON output for read commands

**Files:**
- Modify: `internal/cli/app.go`
- Modify: `internal/cli/v3_cli_test.go`
- Modify: `README.md`

**Step 1: Write the failing tests**

- Add CLI tests for `--output json` on read commands: list/show/message list/transition list.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli`

**Step 3: Write the minimal implementation**

- Add a shared `--output` flag for read commands.
- Keep text output as default and return stable JSON structures when requested.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli`

### Task 6: Final verification and docs

**Files:**
- Modify: `README.md`
- Modify: `internal/cli/app.go` help text as needed

**Step 1: Run focused tests**

Run: `go test ./internal/store ./internal/lifecycle ./internal/cli`

**Step 2: Run full test suite**

Run: `go test ./...`

**Step 3: Build the binary**

Run: `go build ./...`

**Step 4: Update docs**

- Document numbered refs, rename, label commands, and JSON output in `README.md`.

**Step 5: Re-run verification**

Run: `go test ./... && go build ./...`
