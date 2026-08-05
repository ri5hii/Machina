// Package bench is the shared engine benchmark harness used by the CLI and go benchmarks.
package bench

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ri5hii/Machina/internal/engine"
	"github.com/ri5hii/Machina/internal/jobs"
	"github.com/ri5hii/Machina/internal/storage"
)

// BenchmarkOptions controls worker count, queue capacity, and passes per job type.
type BenchmarkOptions struct {
	Workers    int
	QueueSize  int
	Iterations int
}

// BenchmarkResult is the measured outcome for one job type.
type BenchmarkResult struct {
	JobType          string  `json:"jobType"`
	Iterations       int     `json:"iterations"`
	RowsProcessed    int     `json:"rowsProcessed,omitempty"`
	FilesProcessed   int     `json:"filesProcessed,omitempty"`
	BytesProcessed   int64   `json:"bytesProcessed,omitempty"`
	MedianDurationMs float64 `json:"medianDurationMs"`
	RowsPerSec       float64 `json:"rowsPerSec,omitempty"`
	MBPerSec         float64 `json:"mbPerSec,omitempty"`
}

// BenchmarkEngine mirrors the engine configuration used for the run.
type BenchmarkEngine struct {
	WorkerCount int `json:"workerCount"`
	QueueSize   int `json:"queuesize"`
	Iterations  int `json:"iterations"`
}

// BenchmarkReport is the top-level structured JSON output for the benchmark command.
type BenchmarkReport struct {
	Command string            `json:"command"`
	Engine  BenchmarkEngine   `json:"engine"`
	Results []BenchmarkResult `json:"results"`
}

// NewEngine assembles a running engine around a bounded queue and in-memory store.
func NewEngine(ctx context.Context, workers, queueSize int) (*engine.Engine, *storage.JobStore) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewStore()
	eng := engine.New(logger, make(chan jobs.JobSubmission, queueSize), store, workers)
	eng.Start(ctx)
	return eng, store
}

// Submit queues a job, retrying while the bounded queue is saturated so the
// measured time reflects steady-state pipeline throughput under backpressure.
func Submit(eng *engine.Engine, job jobs.JobRunType) (string, error) {
	for {
		// Retrying while the queue is full keeps timing at steady-state throughput.
		id, err := eng.SubmitJob(job)
		if err == nil {
			return id, nil
		}
		if !strings.Contains(err.Error(), "full") {
			return "", err
		}
		runtime.Gosched()
	}
}

// WaitTerminal blocks until the job reaches a terminal state or the timeout hits.
func WaitTerminal(ctx context.Context, store *storage.JobStore, id string, timeout time.Duration) (*storage.JobRecord, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// The store is polled until the job reaches a terminal state.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		record, ok := store.Get(id)
		if ok && (record.JobStatus == storage.StatusCompleted || record.JobStatus == storage.StatusFailed) {
			if record.JobStatus == storage.StatusFailed {
				return record, fmt.Errorf("job %s failed: %w", id, record.Err)
			}
			return record, nil
		}
		time.Sleep(2 * time.Millisecond)
	}
	return nil, fmt.Errorf("timed out waiting for job %s after %s", id, timeout)
}

// SubmitAndWait combines Submit and WaitTerminal for end-to-end job runs.
func SubmitAndWait(ctx context.Context, eng *engine.Engine, store *storage.JobStore, job jobs.JobRunType, timeout time.Duration) (*storage.JobRecord, error) {
	id, err := Submit(eng, job)
	if err != nil {
		return nil, err
	}
	return WaitTerminal(ctx, store, id, timeout)
}

