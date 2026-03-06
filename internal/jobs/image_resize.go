package jobs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
)

// ImageResizeInput defines the user-supplied configuration for this job.
// The user fills this in at submission time. Machina never reads these fields.
type ImageResizeInput struct {
	FolderPath  string
	OutputPath  string
	Width       int
	Height      int
	Format      string
	WorkerCount int
}

// ImageResizeResult is what the job produces when it completes.
// Machina stores this as `any` — the user asserts it back to this type on retrieval.
type ImageResizeResult struct {
	TotalImages int
	Succeeded   int
	Failed      int
	FailedFiles []string
	OutputPath  string
}

// imageResizeTask is a single unit of internal work — one image to process.
// This is invisible to Machina. The job manages these internally.
type imageResizeTask struct {
	inputPath  string
	outputPath string
	width      int
	height     int
	format     string
}

// imageTaskResult is the outcome of processing one image internally.
type imageTaskResult struct {
	inputPath string
	err       error
}

// ImageResizeJob is a batch processing job that resizes all images in a folder.
//
// From Machina's perspective this is just a Job — it has Run, Validate,
// MaxRetries, and JobType. Machina calls Run(ctx) and waits for the result.
//
// Internally, Run fans out across WorkerCount goroutines to process images
// concurrently. Machina never sees those goroutines.
type ImageResizeJob struct {
	Input ImageResizeInput
}

// JobType identifies this job type in logs, metrics, and the registry.
// Satisfies the Describable optional interface.
func (j *ImageResizeJob) JobType() string {
	return "image_resize"
}

// Validate checks that the job's inputs are valid before it is queued.
// Machina calls this before enqueuing — a validation failure never reaches a worker.
// Satisfies the Validatable optional interface.
func (j *ImageResizeJob) Validate() error {
	if j.Input.FolderPath == "" {
		return fmt.Errorf("image_resize: FolderPath is required")
	}
	if j.Input.OutputPath == "" {
		return fmt.Errorf("image_resize: OutputPath is required")
	}
	if j.Input.Width <= 0 {
		return fmt.Errorf("image_resize: Width must be greater than 0, got %d", j.Input.Width)
	}
	if j.Input.Height <= 0 {
		return fmt.Errorf("image_resize: Height must be greater than 0, got %d", j.Input.Height)
	}
	if j.Input.Format == "" {
		j.Input.Format = "jpeg"
	}
	if j.Input.WorkerCount <= 0 {
		j.Input.WorkerCount = 4
	}
	return nil
}

// MaxRetries defines how many times Machina should retry this job on failure.
// Satisfies the Retryable optional interface.
func (j *ImageResizeJob) MaxRetries() int {
	return 1
}

