package jobs

import (
	"context"
	"fmt"
	"sync"
)

// FileEncryptInput defines the user-supplied configuration for this job.
// The user fills this in at submission time. Machina never reads these fields.
type FileEncryptInput struct {
	FolderPath  string
	OutputPath  string
	Algorithm   string
	KeyPath     string
	WorkerCount int
}

// FileEncryptResult is what the job produces when it completes.
// Machina stores this as `any` — the user asserts it back to this type on retrieval.
type FileEncryptResult struct {
	TotalFiles     int
	Succeeded      int
	Failed         int
	FailedFiles    []string
	BytesProcessed int64
	OutputPath     string
}

// fileEncryptTask is a single unit of internal work — one file to encrypt.
// This is invisible to Machina. The job manages these internally.
type fileEncryptTask struct {
	inputPath  string
	outputPath string
	algorithm  string
	keyPath    string
}

// fileEncryptTaskResult is the outcome of encrypting one file internally.
type fileEncryptTaskResult struct {
	inputPath      string
	bytesProcessed int64
	err            error
}

// FileEncryptJob is a batch processing job that encrypts all files in a folder.
//
// From Machina's perspective this is just a Job — it has Run, Validate,
// MaxRetries, and JobType. Machina calls Run(ctx) and waits for the result.
//
// Internally, Run fans out across WorkerCount goroutines to encrypt files
// concurrently. Machina never sees those goroutines.
//
// Each goroutine respects ctx.Done() — if Machina cancels the job,
// all internal goroutines stop at their next checkpoint.
type FileEncryptJob struct {
	Input FileEncryptInput
}

// JobType identifies this job type in logs, metrics, and the registry.
// Satisfies the Describable optional interface.
func (j *FileEncryptJob) JobType() string {
	return "file_encrypt"
}

// Validate checks that the job's inputs are valid before it is queued.
// Machina calls this before enqueuing — a validation failure never reaches a worker.
// Satisfies the Validatable optional interface.
func (j *FileEncryptJob) Validate() error {
	if j.Input.FolderPath == "" {
		return fmt.Errorf("file_encrypt: FolderPath is required")
	}
	if j.Input.OutputPath == "" {
		return fmt.Errorf("file_encrypt: OutputPath is required")
	}
	if j.Input.KeyPath == "" {
		return fmt.Errorf("file_encrypt: KeyPath is required — encryption requires a key")
	}
	if j.Input.Algorithm == "" {
		j.Input.Algorithm = "AES-256-GCM" // sensible default
	}
	if j.Input.WorkerCount <= 0 {
		j.Input.WorkerCount = 4 // sensible default
	}
	return nil
}

// MaxRetries defines how many times Machina should retry this job on failure.
// Encryption failures are typically due to bad keys or corrupt files — not transient.
// We allow one retry in case of a transient I/O error during folder scanning.
// Satisfies the Retryable optional interface.
func (j *FileEncryptJob) MaxRetries() int {
	return 1
}

