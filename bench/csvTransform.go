package bench_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ri5hii/Machina/internal/jobs"
	"github.com/ri5hii/Machina/internal/storage"
)

// BenchmarkBatchCSV measures csv_transform rows/sec through the engine pipeline.
func BenchmarkBatchCSV(b *testing.B) {
	inputPath := filepath.Join(repoRoot(), "tests/data/csv/input/employees_01.csv")
	for _, workers := range []int{4, 9, 10} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			eng, store, ctx := newEngine(b, workers, 8)

			b.ResetTimer()
			var rows int
			for i := 0; i < b.N; i++ {
				job := &jobs.CSVTransformJob{Input: jobs.CSVTransformInput{
					InputPath:       inputPath,
					OutputPath:      filepath.Join(b.TempDir(), fmt.Sprintf("out_%d.csv", i)),
					TransformType:   "uppercase",
					HasHeader:       true,
					ColumnSeparator: ",",
				}}
				id := submit(b, eng, job)
				record := waitTerminal(b, ctx, store, id)
				if record.JobStatus != storage.StatusCompleted {
					b.Fatalf("csv job failed: %v", record.Err)
				}
				csvResult, ok := record.Result.(jobs.CSVTransformResult)
				if !ok {
					b.Fatalf("unexpected result type: %T", record.Result)
				}
				rows += csvResult.TotalRows
			}
			b.ReportMetric(float64(rows)/float64(b.N), "rows/op")
		})
	}
}
