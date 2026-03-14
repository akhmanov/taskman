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
taskman projects describe <project>
taskman projects create <project>
taskman projects archive <project>

taskman tasks get --project <project> --status <status>
taskman tasks describe <project/task>
taskman tasks create --project <project> --name <name>
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
