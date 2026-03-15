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

## Initialize a runtime

Create a minimal `taskman.yaml`:

```bash
taskman --root /path/to/runtime init
```

## Command shape

```bash
taskman project add docs-refresh --description "Refresh taskman docs"
taskman project show docs-refresh
taskman project message add docs-refresh --body "Capture current scope"
taskman project transition list docs-refresh
taskman project plan docs-refresh

taskman task add api-auth -p docs-refresh --description "Implement API auth"
taskman task show api-auth -p docs-refresh
taskman task message add api-auth -p docs-refresh --kind decision --body "Use token auth"
taskman task transition list api-auth -p docs-refresh
taskman task start api-auth -p docs-refresh
```

## Storage model

`taskman` stores each entity as:

- `manifest.json` - immutable identity and description seed
- `events/*.json` - append-only journal, one event per file
- `artifacts/*.json` - durable machine outputs for task middleware

Current state is computed on read from `manifest.json` plus journal events.

## Event model

Raw events are internal storage. User-facing CLI surfaces derive from them:

- `message` events power message timelines
- `transition` events power transition history and current status
- `metadata_patch` events update labels and vars
- middleware lifecycle events stay internal by default

## Middleware model

`taskman.yaml` attaches middleware to built-in transitions:

- `pre` middleware can block a transition
- `post` middleware can emit warnings, facts, and artifacts
- middleware execution is recorded in the internal event journal

## Development

```bash
go test ./...
go build ./...
```
