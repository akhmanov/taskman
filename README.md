# taskman

`taskman` is a Go CLI engine for task/project lifecycle management.

It owns:
- runtime structure validation;
- config-driven task transitions;
- YAML state reads and writes;
- explicit transition execution for tasks and archive execution for projects;
- resource-first CLI commands built on `urfave/cli`.

It does not treat `tasks/` as code. The sibling `tasks/` repository is the runtime instance root.

## Runtime root

Runtime root precedence is:

1. `--root`
2. `TASKMAN_ROOT`
3. fallback `../tasks`

Override it with:

```bash
taskman --root /path/to/tasks project list
TASKMAN_ROOT=/path/to/tasks taskman doctor
```

## Install

```bash
go install github.com/akhmanov/taskman@latest
```

## Command shape

```bash
taskman project list
taskman project show <project> --view raw|agent
taskman project add <project>
taskman project update <project> --var k=v --unset-var k
taskman project brief show <project>
taskman project brief set <project> --content "..."
taskman project brief set <project> --file ./project-brief.md
taskman project brief set <project> --file -
taskman project brief edit <project>
taskman project brief init <project> --force
taskman project event add <project> --id EVT-001 --at 2026-03-14T10:00:00Z --type decision --summary "..." --actor taskman
taskman project event list <project> --type decision --active-only
taskman project archive <project>

taskman task list -p <project> --status <status>
taskman task show <task> -p <project> --view raw|agent
taskman task add <task> -p <project>
taskman task update <task> -p <project> --var k=v --unset-var k
taskman task brief show <task> -p <project>
taskman task brief set <task> -p <project> --content "..."
taskman task brief set <task> -p <project> --file ./task-brief.md
taskman task brief set <task> -p <project> --file -
taskman task brief edit <task> -p <project>
taskman task brief init <task> -p <project> --force
taskman task event add <task> -p <project> --id EVT-001 --at 2026-03-14T10:00:00Z --type note --summary "..." --actor taskman
taskman task event list <task> -p <project> --type blocker --active-only
taskman task start <task> -p <project>
taskman task complete <task> -p <project>
taskman task close <task> -p <project>

taskman doctor
```

The resource-first singular form above is the only supported public CLI.

## Metadata flags

Creation commands accept repeatable metadata flags:

```bash
taskman project add user-permissions --label auth --var area=product
taskman task add api-auth -p user-permissions --label backend --var repo=cloud --var kind=feature
```

- `labels` are for filtering and human grouping.
- `vars` are workflow inputs interpreted by the runtime config and external helpers.
- `task add` is side-effect free; automation runs only on explicit transitions.

## Memory Layer

`taskman` separates machine state from durable agent-facing context:

- `state.yaml` keeps canonical machine-oriented state.
- `brief.md` stores current truth for a project or task.
- `events.yaml` stores typed append-only history.
- `show --view agent` renders a bounded short-memory view instead of raw persistence.

For human editing, prefer `brief edit`.
For automation and agents, prefer `brief set --file` or `brief set --file -`.
Use `brief set --content` only for very small updates.

## Help contract

The CLI is expected to explain itself clearly through `-h` output.

Use:

```bash
taskman --help
taskman project --help
taskman task add --help
taskman task start --help
```

## Development

```bash
go test ./...
go build ./...
```
