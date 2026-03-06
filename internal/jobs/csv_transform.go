package jobs

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"sync"
)

// CSVTransformInput defines the user-supplied configuration for this job.
// The user fills this in at submission time. Machina never reads these fields.
type CSVTransformInput struct {
	InputPath       string
	OutputPath      string
	TransformType   string
	HasHeader       bool
	WorkerCount     int
	ColumnSeparator rune
}

// CSVTransformResult is what the job produces when it completes.
// Machina stores this as `any` — the user asserts it back to this type on retrieval.
type CSVTransformResult struct {
	TotalRows     int
	Succeeded     int
	Failed        int
	FailedRows    []int
	OutputPath    string
	TransformType string
}

// csvRow is a single unit of internal work — one row to transform.
// This is invisible to Machina. The job manages these internally.
type csvRow struct {
	index  int
	fields []string
}

// csvRowResult is the outcome of transforming one row internally.
type csvRowResult struct {
	index          int
	transformedRow []string
	err            error
}

// CSVTransformJob is a batch processing job that reads a CSV file,
// applies a transformation to each row concurrently, and writes the output.
//
// From Machina's perspective this is just a Job — it has Run, Validate,
// MaxRetries, and JobType. Machina calls Run(ctx) and waits for the result.
//
// Internally, Run fans out across WorkerCount goroutines to transform rows
// concurrently. Machina never sees those goroutines.
//
// Supported transform types:
//   - "uppercase"  — converts all string fields to uppercase
//   - "lowercase"  — converts all string fields to lowercase
//   - "trim"       — trims whitespace from all fields
//
// Every goroutine checks ctx.Done(). If Machina cancels the job,
// all internal goroutines stop at their next checkpoint.
type CSVTransformJob struct {
	Input CSVTransformInput
}

// JobType identifies this job type in logs, metrics, and the registry.
// Satisfies the Describable optional interface.
func (j *CSVTransformJob) JobType() string {
	return "csv_transform"
}

// Validate checks that the job's inputs are valid before it is queued.
// Machina calls this before enqueuing — a validation failure never reaches a worker.
// Satisfies the Validatable optional interface.
func (j *CSVTransformJob) Validate() error {
	if j.Input.InputPath == "" {
		return fmt.Errorf("csv_transform: InputPath is required")
	}
	if j.Input.OutputPath == "" {
		return fmt.Errorf("csv_transform: OutputPath is required")
	}
	if j.Input.InputPath == j.Input.OutputPath {
		return fmt.Errorf("csv_transform: InputPath and OutputPath must be different")
	}

	validTransforms := map[string]bool{
		"uppercase": true,
		"lowercase": true,
		"trim":      true,
	}
	if j.Input.TransformType == "" {
		j.Input.TransformType = "trim" // sensible default
	}
	if !validTransforms[j.Input.TransformType] {
		return fmt.Errorf("csv_transform: unknown TransformType %q — must be one of: uppercase, lowercase, trim",
			j.Input.TransformType)
	}

	if j.Input.WorkerCount <= 0 {
		j.Input.WorkerCount = 4 // sensible default
	}
	if j.Input.ColumnSeparator == 0 {
		j.Input.ColumnSeparator = ',' // standard CSV separator
	}
	return nil
}

// MaxRetries defines how many times Machina should retry this job on failure.
// CSV parsing errors are usually deterministic — a corrupt file will fail every time.
// We allow one retry in case of a transient I/O error during file read.
// Satisfies the Retryable optional interface.
func (j *CSVTransformJob) MaxRetries() int {
	return 1
}

