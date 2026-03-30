# Machina User Guide

This guide is for using `machina` from the command line.

For architecture and implementation details, see [README.md](README.md).

## Build

Build the CLI binary from the project root:

```bash
go build -o Machina ./cmd
```

Examples in this guide use the command name `machina`. If your binary is named `Machina`, either run `./Machina ...` or rename it.

## Core Concepts

Machina uses two related terms:

- `profile`: a scaffolding template used when generating a new job source file
- `type`: a registered runtime job type that the server can execute

Examples:

- profiles: `batch`, `singleRun`
- types: `csv_transform`, `file_encrypt`

## Configuration

Machina reads defaults from `config.json`.

For commands that accept `--port`, precedence is:

```text
flag override -> config.json -> 8080 fallback
```

You can inspect the current config with:

```bash
machina config
```

You can update config values with:

```bash
machina config --port 9090
machina config --workers 6 --queue-size 50
```

## Start the Server

Start Machina with config defaults:

```bash
machina start
```

Override runtime settings:

```bash
machina start --port 9090 --workers 8 --queue-size 200
```

Check health:

```bash
machina health
machina health --port 9090
```

Stop the server:

```bash
machina shutdown
machina shutdown --port 9090
```

## Submit and Track Jobs

Machina currently includes two built-in submit aliases:

- `csv-transform`
- `file-encrypt`

Submit a CSV transform job:

```bash
machina submit csv-transform ./input.csv ./output.csv
```

Submit a file encryption job:

```bash
machina submit file-encrypt ./input ./encrypted
```

Submit against a specific port:

```bash
machina submit csv-transform ./input.csv ./output.csv --port 9090
```

The submit response includes a job id:

```json
{
  "id": "1234567890",
  "status": "pending"
}
```

Check the current status:

```bash
machina status 1234567890
machina status 1234567890 --port 9090
```

Watch until the job reaches a terminal state:

```bash
machina status 1234567890 --watch
machina status 1234567890 --watch --interval 1 --port 9090
```

List jobs:

```bash
machina list
machina list --status running
machina list --port 9090
```

## Discover Profiles and Types

List scaffolding profiles:

```bash
machina profile
```

Example output:

```json
[
  "batch",
  "singleRun"
]
```

List registered runtime job types:

```bash
machina types
```

Example output:

```json
[
  "csv_transform",
  "file_encrypt"
]
```

## Generate a New Job

Create a new single-run job:

```bash
machina register singleRun image_cleanup
```

Create a new batch job:

```bash
machina register batch image_resize
```

What happens:

1. Machina creates a temporary scaffold from the selected profile.
2. Your default editor opens.
3. After you save and exit, the file is written to `internal/jobs/<job-name>.go`.
4. The job is registered in `internal/registry/payloadConstructor.go`.

The editor is resolved from:

1. `$VISUAL`
2. `$EDITOR`
3. `nano`, `vim`, or `vi`

If no editor is available, `machina register` exits with an error.

## Create Job Implementations

`machina register` gives you a scaffold. You still need to finish the job implementation before it is useful.

### Choose the Right Profile

Use `singleRun` when the job is one logical unit of work with a single `Run(ctx)` entrypoint.

Use `batch` when the job naturally operates over many independent items and you want the engine to drive:

- discovery with `Scan()`
- chunk sizing with `ChunkSize()`
- concurrent batch execution with `RunBatch(...)`
- result merge with `Aggregate(...)`

### What the Generated File Contains

The generated file is written to:

```text
internal/jobs/<job-name>.go
```

The scaffold includes:

- the job input struct
- the job struct
- a payload constructor
- the execution methods for the selected profile
- TODO comments describing what to replace

### Finish a Single-Run Job

For a `singleRun` job, focus on:

1. defining the input fields your job needs
2. validating input early
3. implementing `Run(ctx)` with the real work
4. returning a clear result value

Typical responsibilities inside `Run(ctx)`:

- check `ctx.Done()` during long-running work
- read input files or payload values
- perform the job's main logic
- write output if needed
- return a result object or summary

### Finish a Batch Job

For a `batch` job, fill in each phase:

1. `Scan()` should discover the work items
2. `ChunkSize()` should return a sensible batch size
3. `RunBatch(ctx, batch)` should process one chunk
4. `Aggregate(results)` should combine partial results into one final result

Use the batch profile when each item can be processed independently.

### Registering and Running the Job

After you save and exit the editor during `machina register`, Machina:

1. saves the source file in `internal/jobs/`
2. updates `internal/registry/payloadConstructor.go`
3. formats the generated files

After that, rebuild the binary:

```bash
go build -o Machina ./cmd
```

Then confirm the job type is registered:

```bash
machina types
```

Important: the current `submit` command is still hardcoded to the built-in aliases `csv-transform` and `file-encrypt`. Newly generated jobs are registered in the runtime registry, but they do not automatically become submittable through `machina submit` until the CLI submit flow is made registry-driven.

## Remove a Generated Job

Unregister a generated job type:

```bash
machina unregister image_cleanup
```

This removes:

- the job registration entry from `internal/registry/payloadConstructor.go`
- the generated job file from `internal/jobs/`

## Help

List top-level commands:

```bash
machina help
```

Show command-specific help:

```bash
machina help start
machina help submit
machina help register
```

## Common Workflows

Start the server, submit work, then watch it finish:

```bash
machina start --port 9090
machina submit csv-transform ./input.csv ./output.csv --port 9090
machina status <job-id> --watch --port 9090
```

Inspect available generation options before adding a custom job:

```bash
machina profile
machina register singleRun thumbnail_cleanup
machina types
```

## Troubleshooting

`missing value for --port`

Provide a value after the flag:

```bash
machina health --port 9090
```

`could not reach server`

Make sure the server is running on the expected port:

```bash
machina start --port 9090
machina health --port 9090
```

`unknown job name`

`submit` only accepts the built-in CLI aliases currently implemented by the binary:

- `csv-transform`
- `file-encrypt`

`job "<name>" is already registered`

Choose a different job name or unregister the existing one first.
