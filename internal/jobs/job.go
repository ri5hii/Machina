// Package jobs defines the job contracts for the Machina engine.
//
// There are two job profiles. Pick one based on your workload:
//
//   - SingleRunJob: one logical unit of work with a single Run(ctx) entrypoint.
//     Use when parallelism within the job is unnecessary.
//
//   - BatchProcessingJob: work over a collection of independent items.
//     The engine drives the full lifecycle: Scan → partition → RunBatch (concurrent) → Aggregate.
//     Use when items can be processed independently and you want engine-managed parallelism.
//
// To add a new job:
//  1. Create a file in this package (e.g. internal/jobs/my_job.go).
//  2. Define an Input struct with json tags matching the expected API payload.
//  3. Define a Result struct for what the job returns.
//  4. Define a Job struct holding Input and any runtime state.
//  5. Implement the chosen interface (SingleRunJob or BatchProcessingJob).
//  6. Optionally add Validate() and JobType() methods.
//  7. Add a PayloadConstructor in internal/registry/payloadConstructor.go.
//  8. Register it in RegisterJobs().
package jobs

import (
	"context"
)

// JobSubmission wraps a job with its assigned ID for the engine queue.
type JobSubmission struct {
	JobID string
	Job   JobRunType
}

// Item is a single unit of work discovered by Scan() and processed by RunBatch().
// It can be any type — the job owns the concrete type and casts inside RunBatch.
type Item any

// JobRunType is a marker interface satisfied by all runnable jobs.
// The engine type-switches on this to dispatch SingleRunJob vs BatchProcessingJob.
type JobRunType interface{}

// SingleRunJob is for work that does not benefit from item-level parallelism.
//
// Implement Run(ctx) with the full job logic. The method receives a context that
// is cancelled when the engine shuts down or when the parent context is done.
// Periodically check ctx.Done() during long-running work.
//
// Example:
//
//	type MyJob struct {
//		Input MyInput
//	}
//
//	func (j *MyJob) Run(ctx context.Context) (any, error) {
//		if err := j.Validate(); err != nil {
//			return nil, err
//		}
//		select {
//		case <-ctx.Done():
//			return nil, ctx.Err()
//		default:
//		}
//		return MyResult{Message: "done"}, nil
//	}
type SingleRunJob interface {
	Run(ctx context.Context) (any, error)
}

// BatchProcessingJob is for work over a collection of independent items.
// The engine drives the full lifecycle — the job only provides domain logic.
//
// Lifecycle:
//  1. Scan()          — discover all work items, return them as []Item
//  2. ChunkSize()     — declare how many items per batch chunk
//  3. RunBatch()      — called concurrently per chunk via errgroup
//  4. Aggregate()     — merge all partial results into one final result
//
// If any chunk fails, the errgroup cancels the shared context and all other
// in-flight chunks stop at their next ctx.Done() check.
//
// Example:
//
//	type MyBatchJob struct {
//		Input MyBatchInput
//	}
//
//	func (j *MyBatchJob) ChunkSize() int { return 4 }
//
//	func (j *MyBatchJob) Scan() ([]Item, error) {
//		// discover items (files, DB rows, API pages, etc.)
//		return items, nil
//	}
//
//	func (j *MyBatchJob) RunBatch(ctx context.Context, batch []Item) (any, error) {
//		// process one chunk concurrently
//		return partialResult, nil
//	}
//
//	func (j *MyBatchJob) Aggregate(results []any) (any, error) {
//		// merge all partial results
//		return finalResult, nil
//	}
type BatchProcessingJob interface {
	// Scan discovers and returns all work items.
	// Called once at the start of execution.
	Scan() ([]Item, error)

	// ChunkSize returns the number of items per batch.
	// The engine partitions Scan() output into chunks of this size.
	// A value <= 0 means all items are processed as one chunk.
	ChunkSize() int

	// RunBatch processes one chunk of items concurrently.
	// Each chunk runs in its own goroutine via errgroup.
	// Keep the return value focused on data that Aggregate can merge.
	RunBatch(ctx context.Context, batch []Item) (any, error)

	// Aggregate merges all chunk results into a single final result.
	// Called once after all chunks complete. Receives one entry per chunk
	// in the order they were dispatched.
	Aggregate(results []any) (any, error)
}
