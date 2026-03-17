package jobs

import (
	"context"
)

type JobSubmission struct {
	JobID string
	Job   JobType
}

type Item any

type JobType interface{}

type BatchProcessingJob interface {
	Scan() ([]Item, error)
	ChunkSize() int
	RunBatch(ctx context.Context, batch []Item) (any, error)
	Aggregate(results []any) (any, error)
}

type ParallelProcessingJob interface {
	Run(ctx context.Context) (any, error)
}