// Run executes the batch image resize operation.
//
// From Machina's perspective:
//   - Run starts
//   - Run returns (Result, error)
//
// Internally, Run:
//  1. Scans the input folder for images
//  2. Builds a task list — one task per image
//  3. Distributes tasks across WorkerCount goroutines via a task channel
//  4. Each goroutine processes tasks until the channel is drained or ctx is cancelled
//  5. Results are collected and aggregated
//
// Every goroutine checks ctx.Done(). If Machina cancels the job,
// all internal goroutines stop at their next checkpoint.
func (j *ImageResizeJob) Run(ctx context.Context) (any, error) {
	// Step 1: discover all images in the input folder.
	images, err := discoverImages(j.Input.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("image_resize: failed to scan folder %q: %w", j.Input.FolderPath, err)
	}

	if len(images) == 0 {
		return ImageResizeResult{
			TotalImages: 0,
			OutputPath:  j.Input.OutputPath,
		}, nil
	}

	// Ensure output directory exists before workers start writing into it.
	if err := os.MkdirAll(j.Input.OutputPath, 0755); err != nil {
		return nil, fmt.Errorf("image_resize: failed to create output directory %q: %w", j.Input.OutputPath, err)
	}

	// Step 2: build the task list — one task per image.
	tasks := make([]imageResizeTask, len(images))
	for i, imgPath := range images {
		tasks[i] = imageResizeTask{
			inputPath:  imgPath,
			outputPath: j.Input.OutputPath,
			width:      j.Input.Width,
			height:     j.Input.Height,
			format:     j.Input.Format,
		}
	}

	// Step 3: fan out across WorkerCount goroutines.
	//
	// taskCh is the internal work queue. The main goroutine feeds tasks into it.
	// Worker goroutines drain it. This is the same pattern as Machina's own
	// worker pool — but scoped entirely inside this one job.
	taskCh := make(chan imageResizeTask, len(tasks))
	resultCh := make(chan imageTaskResult, len(tasks))

	// Feed all tasks into the channel upfront, then close it.
	// Workers range over taskCh and stop naturally when it is drained.
	for _, task := range tasks {
		taskCh <- task
	}
	close(taskCh)

	// Step 4: start WorkerCount goroutines.
	// Each goroutine:
	//   - ranges over taskCh until it is drained
	//   - checks ctx.Done() before each task
	//   - sends its result to resultCh
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

				err := resizeImage(ctx, task)
				resultCh <- imageTaskResult{
					inputPath: task.inputPath,
					err:       err,
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
	result := ImageResizeResult{
		TotalImages: len(tasks),
		OutputPath:  j.Input.OutputPath,
	}

	for r := range resultCh {
		if r.err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, r.inputPath)
		} else {
			result.Succeeded++
		}
	}

	// If the context was cancelled, surface that as the error.
	// The partial result is still returned so the caller can see what completed.
	if ctx.Err() != nil {
		return result, fmt.Errorf("image_resize: cancelled after processing %d/%d images: %w",
			result.Succeeded+result.Failed, result.TotalImages, ctx.Err())
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Real implementations
// ---------------------------------------------------------------------------

// supportedFormats lists the image extensions this job will process.
var supportedFormats = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".tiff": true,
	".bmp":  true,
}

// discoverImages scans a folder and returns the paths of all supported image files.
func discoverImages(folderPath string) ([]string, error) {
	if folderPath == "" {
		return nil, fmt.Errorf("folder path is empty")
	}

	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", folderPath, err)
	}

	var images []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if supportedFormats[ext] {
			images = append(images, filepath.Join(folderPath, entry.Name()))
		}
	}
	return images, nil
}

// resizeImage performs the actual resize operation on a single image file.
// It reads the source image, resizes it using Lanczos resampling (high quality),
// and writes the result to the output directory with the same filename.
func resizeImage(ctx context.Context, task imageResizeTask) error {
	// Check context before doing any I/O.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Open and decode the source image.
	src, err := imaging.Open(task.inputPath)
	if err != nil {
		return fmt.Errorf("failed to open %q: %w", task.inputPath, err)
	}

	// Check context again after the potentially slow open operation.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Resize using Lanczos resampling — high quality, preserves sharpness.
	// Fit resizes the image to fit within the given dimensions while
	// preserving the aspect ratio. No stretching or cropping occurs.
	resized := imaging.Fit(src, task.width, task.height, imaging.Lanczos)

	// Determine the output format from the task.
	format, err := resolveFormat(task.format)
	if err != nil {
		return err
	}

	// Determine the output filename.
	// Keep the original filename, but change the extension to match the output format.
	baseName := strings.TrimSuffix(filepath.Base(task.inputPath), filepath.Ext(task.inputPath))
	outputExt := formatToExtension(task.format)
	outputFilename := baseName + "_resized" + outputExt
	outputFilePath := filepath.Join(task.outputPath, outputFilename)

	// Check context one more time before the write operation.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Save the resized image.
	if err := imaging.Save(resized, outputFilePath, imaging.JPEGQuality(90), format); err != nil {
		return fmt.Errorf("failed to save resized image to %q: %w", outputFilePath, err)
	}

	return nil
}

// resolveFormat maps a format string to the imaging.Format constant.
func resolveFormat(format string) (imaging.EncodeOption, error) {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return imaging.JPEGQuality(90), nil
	case "png":
		return imaging.PNGCompressionLevel(5), nil
	default:
		return imaging.JPEGQuality(90), nil
	}
}

// formatToExtension maps a format string to its file extension.
func formatToExtension(format string) string {
	switch strings.ToLower(format) {
	case "png":
		return ".png"
	case "jpeg", "jpg":
		return ".jpg"
	default:
		return ".jpg"
	}
}
