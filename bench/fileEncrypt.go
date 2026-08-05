package bench_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ri5hii/Machina/internal/jobs"
	"github.com/ri5hii/Machina/internal/storage"
)

// BenchmarkEncrypt measures AES-256-GCM MB/s over the full test corpus.
func BenchmarkEncrypt(b *testing.B) {
	folderPath := filepath.Join(repoRoot(), "tests/data/encrypt/input")
	keyPath := filepath.Join(repoRoot(), "tests/data/keys/default.key")
	for _, workers := range []int{4, 9, 10} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			eng, store, ctx := newEngine(b, workers, 8)

			b.ResetTimer()
			var bytesProcessed int64
			for i := 0; i < b.N; i++ {
				job := &jobs.FileEncryptJob{Input: jobs.FileEncryptInput{
					FolderPath: folderPath,
					OutputPath: b.TempDir(),
					Algorithm:  "AES-256-GCM",
					KeyPath:    keyPath,
				}}
				id := submit(b, eng, job)
				record := waitTerminal(b, ctx, store, id)
				if record.JobStatus != storage.StatusCompleted {
					b.Fatalf("encrypt job failed: %v", record.Err)
				}
				encryptResult, ok := record.Result.(jobs.FileEncryptResult)
				if !ok {
					b.Fatalf("unexpected result type: %T", record.Result)
				}
				if encryptResult.Succeeded == 0 {
					b.Fatalf("encrypt job processed zero files")
				}
				bytesProcessed += encryptResult.BytesProcessed
			}
			b.ReportMetric(float64(bytesProcessed)/float64(b.N), "bytes/op")
		})
	}
}