// Run executes the batch CSV transformation operation.
//
// This is the only method Machina calls. From Machina's perspective:
//   - Run starts
//   - Run returns (Result, error)
//
// Internally, Run:
//  1. Reads and parses the entire CSV file into rows
//  2. Separates the header row if HasHeader is true
//  3. Distributes rows across WorkerCount goroutines via a task channel
//  4. Each goroutine transforms rows until the channel is drained or ctx is cancelled
//  5. Results are collected, reordered to preserve original row sequence, and written
//
// Row order is preserved in the output regardless of which goroutine processed
// each row — each result carries its original index for reordering.
//
// Every goroutine checks ctx.Done(). If Machina cancels the job,
// all internal goroutines stop at their next checkpoint.
func (j *CSVTransformJob) Run(ctx context.Context) (any, error) {
	// Step 1: read and parse the entire CSV file.
	// In a real implementation this reads from j.Input.InputPath using os.Open.
	// Here we use a simulated CSV string to keep the job self-contained.
	rows, header, err := readCSV(j.Input.InputPath, j.Input.ColumnSeparator, j.Input.HasHeader)
	if err != nil {
		return nil, fmt.Errorf("csv_transform: failed to read %q: %w", j.Input.InputPath, err)
	}

	if len(rows) == 0 {
		return CSVTransformResult{
			TotalRows:     0,
			OutputPath:    j.Input.OutputPath,
			TransformType: j.Input.TransformType,
		}, nil
	}

	// Step 2: build the task list — one task per data row.
	// Header is handled separately and written as-is to the output.
	tasks := make([]csvRow, len(rows))
	for i, row := range rows {
		tasks[i] = csvRow{
			index:  i,
			fields: row,
		}
	}

	// Step 3: fan out across WorkerCount goroutines.
	//
	// taskCh is the internal work queue. The main goroutine feeds tasks into it.
	// Worker goroutines drain it. When taskCh is closed and drained, workers exit.
	//
	// resultCh is buffered to the full row count — workers never block on sending.
	taskCh := make(chan csvRow, len(tasks))
	resultCh := make(chan csvRowResult, len(tasks))

	// Feed all tasks into the channel upfront, then close it.
	// Workers range over taskCh and stop naturally when it is drained.
	for _, task := range tasks {
		taskCh <- task
	}
	close(taskCh)

	// Step 4: start WorkerCount goroutines.
	// Each goroutine:
	//   - ranges over taskCh until it is drained
	//   - checks ctx.Done() before processing each row
	//   - applies the transform function to every field in the row
	//   - sends its outcome to resultCh carrying the original row index
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

				transformed, err := transformRow(task.fields, j.Input.TransformType)
				resultCh <- csvRowResult{
					index:          task.index,
					transformedRow: transformed,
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

	// Step 5: collect results.
	//
	// We preallocate a slice indexed by original row position.
	// This restores the original row order regardless of goroutine scheduling.
	// If a row failed, its slot remains nil and is recorded in FailedRows.
	transformedRows := make([][]string, len(tasks))
	result := CSVTransformResult{
		TotalRows:     len(tasks),
		OutputPath:    j.Input.OutputPath,
		TransformType: j.Input.TransformType,
	}

	for r := range resultCh {
		if r.err != nil {
			result.Failed++
			result.FailedRows = append(result.FailedRows, r.index)
		} else {
			result.Succeeded++
			transformedRows[r.index] = r.transformedRow
		}
	}

	// If the context was cancelled, surface that as the error.
	// The partial result is still returned so the caller can see progress.
	if ctx.Err() != nil {
		return result, fmt.Errorf("csv_transform: cancelled after processing %d/%d rows: %w",
			result.Succeeded+result.Failed, result.TotalRows, ctx.Err())
	}

	// Step 6: write the output CSV.
	// In a real implementation this writes to j.Input.OutputPath using os.Create.
	// Here we simulate the write to keep the job self-contained.
	if err := writeCSV(j.Input.OutputPath, header, transformedRows, j.Input.ColumnSeparator); err != nil {
		return result, fmt.Errorf("csv_transform: failed to write output %q: %w", j.Input.OutputPath, err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Transform functions
//
// Each function applies a transformation to a single row's fields.
// Adding a new transform type means adding a case here — no other changes needed.
// ---------------------------------------------------------------------------

// transformRow applies the named transform to every field in a row.
func transformRow(fields []string, transformType string) ([]string, error) {
	result := make([]string, len(fields))
	for i, field := range fields {
		switch transformType {
		case "uppercase":
			result[i] = strings.ToUpper(field)
		case "lowercase":
			result[i] = strings.ToLower(field)
		case "trim":
			result[i] = strings.TrimSpace(field)
		default:
			return nil, fmt.Errorf("unknown transform type: %q", transformType)
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Stub implementations
//
// These stand in for real I/O operations. In a production implementation:
//   - readCSV would open the file with os.Open and parse with encoding/csv
//   - writeCSV would create the file with os.Create and write with encoding/csv
//
// They are kept as stubs so the job compiles and the concurrency pattern
// is fully readable without external dependencies.
// ---------------------------------------------------------------------------

// readCSV reads and parses a CSV file.
// Returns the data rows, the header row (if hasHeader is true), and any error.
// Stub: returns simulated CSV data for demonstration.
func readCSV(inputPath string, separator rune, hasHeader bool) ([][]string, []string, error) {
	if inputPath == "" {
		return nil, nil, fmt.Errorf("input path is empty")
	}

	// Simulate a CSV with 10 data rows and 3 columns.
	raw := `name, city, country
alice , new york , usa
BOB , london , uk
  carol , paris , france
dave,berlin,germany
eve , tokyo , japan
FRANK , sydney , australia
grace , toronto , canada
henry , dubai , uae
iris , singapore , sg
jack , amsterdam , nl`

	reader := csv.NewReader(strings.NewReader(raw))
	reader.Comma = separator
	reader.TrimLeadingSpace = false

	allRows, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(allRows) == 0 {
		return nil, nil, nil
	}

	var header []string
	dataRows := allRows

	if hasHeader && len(allRows) > 0 {
		header = allRows[0]
		dataRows = allRows[1:]
	}

	return dataRows, header, nil
}

// writeCSV writes transformed rows to the output path.
// The header row is written first if non-nil.
// Stub: simulates the write without real I/O.
func writeCSV(outputPath string, header []string, rows [][]string, separator rune) error {
	if outputPath == "" {
		return fmt.Errorf("output path is empty")
	}

	// In a real implementation:
	//   f, err := os.Create(outputPath)
	//   defer f.Close()
	//   w := csv.NewWriter(f)
	//   w.Comma = separator
	//   if header != nil { w.Write(header) }
	//   for _, row := range rows {
	//       if row != nil { w.Write(row) }
	//   }
	//   w.Flush()
	//   return w.Error()

	// Simulate successful write.
	rowCount := 0
	for _, row := range rows {
		if row != nil {
			rowCount++
		}
	}
	_ = fmt.Sprintf("writing %d rows to %s", rowCount, outputPath)
	return nil
}
