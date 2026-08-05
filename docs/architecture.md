# Architecture

> See [README.md](../README.md) for the main philosophy, approach, and CLI.

Machina is built around four components. Each has a single responsibility, and together they form a pipeline from HTTP request to executed work.

## Job

A job is a self-contained unit of work. It owns its input structure, its validation logic, and its processing logic. It has no knowledge of the engine, the queue, or any other job. It is just a struct that implements an interface.

There are two kinds: a `SingleRunJob` with a single `Run` method, and a `BatchProcessingJob` with four methods that the engine drives in sequence. Either way, the job's only job is to describe what work needs to be done and how to do it.

## Payload Constructor

A payload constructor is a function that takes a raw JSON payload — a `map[string]any` from the HTTP request body — and produces a concrete job instance. Its sole responsibility is the translation from untyped wire data into a typed, ready-to-run struct.

```go
type PayloadConstructor func(payload map[string]any) (Runnable, error)
```

Payload constructors exist because the engine receives job submissions as JSON. Something has to bridge the gap between `"type": "file_encrypt"` and a `*FileEncryptJob` with a properly populated `FileEncryptInput`. That is all a payload constructor does. It marshals the payload into the job's input type and returns the job.

## Registry

The registry is a map from job type strings to payload constructors. It is populated once at startup and never changes at runtime.

```
"file_encrypt"  →  fileEncryptPayloadConstructor
"csv_transform" →  csvTransformPayloadConstructor
```

When a submission arrives, the API handler asks the registry for the payload constructor matching the requested type. If the type is unknown, the request is rejected before anything else happens. If it is known, the payload constructor is called. The registry itself has no logic — it is purely a lookup table.

The separation between registry and payload constructor matters: the registry does not know how to build jobs, and payload constructors do not know how to find each other. Adding a new job type means writing a payload constructor and calling `Register` once — nothing else in the system changes.

## Engine

The engine is the runtime. It owns the bounded queue, the worker pool, and the job store. It has three responsibilities: accepting submitted jobs, dispatching them to workers, and tracking their lifecycle state.

When `SubmitJob` is called, the engine assigns the job an ID, stores it as `pending`, and writes it to the queue channel. If the queue is full it fails immediately — it never blocks the caller. Workers pull from that same channel and execute jobs concurrently up to the configured pool size. The engine does not know what any job does. It only calls the interface.

## How They Connect

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

## Separation of Concerns

A job file contains exactly one thing: logic for what to do with its data. It has no knowledge of queues, workers, goroutines, or the store.

The engine has no knowledge of what any job does. It only knows the interface.

This separation means jobs are independently testable, portable, and easy to reason about. Adding a new job type means writing a struct, implementing the interface, and registering a payload constructor — nothing else changes.

## Job Lifecycle

Every job moves through exactly these states:

```
pending → running → completed
                 └→ failed
```

State transitions are managed exclusively by the engine. A job can only influence its own outcome by returning an error or respecting context cancellation.
