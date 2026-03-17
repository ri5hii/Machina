package worker

import (
	"log/slog"

	"github.com/ri5hii/Machina/internal/jobs"
	"github.com/ri5hii/Machina/internal/storage"
)

type WorkerPool struct {
	workerCount int
	queue       <-chan jobs.JobSubmission
	store       storage.JobStore
	logger      *slog.Logger
}
