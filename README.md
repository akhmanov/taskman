# taskman

`taskman` is a Go CLI engine for task/project lifecycle management.

It owns:
- runtime structure validation;
- lifecycle transitions;
- YAML state reads and writes;
- phase execution for `task_start`, `task_done`, `task_cleanup`, and `project_archive`;
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
taskman projects describe <project>
taskman projects create <project>
taskman projects archive <project>

taskman tasks get --project <project> --status <status>
taskman tasks describe <project/task>
taskman tasks create --project <project> --repo <repo> --name <name>
taskman tasks block <project/task>
taskman tasks unblock <project/task>
taskman tasks done <project/task>
taskman tasks cancel <project/task>
taskman tasks cleanup <project/task>

taskman doctor
```

## Metadata flags

Creation commands accept repeatable metadata flags:

```bash
taskman projects create user-permissions --label auth --trait preview=app-api
taskman tasks create --project user-permissions --repo cloud --name api-auth --label backend --trait mr=required
```

- `labels` are for filtering and human grouping.
- `traits` are typed workflow inputs used by `when` selectors in `tasks/config.yaml`.

## Help contract

The CLI is expected to explain itself clearly through `-h` output.

Use:

```bash
taskman --help
taskman projects --help
taskman tasks create --help
```

## Development

```bash
go test ./...
go build ./...
```
