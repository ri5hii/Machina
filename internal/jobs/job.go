// Package jobs defines the job contracts for the Machina engine.
//
// Two job profiles exist:
//   - SingleRunJob: one logical unit of work with a single Run(ctx) entrypoint.
//   - BatchProcessingJob: work over a collection of independent items; the
//     engine drives Scan → partition → RunBatch (concurrent) → Aggregate.
//
// New job types are scaffolded with `machina register` (see the User Guide).
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
type SingleRunJob interface {
	Run(ctx context.Context) (any, error)
}

// BatchProcessingJob is for work over a collection of independent items.
// The engine drives the full lifecycle; the job only provides domain logic.
// If any chunk fails, the errgroup cancels the shared context and the rest of
// the in-flight chunks stop at their next ctx.Done() check.
type BatchProcessingJob interface {
	// Scan discovers and returns all work items. Called once at the start of execution.
	Scan() ([]Item, error)

	// ChunkSize returns the number of items per batch. The engine partitions
	// Scan() output into chunks of this size; a value <= 0 means one chunk.
	ChunkSize() int

	// RunBatch processes one chunk of items concurrently via errgroup.
	// Keep the return value focused on data that Aggregate can merge.
	RunBatch(ctx context.Context, batch []Item) (any, error)

	// Aggregate merges all chunk results into a single final result.
	// Called once after all chunks complete, in dispatch order.
	Aggregate(results []any) (any, error)
}
