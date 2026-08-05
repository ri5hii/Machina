// Shared helpers for the bench_test package plus the dispatch-throughput benchmark.
package bench_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ri5hii/Machina/internal/bench"
	"github.com/ri5hii/Machina/internal/engine"
	"github.com/ri5hii/Machina/internal/jobs"
	"github.com/ri5hii/Machina/internal/storage"
)

// noopJob is a minimal SingleRunJob used to isolate dispatch throughput.
type noopJob struct{}

func (noopJob) Run(ctx context.Context) (any, error) { return "ok", nil }

// repoRoot resolves the repo root so benchmarks find the committed sample data.
func repoRoot() string {
	workingDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(workingDir, "..")
}

// newEngine builds a running engine and tears it down after the benchmark.
func newEngine(t testing.TB, workers, queueSize int) (*engine.Engine, *storage.JobStore, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	eng, store := bench.NewEngine(ctx, workers, queueSize)
	t.Cleanup(func() {
		cancel()
		eng.Shutdown()
	})
	return eng, store, ctx
}

// submit queues a job and fails the benchmark on unexpected errors.
func submit(t testing.TB, eng *engine.Engine, job jobs.JobRunType) string {
	id, err := bench.Submit(eng, job)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	return id
}

// waitTerminal blocks until the job completes and fails on error/timeout.
func waitTerminal(t testing.TB, ctx context.Context, store *storage.JobStore, id string) *storage.JobRecord {
	record, err := bench.WaitTerminal(ctx, store, id, 2*time.Minute)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	return record
}

// BenchmarkSubmitJobs measures sustained dispatch throughput under backpressure.
func BenchmarkSubmitJobs(b *testing.B) {
	for _, workers := range []int{4, 9, 10} {
		for _, queueSize := range []int{8, 100} {
			b.Run(fmt.Sprintf("workers=%d/queue=%d", workers, queueSize), func(b *testing.B) {
				eng, store, _ := newEngine(b, workers, queueSize)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					submit(b, eng, noopJob{})
				}
				b.StopTimer()

				waitDrain(b, store, b.N)
				b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "jobs/s")
			})
		}
	}
}

// waitDrain polls the store until all submitted jobs reach a terminal state.
func waitDrain(b *testing.B, store *storage.JobStore, want int) {
	deadline := time.Now().Add(2 * time.Minute)
	terminal := 0
	for time.Now().Before(deadline) {
		terminal = 0
		for _, record := range store.List() {
			if record.JobStatus == storage.StatusCompleted || record.JobStatus == storage.StatusFailed {
				terminal++
			}
		}
		if terminal >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	b.Fatalf("drain timeout: terminal=%d want=%d", terminal, want)
}