// Run executes the batch file encryption operation.
//
// This is the only method Machina calls. From Machina's perspective:
//   - Run starts
//   - Run returns (Result, error)
//
// Internally, Run:
//  1. Scans the input folder for files
//  2. Builds a task list — one task per file
//  3. Distributes tasks across WorkerCount goroutines via a task channel
//  4. Each goroutine encrypts files until the channel is drained or ctx is cancelled
//  5. Results are collected and aggregated into a single FileEncryptResult
//
// Every goroutine checks ctx.Done(). If Machina cancels the job,
// all internal goroutines stop at their next checkpoint.
func (j *FileEncryptJob) Run(ctx context.Context) (any, error) {
	// Step 1: discover all files in the input folder.
	// In a real implementation this would use os.ReadDir.
	// Here we simulate it to keep the job focused on the concurrency pattern.
	files, err := discoverFiles(j.Input.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("file_encrypt: failed to scan folder %q: %w", j.Input.FolderPath, err)
	}

	if len(files) == 0 {
		return FileEncryptResult{
			TotalFiles: 0,
			OutputPath: j.Input.OutputPath,
		}, nil
	}

	// Step 2: build the task list — one task per file.
	tasks := make([]fileEncryptTask, len(files))
	for i, filePath := range files {
		tasks[i] = fileEncryptTask{
			inputPath:  filePath,
			outputPath: j.Input.OutputPath,
			algorithm:  j.Input.Algorithm,
			keyPath:    j.Input.KeyPath,
		}
	}

	// Step 3: fan out across WorkerCount goroutines.
	//
	// taskCh is the internal work queue. The main goroutine feeds tasks into it.
	// Worker goroutines drain it. When taskCh is closed and drained, workers exit.
	taskCh := make(chan fileEncryptTask, len(tasks))
	resultCh := make(chan fileEncryptTaskResult, len(tasks))

	// Feed all tasks into the channel upfront, then close it.
	// Workers range over taskCh and stop naturally when it is drained.
	for _, task := range tasks {
		taskCh <- task
	}
	close(taskCh)

	// Step 4: start WorkerCount goroutines.
	// Each goroutine:
	//   - ranges over taskCh until it is drained
	//   - checks ctx.Done() before processing each file
	//   - sends its outcome to resultCh
	var wg sync.WaitGroup
	for i := 0; i < j.Input.WorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskCh {
				// Cooperative cancellation check.
				// If Machina cancelled this job, stop immediately.
				select {
				case <-ctx.Done():
					return
				default:
				}

				bytesProcessed, err := encryptFile(ctx, task)
				resultCh <- fileEncryptTaskResult{
					inputPath:      task.inputPath,
					bytesProcessed: bytesProcessed,
					err:            err,
				}
			}
		}()
	}

	// Close resultCh once all workers have finished.
	// This allows the result collection loop below to terminate naturally.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Step 5: collect and aggregate results.
	result := FileEncryptResult{
		TotalFiles: len(tasks),
		OutputPath: j.Input.OutputPath,
	}

	for r := range resultCh {
		if r.err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, r.inputPath)
		} else {
			result.Succeeded++
			result.BytesProcessed += r.bytesProcessed
		}
	}

	// If the context was cancelled, surface that as the error.
	// The partial result is still returned so the caller can see what completed.
	if ctx.Err() != nil {
		return result, fmt.Errorf("file_encrypt: cancelled after processing %d/%d files: %w",
			result.Succeeded+result.Failed, result.TotalFiles, ctx.Err())
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Stub implementations
//
// These stand in for real I/O operations. In a production implementation:
//   - discoverFiles would use os.ReadDir
//   - encryptFile would use crypto/aes + crypto/cipher for AES-256-GCM
//     or call out to an encryption library
//
// They are kept as stubs so the job compiles and the concurrency pattern
// is fully readable without external dependencies.
// ---------------------------------------------------------------------------

// discoverFiles scans a folder and returns the paths of all files.
// Stub: returns simulated file paths for demonstration.
func discoverFiles(folderPath string) ([]string, error) {
	if folderPath == "" {
		return nil, fmt.Errorf("folder path is empty")
	}
	// Simulate finding 10 files in the folder.
	files := make([]string, 10)
	for i := range files {
		files[i] = fmt.Sprintf("%s/file_%04d.dat", folderPath, i+1)
	}
	return files, nil
}

// encryptFile performs the encryption operation on a single file.
// Returns the number of bytes processed and any error encountered.
// Stub: simulates the operation without real I/O or cryptography.
func encryptFile(ctx context.Context, task fileEncryptTask) (int64, error) {
	// In a real implementation:
	//   plaintext, err := os.ReadFile(task.inputPath)
	//   key, err := os.ReadFile(task.keyPath)
	//   ciphertext, err := aesGCMEncrypt(key, plaintext)
	//   err = os.WriteFile(outputFilePath, ciphertext, 0600)
	//   return int64(len(plaintext)), err

	// Check context before doing work.
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	// Simulate 1KB of data encrypted per file.
	const simulatedFileSize int64 = 1024
	_ = fmt.Sprintf("encrypting %s using %s with key %s → %s",
		task.inputPath, task.algorithm, task.keyPath, task.outputPath)
	return simulatedFileSize, nil
}
