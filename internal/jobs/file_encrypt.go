package jobs

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const DefaultKeyPath = "tests/data/keys/default.key"

type FileEncryptInput struct {
	FolderPath string `json:"folder_path"`
	OutputPath string `json:"output_path"`
	Algorithm  string `json:"algorithm"`
	KeyPath    string `json:"key_path"`
}

type FileEncryptResult struct {
	TotalFiles     int
	Succeeded      int
	Failed         int
	FailedFiles    []string
	BytesProcessed int64
	OutputPath     string
}

type FileEncryptJob struct {
	Input FileEncryptInput
	key   []byte
}

// JobType reports the runtime registry key for this job.
func (j *FileEncryptJob) JobType() string { return "file_encrypt" }

// Validate checks the file encryption input before execution starts.
func (j *FileEncryptJob) Validate() error {
	if j.Input.FolderPath == "" {
		return fmt.Errorf("file_encrypt: folder_path is required")
	}
	if j.Input.OutputPath == "" {
		return fmt.Errorf("file_encrypt: output_path is required")
	}
	if j.Input.KeyPath == "" {
		j.Input.KeyPath = DefaultKeyPath
	}
	if j.Input.Algorithm == "" {
		j.Input.Algorithm = "AES-256-GCM"
	}
	if j.Input.Algorithm != "AES-256-GCM" {
		return fmt.Errorf("file_encrypt: unsupported algorithm %q (allowed: AES-256-GCM)", j.Input.Algorithm)
	}
	return nil
}

// MaxRetries reports the retry policy for this job type.
func (j *FileEncryptJob) MaxRetries() int { return 1 }

// ChunkSize controls how many files are processed per batch.
func (j *FileEncryptJob) ChunkSize() int { return 3 }

// Scan discovers input files and turns them into batch work items.
func (j *FileEncryptJob) Scan() ([]Item, error) {
	if j.Input.KeyPath == "" {
		j.Input.KeyPath = DefaultKeyPath
	}
	if j.Input.Algorithm == "" {
		j.Input.Algorithm = "AES-256-GCM"
	}

	if err := os.MkdirAll(j.Input.OutputPath, 0o755); err != nil {
		return nil, fmt.Errorf("file_encrypt: failed to create output directory %q: %w", j.Input.OutputPath, err)
	}

	key, err := loadKey(j.Input.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("file_encrypt: failed to load key %q: %w", j.Input.KeyPath, err)
	}
	j.key = key

	files, err := discoverFiles(j.Input.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("file_encrypt: failed to scan folder %q: %w", j.Input.FolderPath, err)
	}

	items := make([]Item, len(files))
	for i, f := range files {
		items[i] = f
	}
	return items, nil
}

// RunBatch encrypts one chunk of files into the configured output directory.
func (j *FileEncryptJob) RunBatch(ctx context.Context, batch []Item) (any, error) {
	result := FileEncryptResult{
		TotalFiles: len(batch),
		OutputPath: j.Input.OutputPath,
	}

	for _, item := range batch {
		if ctx.Err() != nil {
			return result, fmt.Errorf(
				"file_encrypt: cancelled after processing %d/%d files: %w",
				result.Succeeded+result.Failed,
				result.TotalFiles,
				ctx.Err(),
			)
		}

		filePath := item.(string)
		n, err := encryptFile(ctx, filePath, j.Input.OutputPath, j.key)
		if err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		result.Succeeded++
		result.BytesProcessed += n
	}

	return result, nil
}

// Aggregate combines per-batch byte counts into the final job result.
func (j *FileEncryptJob) Aggregate(partials []any) (any, error) {
	final := FileEncryptResult{
		OutputPath: j.Input.OutputPath,
	}

	for _, p := range partials {
		r := p.(FileEncryptResult)
		final.TotalFiles += r.TotalFiles
		final.Succeeded += r.Succeeded
		final.Failed += r.Failed
		final.BytesProcessed += r.BytesProcessed
		final.FailedFiles = append(final.FailedFiles, r.FailedFiles...)
	}

	return final, nil
}

// discoverFiles walks the input folder and returns every regular file path.
func discoverFiles(folderPath string) ([]string, error) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", folderPath, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, filepath.Join(folderPath, e.Name()))
	}
	return files, nil
}

// loadKey reads the AES key material from disk for batch encryption.
func loadKey(keyPath string) ([]byte, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be exactly 32 bytes for AES-256, got %d", len(key))
	}
	return key, nil
}

// encryptFile encrypts one file and writes the ciphertext into the output directory.
func encryptFile(ctx context.Context, inputPath, outputDir string, key []byte) (int64, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read %q: %w", inputPath, err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return 0, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	outPath := filepath.Join(outputDir, filepath.Base(inputPath)+".enc")
	if err := os.WriteFile(outPath, ciphertext, 0o600); err != nil {
		return 0, fmt.Errorf("failed to write %q: %w", outPath, err)
	}

	return int64(len(plaintext)), nil
}
