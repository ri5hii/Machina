# Machina

Machina is a concurrent job execution engine for Go. It decouples work submission from work execution, giving you a structured runtime for asynchronous, resource-controlled processing.

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

---

## Architecture

Machina is built around four components. Each has a single responsibility, and together they form a pipeline from HTTP request to executed work.

### Job

A job is a self-contained unit of work. It owns its input structure, its validation logic, and its processing logic. It has no knowledge of the engine, the queue, or any other job. It is just a struct that implements an interface.

There are two kinds: a `SingleRunJob` with a single `Run` method, and a `BatchProcessingJob` with four methods that the engine drives in sequence. Either way, the job's only job is to describe what work needs to be done and how to do it.

### Payload Constructor

A payload constructor is a function that takes a raw JSON payload — a `map[string]any` from the HTTP request body — and produces a concrete job instance. Its sole responsibility is the translation from untyped wire data into a typed, ready-to-run struct.

```go
type PayloadConstructor func(payload map[string]any) (Runnable, error)
```

Payload constructors exist because the engine receives job submissions as JSON. Something has to bridge the gap between `"type": "file_encrypt"` and a `*FileEncryptJob` with a properly populated `FileEncryptInput`. That is all a payload constructor does. It marshals the payload into the job's input type and returns the job.

### Registry

The registry is a map from job type strings to payload constructors. It is populated once at startup and never changes at runtime.

```
"file_encrypt"  →  fileEncryptPayloadConstructor
"csv_transform" →  csvTransformPayloadConstructor
```

When a submission arrives, the API handler asks the registry for the payload constructor matching the requested type. If the type is unknown, the request is rejected before anything else happens. If it is known, the payload constructor is called. The registry itself has no logic — it is purely a lookup table.

The separation between registry and payload constructor matters: the registry does not know how to build jobs, and payload constructors do not know how to find each other. Adding a new job type means writing a payload constructor and calling `Register` once — nothing else in the system changes.

### Engine

The engine is the runtime. It owns the bounded queue, the worker pool, and the job store. It has three responsibilities: accepting submitted jobs, dispatching them to workers, and tracking their lifecycle state.

When `SubmitJob` is called, the engine assigns the job an ID, stores it as `pending`, and writes it to the queue channel. If the queue is full it fails immediately — it never blocks the caller. Workers pull from that same channel and execute jobs concurrently up to the configured pool size. The engine does not know what any job does. It only calls the interface.

### How They Connect

```
POST /jobs { "type": "csv_transform", "payload": { ... } }
     │
     ▼
Registry              looks up "csv_transform" → csvTransformPayloadConstructor
     │
     ▼
PayloadConstructor    unmarshals payload → *CSVTransformJob{Input: ...}
     │
     ▼
Engine                assigns ID, stores as pending, enqueues Submission{ID, job}
     │
     ▼
Worker                dequeues, detects BatchProcessingJob, calls executeBatch
     │
     ▼
Job                   Scan → RunBatch (×N, concurrent) → Aggregate
```

Each component only knows about the one to its right through an interface or a function signature. The registry never imports the engine. The engine never imports a specific job type. Jobs never import anything from the engine. This is what makes each piece independently testable and the system as a whole easy to extend.

---

## Job Contracts

There are two kinds of jobs.

### Single-Run Job

For work that does not benefit from item-level parallelism. Implements a single method:

```go
type SingleRunJob interface {
    Run(ctx context.Context) (any, error)
}
```

### Batch Processing Job

For work over a collection of independent items. The engine drives the full lifecycle — the job only provides the domain logic for each phase:

```go
type BatchProcessingJob interface {
    Scan()                                      ([]Item, error)
    ChunkSize()                                 int
    RunBatch(ctx context.Context, batch []Item) (any, error)
    Aggregate(results []any)                    (any, error)
}
```

`Scan` discovers and returns all items. `ChunkSize` tells the engine how to partition them. `RunBatch` processes one chunk — it is called concurrently across all chunks. `Aggregate` receives all partial results and merges them into a final result. It runs once, after all chunks complete.

The job never calls `RunBatch` itself. It never manages goroutines or synchronisation. It only implements the four methods and lets the engine handle the rest.

### What the Job Owns

| Job owns | Engine owns |
|---|---|
| Domain logic | Scheduling |
| Input validation | Worker lifecycle |
| Per-item processing | Chunk partitioning |
| Result structure | Parallel fan-out |
| Cancellation checks | Context propagation |

---

## Separation of Concerns

A job file contains exactly one thing: logic for what to do with its data. It has no knowledge of queues, workers, goroutines, or the store.

The engine has no knowledge of what any job does. It only knows the interface.

This separation means jobs are independently testable, portable, and easy to reason about. Adding a new job type means writing a struct, implementing the interface, and registering a payload constructor — nothing else changes.

---

## Job Lifecycle

Every job moves through exactly these states:

```
pending → running → completed
                 └→ failed
```

State transitions are managed exclusively by the engine. A job can only influence its own outcome by returning an error or respecting context cancellation.

---

## API

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/jobs` | Submit a job |
| `GET` | `/jobs` | List all jobs |
| `GET` | `/jobs/:id` | Get job status and result |
| `GET` | `/health` | Engine health check |

### Submit a job

```
POST /jobs
{
  "type": "file_encrypt",
  "payload": {
    "folder_path": "/data/input",
    "output_path": "/data/output"
  }
}
```

Response:

```
202 Accepted
{ "id": "1234567890", "status": "pending" }
```

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

---

## Built-in Jobs

| Job | Type | What it does |
|---|---|---|
| File encrypt | `file_encrypt` | AES-256-GCM encrypts every file in a folder |
| CSV transform | `csv_transform` | Applies uppercase, lowercase, or trim to every row in a CSV |

---

## Benchmarks

`machina benchmark` runs both built-in job types through the real engine pipeline (submit → worker pool → complete) and prints a structured JSON report with median throughput. Run it from the repo root so the sample test data is found:

```
machina benchmark
machina benchmark --workers 4 --queue-size 100 --iterations 5
machina benchmark --csv-input tests/data/csv/input/employees_01.csv --folder tests/data/encrypt/input
```

Flags:

| Flag | Default | Description |
|---|---|---|
| `--workers` | `config.json` | Worker pool size |
| `--queue-size` | `config.json` | Bounded queue capacity |
| `--iterations` | `3` | Passes per job type; median is reported |
| `--csv-input` | `tests/data/csv/input/employees_01.csv` | CSV file for `csv_transform` |
| `--folder` | `tests/data/encrypt/input` | Folder for `file_encrypt` |
| `--key` | `tests/data/keys/default.key` | 32-byte AES key |

Measured results (medians of 3 runs, 12th Gen Intel i7-12700H, from `bench/benchmark-results.txt`):

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

---

## Configuration

Machina reads defaults from `config.json`. For commands that accept `--port`, precedence is:

```
flag override -> config.json -> 8080 fallback
```

At startup, `machina start` also accepts environment overrides.

| Flag | Env | Default | Description |
|---|---|---|---|
| `--port` | `PORT` | `8080` | HTTP listen port |
| `--workers` | `WORKER_COUNT` | `4` | Number of worker goroutines |
| `--queue-size` | `QUEUE_SIZE` | `100` | Maximum queued jobs |
| `--log-level` | `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN`, `ERROR` |
