# taskman

`taskman` is an opinionated Go CLI for agent-first project and task workflow management.

The product contract is built in:
- fixed statuses for both `project` and `task`: `backlog`, `planned`, `in_progress`, `paused`, `done`, `canceled`;
- fixed transition verbs;
- flat `list` views for scanning;
- grouped `show` views for operational work;
- append-only transition history kept separate from narrative events.

`taskman` does not use a workflow DSL. `taskman.yaml` is only for runtime defaults and pre/post middleware.

## Runtime root

Runtime root precedence is:

1. `--root`
2. `TASKMAN_ROOT`
3. fallback `../tasks`

Examples:

```bash
taskman --root /path/to/runtime init
TASKMAN_ROOT=/path/to/runtime taskman project list
```

## Install

```bash
go install github.com/akhmanov/taskman@latest
```

## Initialize a runtime

Create a minimal `taskman.yaml`:

```bash
taskman --root /path/to/runtime init
```

This writes only `taskman.yaml`. Projects and tasks create their own directories lazily.

## Command shape

```bash
taskman project list
taskman project show <project> --all
taskman project add <project>
taskman project update <project> --var k=v --unset-var k
taskman project plan <project>
taskman project start <project>
taskman project pause <project> --reason-type external_blocker --reason "..." --resume-when "..."
taskman project resume <project>
taskman project complete <project> --summary "..."
taskman project cancel <project> --reason-type deprioritized --reason "..."
taskman project reopen <project>

taskman task list -p <project> --active
taskman task list -p <project> --status paused,done
taskman task list -p <project> --exclude-status done,canceled
taskman task show <task> -p <project>
taskman task add <task> -p <project>
taskman task update <task> -p <project> --var k=v --unset-var k
taskman task plan <task> -p <project>
taskman task start <task> -p <project>
taskman task pause <task> -p <project> --reason-type waiting_feedback --reason "..." --resume-when "..."
taskman task resume <task> -p <project>
taskman task complete <task> -p <project> --summary "..."
taskman task cancel <task> -p <project> --reason-type deprioritized --reason "..."
taskman task reopen <task> -p <project>
```

The singular resource-first form above is the public CLI.

## Metadata

Creation and update commands support repeatable metadata flags:

```bash
taskman project add user-permissions --label auth --var repo=cloud
taskman task add api-auth -p user-permissions --label backend --var branch=feature/api-auth
```

- `labels` are for filtering and grouping.
- `vars` are runtime metadata that middleware can inspect.

## Storage model

`taskman` keeps current state and history separate:

- `state.yaml` — current snapshot
- `brief.md` — current human/agent context
- `transitions.yaml` — append-only transition audit log
- `events.yaml` — append-only narrative events (notes, decisions, blockers, handoffs)

There is no archive area. Terminal entities stay queryable through filters and views.

## Middleware model

`taskman.yaml` can attach middleware to built-in transitions:

- `pre` middleware can block a transition
- `post` middleware can emit warnings, facts, and artifacts
- `post` middleware never rolls back an already-applied transition

## Views

- `list` is flat and sorted by canonical status order, then `updated_at desc`
- `project show` is the main grouped workboard
- terminal task groups are collapsed by default in `project show`; use `--all` to expand them

## Development

```bash
go test ./...
go build ./...
```
