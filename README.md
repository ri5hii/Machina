# Machina

Machina is a modular, concurrent job execution engine for Go. It accepts user-defined workloads, schedules them intelligently, and executes them asynchronously with controlled resource utilization.

---

## Overview

Machina provides a structured runtime for processing tasks that are:

- **Long running** — work that takes seconds, minutes, or longer
- **Compute intensive** — CPU or I/O heavy operations
- **Failure prone** — tasks that may fail and need retry logic
- **Latency insensitive** — work that does not need an immediate response

Machina decouples task submission from task execution, enabling responsive APIs while maintaining high throughput and predictable performance.

---

## How It Works

Machina acts as an execution layer between clients and workloads. Instead of executing work inline, Machina:

1. Receives a job request
2. Validates and registers it
3. Schedules it
4. Executes it using controlled concurrency
5. Tracks lifecycle state
6. Exposes results and telemetry

This architecture allows systems to process heavy or asynchronous work without blocking request paths.

```
Client
  │
  │  submit job
  ▼
Machina Engine
  ├── validate & register
  ├── schedule
  ├── execute (controlled concurrency)
  ├── track state
  └── expose results / telemetry
```

---

## Target Workloads

Machina is designed to execute workloads that benefit from asynchronous execution, including:

| Workload Type       | Description                                      |
|---------------------|--------------------------------------------------|
| Compute tasks        | CPU-bound algorithms, encoding, transformation  |
| Data processing      | ETL, aggregation, parsing large datasets         |
| Network operations   | Bulk API calls, scraping, distributed fetches    |
| Batch pipelines      | Multi-step sbatch-style job chains               |
| Staged workflows     | Jobs with ordered, dependent execution phases    |
| Retryable operations | Tasks that tolerate and recover from failure     |

It is optimized for workloads where **execution control matters more than immediate response**.

---

## Runtime Guarantees

Machina follows strict runtime guarantees to ensure stability under high workload volume:

| Guarantee                      | Description                                                     |
|--------------------------------|-----------------------------------------------------------------|
| Bounded execution parallelism  | Concurrency is capped — no goroutine explosion                  |
| Cooperative cancellation       | Jobs respect `context.Context` and stop cleanly when cancelled  |
| No unbounded resource growth   | Memory and goroutine usage stay predictable under load          |
| Predictable scheduling         | Jobs are scheduled in a consistent, fair order                  |
| Failure containment            | A failing job does not affect the engine or sibling jobs        |
| Consistent state tracking      | Every job moves through a well-defined lifecycle                |

---

## Architecture: Separation of Concerns

Machina cleanly separates engine responsibilities from job responsibilities.

### The Engine's Responsibility

The engine handles the infrastructure layer:

- Scheduling
- Concurrency control
- Lifecycle transitions
- Cancellation propagation
- State tracking
- Metrics and telemetry

The engine does not care what a job does. It only cares that the job conforms to the expected interface.

### The Job's Responsibility

Each job type is a self-contained unit that owns:

- Its own processing logic
- Its own input validation
- Its own execution steps
- Its own internal state

### What Machina Does at Runtime

When a job is submitted, Machina:

1. Looks up the job type in the registry
2. Calls the registered factory function
3. Creates a concrete job instance
4. Stores it
5. Enqueues it for execution

The job instance carries its own data from that point forward.

### Job Contract

For a job to work correctly within Machina, it should:

- Own its payload structure
- Own its validation logic
- Own its execution steps
- Periodically check `ctx.Done()` to support cancellation
- Return a typed result upon completion

This makes each job an independent, portable unit of work.

---

## Division of Labor

| Machina Provides          | Jobs Provide              |
|---------------------------|---------------------------|
| Concurrency               | Domain logic              |
| Scheduling                | Computation               |
| Lifecycle control         | Task-specific behavior    |
| Cancellation propagation  | Input validation          |
| State tracking            | Result structure           |

---

## Example: Video Frame Extraction

To illustrate how Machina is used in practice, consider a user who wants to extract frames from a video file as images.

### The Workload

Video frame extraction is a well-suited Machina workload because it is:

- CPU and I/O heavy
- Long running
- Cancellable mid-stream
- Chunkable into segments
- Parallelizable across multiple videos

### The Separation

> Machina is not the tool that extracts frames.
> Machina is the system that **runs the job** that extracts frames.

```
User
  │
  │  submit FrameExtractionJob { videoPath, outputDir, fps }
  ▼
Machina Engine
  ├── validates job input
  ├── enqueues job
  ├── assigns worker
  └── executes FrameExtractionJob.Run(ctx)
        │
        ├── opens video file
        ├── decodes frames in a loop
        │     └── checks ctx.Done() on each iteration
        ├── writes frame images to outputDir
        └── returns result { frameCount, duration, outputDir }
```

### What the User Defines

The user (or library author) implements the job:

```go
type FrameExtractionJob struct {
    VideoPath string
    OutputDir string
    FPS       int
}

func (j *FrameExtractionJob) Validate() error {
    // check that file exists, fps > 0, etc.
}

func (j *FrameExtractionJob) Run(ctx context.Context) (Result, error) {
    for each frame {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
            // decode and write frame
        }
    }
    return FrameResult{FrameCount: n}, nil
}
```

### What the User Submits

```go
jobID, err := engine.Submit(FrameExtractionJob{
    VideoPath: "/media/input.mp4",
    OutputDir: "/media/frames/",
    FPS:       24,
})
```

### What Machina Handles Automatically

- Queuing the job without blocking the caller
- Respecting the concurrency limit across all running jobs
- Propagating cancellation if the job is cancelled or the engine shuts down
- Tracking the job through `Pending → Running → Completed` (or `Failed`)
- Making the result available for retrieval after completion

