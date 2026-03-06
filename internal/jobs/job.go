package jobs

import (
	"context"
)

type Job interface {
	Run(ctx context.Context) (any, error)
}

type Submission struct {
	ID string
	Job Job
}

type Item any

type BatchProcessingJob interface {
	Job
	Scan() ([]Item, error)
	RunBatch(ctx context.Context, batch []Item) (any, error)
	Aggregate(results []any) (any, error)
}
