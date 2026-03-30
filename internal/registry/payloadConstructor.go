package registry

import (
	"encoding/json"
	"fmt"

	"github.com/ri5hii/Machina/internal/jobs"
)

// EncryptFilePayloadConstructor converts a raw API payload into a FileEncryptJob.
func EncryptFilePayloadConstructor(payload map[string]any) (jobs.JobRunType, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("file_encrypt: Failed to encode payload: %w", err)
	}

	var input jobs.FileEncryptInput
	err = json.Unmarshal(b, &input)
	if err != nil {
		return nil, fmt.Errorf("file_encrypt: Failed to decode payload: %w", err)
	}

	return &jobs.FileEncryptJob{Input: input}, nil

}

// CSVTransformPayloadConstructor converts a raw API payload into a CSVTransformJob.
func CSVTransformPayloadConstructor(payload map[string]any) (jobs.JobRunType, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("csv_transform: Failed to encode payload: %w", err)
	}

	var input jobs.CSVTransformInput
	err = json.Unmarshal(b, &input)
	if err != nil {
		return nil, fmt.Errorf("csv_transform: Failed to decode payload: %w", err)
	}

	return &jobs.CSVTransformJob{Input: input}, nil
}

// RegisterJobs registers the built-in runtime job types.

func (reg *Registry) RegisterJobs() {
	reg.Register("file_encrypt", EncryptFilePayloadConstructor)
	reg.Register("csv_transform", CSVTransformPayloadConstructor)

}