// RunCSVTransform benchmarks csv_transform over the given CSV input file.
func RunCSVTransform(ctx context.Context, opts BenchmarkOptions, inputPath string) (BenchmarkResult, error) {
	result := BenchmarkResult{JobType: "csv_transform", Iterations: opts.Iterations}
	eng, store := NewEngine(ctx, opts.Workers, opts.QueueSize)
	defer eng.Shutdown()

	durations := make([]time.Duration, 0, opts.Iterations)
	for i := 0; i < opts.Iterations; i++ {
		// Each pass writes to a fresh temp dir to keep the repo clean.
		outputDir, err := os.MkdirTemp("", "machina-bench-csv-*")
		if err != nil {
			return result, err
		}
		job := &jobs.CSVTransformJob{Input: jobs.CSVTransformInput{
			InputPath:       inputPath,
			OutputPath:      filepath.Join(outputDir, fmt.Sprintf("out_%d.csv", i)),
			TransformType:   "uppercase",
			HasHeader:       true,
			ColumnSeparator: ",",
		}}
		startTime := time.Now()
		record, err := SubmitAndWait(ctx, eng, store, job, 2*time.Minute)
		durations = append(durations, time.Since(startTime))
		os.RemoveAll(outputDir)
		if err != nil {
			return result, err
		}
		csvResult, ok := record.Result.(jobs.CSVTransformResult)
		if !ok {
			return result, fmt.Errorf("unexpected csv result type: %T", record.Result)
		}
		result.RowsProcessed += csvResult.TotalRows
	}

	medianDuration := median(durations)
	result.MedianDurationMs = float64(medianDuration.Microseconds()) / 1000.0
	result.RowsPerSec = float64(result.RowsProcessed/opts.Iterations) / medianDuration.Seconds()
	return result, nil
}

// RunFileEncrypt benchmarks file_encrypt over every file in the given folder.
func RunFileEncrypt(ctx context.Context, opts BenchmarkOptions, folderPath, keyPath string) (BenchmarkResult, error) {
	result := BenchmarkResult{JobType: "file_encrypt", Iterations: opts.Iterations}
	eng, store := NewEngine(ctx, opts.Workers, opts.QueueSize)
	defer eng.Shutdown()

	durations := make([]time.Duration, 0, opts.Iterations)
	for i := 0; i < opts.Iterations; i++ {
		// Each pass writes ciphertext to a fresh temp dir, then cleans up.
		outputDir, err := os.MkdirTemp("", "machina-bench-encrypt-*")
		if err != nil {
			return result, err
		}
		job := &jobs.FileEncryptJob{Input: jobs.FileEncryptInput{
			FolderPath: folderPath,
			OutputPath: outputDir,
			Algorithm:  "AES-256-GCM",
			KeyPath:    keyPath,
		}}
		startTime := time.Now()
		record, err := SubmitAndWait(ctx, eng, store, job, 5*time.Minute)
		durations = append(durations, time.Since(startTime))
		os.RemoveAll(outputDir)
		if err != nil {
			return result, err
		}
		encryptResult, ok := record.Result.(jobs.FileEncryptResult)
		if !ok {
			return result, fmt.Errorf("unexpected encrypt result type: %T", record.Result)
		}
		result.FilesProcessed += encryptResult.Succeeded
		result.BytesProcessed += encryptResult.BytesProcessed
	}

	medianDuration := median(durations)
	result.MedianDurationMs = float64(medianDuration.Microseconds()) / 1000.0
	result.MBPerSec = (float64(result.BytesProcessed) / float64(opts.Iterations)) / medianDuration.Seconds() / 1e6
	return result, nil
}

// RunAll benchmarks the built-in job types and assembles the structured report.
func RunAll(ctx context.Context, opts BenchmarkOptions, csvInput, folderPath, keyPath string) (BenchmarkReport, error) {
	report := BenchmarkReport{
		Command: "benchmark",
		Engine: BenchmarkEngine{
			WorkerCount: opts.Workers,
			QueueSize:   opts.QueueSize,
			Iterations:  opts.Iterations,
		},
	}

	// Each built-in job type gets its own engine run and median throughput.
	csvResult, err := RunCSVTransform(ctx, opts, csvInput)
	if err != nil {
		return report, err
	}
	encryptResult, err := RunFileEncrypt(ctx, opts, folderPath, keyPath)
	if err != nil {
		return report, err
	}
	report.Results = []BenchmarkResult{csvResult, encryptResult}
	return report, nil
}

// median returns the middle duration, robust to single-pass noise.
func median(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
