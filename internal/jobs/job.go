package jobs

import "context"

type Job interface {
	Run(ctx context.Context) (any, error)
}

type BatchProcessingJob interface {
	Scan() ([]Item, error)
	ChunkSize() int
	RunBatch(ctx context.Context, batch []Item) (any, error)
	Aggregate(results []any) (any, error)
}

type Runnable interface{}

type Submission struct {
	ID  string
	Job Runnable
}

type Item any
