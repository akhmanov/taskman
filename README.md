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

By default the CLI looks for `../tasks` relative to the `taskman/` repo.

Override it with:

```bash
taskman --root /path/to/tasks projects get
```

## Command shape

```bash
taskman projects get
taskman projects describe <project> --view raw|agent
taskman projects create <project>
taskman projects update <project> --var k=v --unset-var k
taskman projects brief get <project>
taskman projects brief set <project> --content "..."
taskman projects brief set <project> --file ./project-brief.md
taskman projects brief edit <project>
taskman projects brief init <project> --force
taskman projects events add <project> --id EVT-001 --at 2026-03-14T10:00:00Z --type decision --summary "..." --actor taskman
taskman projects events get <project> --type decision --active-only
taskman projects archive <project>

taskman tasks get --project <project> --status <status>
taskman tasks describe <project/task> --view raw|agent
taskman tasks create --project <project> --name <name>
taskman tasks update <project/task> --var k=v --unset-var k
taskman tasks brief get <project/task>
taskman tasks brief set <project/task> --content "..."
taskman tasks brief set <project/task> --file ./task-brief.md
taskman tasks brief edit <project/task>
taskman tasks brief init <project/task> --force
taskman tasks events add <project/task> --id EVT-001 --at 2026-03-14T10:00:00Z --type note --summary "..." --actor taskman
taskman tasks events get <project/task> --type blocker --active-only
taskman tasks transition <project/task> <transition>

taskman doctor
```

## Metadata flags

Creation commands accept repeatable metadata flags:

```bash
taskman projects create user-permissions --label auth --var area=product
taskman tasks create --project user-permissions --name api-auth --label backend --var repo=cloud --var kind=feature
```

- `labels` are for filtering and human grouping.
- `vars` are workflow inputs interpreted by the runtime config and external helpers.
- `tasks create` is side-effect free; automation runs only on explicit transitions.

## Memory Layer

`taskman` separates machine state from durable agent-facing context:

- `state.yaml` keeps canonical machine-oriented state.
- `brief.md` stores current truth for a project or task.
- `events.yaml` stores typed append-only history.
- `describe --view agent` renders a bounded short-memory view instead of raw persistence.

For human editing, prefer `brief edit`.
For automation and agents, prefer `brief set --file`.
Use `brief set --content` only for very small updates.

## Help contract

The CLI is expected to explain itself clearly through `-h` output.

Use:

```bash
taskman --help
taskman projects --help
taskman tasks create --help
taskman tasks transition --help
```

## Development

```bash
go test ./...
go build ./...
```
