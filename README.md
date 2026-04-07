# taskman

`taskman` is an append-only Go CLI for project and task workflow management.

## Runtime root

Runtime root precedence is:

1. `--root`
2. `TASKMAN_ROOT`
3. current directory

Examples:

```bash
taskman --root /path/to/runtime init
TASKMAN_ROOT=/path/to/runtime taskman project list
```

## Optional config overlay

`taskman` works without `taskman.yaml`.

Create a minimal overlay config when you want defaults or transition middleware:

```bash
taskman --root /path/to/runtime init
```

## Refs and numbering

Projects and tasks get stable numbers when they are created.

- project ref: `<number>_<slug>` like `1_docs-refresh`
- task ref: `<number>_<slug>` like `1_api-auth`

Projects can be addressed by:

- canonical ref: `1_docs-refresh`
- short number: `1` or `#1`
- bare slug: `docs-refresh`

Tasks use the same ref forms inside a project selected by `-p/--project`.

## Common flow

```bash
taskman project add docs-refresh --description "Refresh taskman docs"
taskman project show docs-refresh
taskman project plan docs-refresh

taskman task add api-auth -p docs-refresh --description "Implement API auth"
taskman task show api-auth -p docs-refresh
taskman task message add api-auth -p docs-refresh --kind decision --body "Use token auth"
taskman task transition list api-auth -p docs-refresh
taskman task start api-auth -p docs-refresh
```

After creation, the canonical refs will look like `1_docs-refresh` and `1_docs-refresh/1_api-auth`.

## Labels

Use labels to classify and filter projects and tasks.

```bash
taskman project label add docs-refresh feature cleanup
taskman project label remove docs-refresh cleanup

taskman task label add api-auth -p docs-refresh backend feature
taskman task label remove api-auth -p docs-refresh backend

taskman project list --label feature
taskman task list -p docs-refresh --label feature --label cleanup
```

Notes:

- labels are normalized to lowercase
- duplicate labels are removed
- repeated `--label` in `list` uses any-of matching
- `update --label` still replaces the full label set

## Rename

Rename keeps the number stable and changes only the slug part of the ref.

```bash
taskman project rename docs-refresh docs-v2
taskman task rename api-auth auth-backend -p docs-v2
```

Examples above become canonical refs like `1_docs-v2` and `1_docs-v2/1_auth-backend`.

## JSON output

Read commands support `--output json`.

```bash
taskman project list --output json
taskman project show docs-refresh --output json

taskman task list -p docs-refresh --output json
taskman task show api-auth -p docs-refresh --output json

taskman task message list api-auth -p docs-refresh --output json
taskman task transition list api-auth -p docs-refresh --output json
```

## Storage model

`taskman` stores each entity as:

- `manifest.json` - immutable identity and description seed
- `events/*.json` - append-only journal, one event per file
- `artifacts/*.json` - durable machine outputs for task middleware

Current state is computed on read from `manifest.json` plus journal events.

Canonical filesystem layout uses numbered refs:

```text
projects/1_docs-refresh/manifest.json
projects/1_docs-refresh/events/*.json
projects/1_docs-refresh/tasks/1_api-auth/manifest.json
projects/1_docs-refresh/tasks/1_api-auth/events/*.json
projects/1_docs-refresh/tasks/1_api-auth/artifacts/*.json
```

## Event model

Raw events are internal storage. User-facing CLI surfaces derive from them:

- `message` events power message timelines
- `transition` events power transition history and current status
- `metadata_patch` events update labels and vars
- middleware lifecycle events stay internal by default

## Middleware model

If present, `taskman.yaml` attaches middleware to built-in transitions:

- `pre` middleware can block a transition
- `post` middleware can emit warnings, facts, and artifacts
- middleware execution is recorded in the internal event journal

If the file is absent, `taskman` falls back to built-in runtime behavior with empty defaults and no middleware.

## Development

```bash
go test ./...
go build ./...
```
