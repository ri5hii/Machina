# Machina

Machina is a concurrent job execution engine for Go. It decouples work submission from work execution, giving you a structured runtime for asynchronous, resource-controlled processing.

---

## Documentation

| Doc | What it covers |
|---|---|
| [User Guide](USERGUIDE.md) | Command-line usage, building, and common workflows |
| [Architecture](docs/architecture.md) | Components, job contracts, and lifecycle |
| [API](docs/api.md) | HTTP endpoints and built-in jobs |
| [Configuration](docs/configuration.md) | Config precedence, env overrides, and flags |

---

## What It Is

Most systems execute work inline — a request comes in, the work happens, the response goes out. That breaks down when the work is slow, heavy, or unpredictable.

Machina sits between the caller and the work. The caller submits a job and gets an ID back immediately. Machina queues it, assigns it to a worker, executes it, and makes the result available when it's done. The caller never blocks.

---

## When To Use It

Machina is suited for work that is:

- **Slow** — takes longer than a request should wait
- **Heavy** — CPU or I/O intensive enough to need resource caps
- **Batchable** — operates over a collection of independent items
- **Cancellable** — needs to stop cleanly when the context is cancelled

It is not suited for work that needs an immediate, synchronous result.

---

## How It Works

### Two Tiers of Concurrency

Machina runs concurrency at two levels simultaneously.

**Job level** — a fixed worker pool processes multiple jobs in parallel. The pool size is bounded, so the engine never spawns unbounded goroutines regardless of how many jobs are submitted.

**Item level** — within a single job, the engine partitions the job's work into chunks and fans them out in parallel using an `errgroup`. Each chunk runs its own `RunBatch` call concurrently. The job declares how large each chunk should be via `ChunkSize()`.

This means a single job with thousands of items does not monopolise one worker goroutine for its entire duration. The worker dispatches the fan-out and waits, while the real work runs across many goroutines simultaneously.

### Execution Flow

```
Client
  │
  │  POST /jobs  { type, payload }
  ▼
Engine
  ├── registry looks up constructor by type
  ├── constructor builds concrete job from payload
  ├── job is validated
  ├── job is stored and enqueued
  └── worker picks up job
        │
        ├── Scan()
        │     discovers all items, returns []Item
        │
        ├── partition into chunks of ChunkSize()
        │
        ├── errgroup fan-out
        │     ├── RunBatch(ctx, chunk_0)  ─┐
        │     ├── RunBatch(ctx, chunk_1)   ├── concurrent
        │     └── RunBatch(ctx, chunk_N)  ─┘
        │
        └── Aggregate(partials)
              merges all chunk results into a final result
```

If any chunk fails, the errgroup cancels the shared context and all other in-flight chunks stop at their next `ctx.Done()` check.

For how the components behind this fit together, see [Architecture](docs/architecture.md).

---

## CLI

```
machina start                                  Start the engine and HTTP server
machina shutdown [--port]                      Stop the running server
machina health [--port]                        Check server health
machina submit <job> <input> <output>         Submit a job
machina status <id> [--watch] [--port]        Get job status and result
machina list [--status <status>] [--port]     List jobs
machina profile                                List job scaffolding profiles
machina types                                  List registered job types
machina register <profile> <job-name>         Generate and register a job
machina unregister <job-name>                 Remove a registered job
machina config [flags]                         Read or update config.json
machina benchmark [flags]                      Benchmark built-in jobs (JSON output)
```

`profile` returns scaffolding profiles such as `batch` and `singleRun`.
`types` returns registered runtime job types such as `csv_transform` and `file_encrypt`.

For examples and common workflows, see the [User Guide](USERGUIDE.md).

---

## Benchmarks

`machina benchmark` runs both built-in job types through the real engine pipeline (submit → worker pool → complete) and prints a structured JSON report with median throughput. Run it from the repo root so the sample test data is found:

```
machina benchmark
machina benchmark --workers 4 --queue-size 100 --iterations 5
machina benchmark --csv-input tests/data/csv/input/employees_01.csv --folder tests/data/encrypt/input
```


| Benchmark | Config | Median | Throughput |
|---|---|---|---|
| SubmitJobs | 9 workers / queue 8 (default) | 4,879 ns/op | 205k jobs/s |
| SubmitJobs | 4 workers / queue 100 | 3,965 ns/op | 252k jobs/s |
| SubmitJobs | 9 workers / queue 100 | 4,477 ns/op | 223k jobs/s |
| BatchCSV | 9 workers | 17.3 ms per 10k rows | 579k rows/s |
| BatchCSV | 4 workers | 17.9 ms per 10k rows | 558k rows/s |
| FileEncrypt (AES-256-GCM) | 9 workers | 49.7 ms per 104.9 MB | 2.11 GB/s |
| FileEncrypt (AES-256-GCM) | 4 workers | 44.8 ms per 104.9 MB | 2.34 GB/s |

Numbers vary by hardware; the CSV transform is I/O-bound, while AES-256-GCM is limited by AES-NI throughput.
Flags:

| Flag | Default | Description |
|---|---|---|
| `--workers` | `config.json` | Worker pool size |
| `--queue-size` | `config.json` | Bounded queue capacity |
| `--iterations` | `3` | Passes per job type; median is reported |
| `--csv-input` | `tests/data/csv/input/employees_01.csv` | CSV file for `csv_transform` |
| `--folder` | `tests/data/encrypt/input` | Folder for `file_encrypt` |
| `--key` | `tests/data/keys/default.key` | 32-byte AES key |




