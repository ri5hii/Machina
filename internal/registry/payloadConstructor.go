package registry

import (
	"encoding/json"
	"fmt"

	"github.com/ri5hii/Machina/internal/jobs"
)

func fileEncryptPayloadConstructor(payload map[string]any) (jobs.Runnable, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("file_encrypt: failed to encode payload: %w", err)
	}

	var input jobs.FileEncryptInput
	if err := json.Unmarshal(b, &input); err != nil {
		return nil, fmt.Errorf("file_encrypt: failed to decode payload: %w", err)
	}

	return &jobs.FileEncryptJob{Input: input}, nil
}

func csvTransformPayloadConstructor(payload map[string]any) (jobs.Runnable, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("csv_transform: failed to encode payload: %w", err)
	}

	var input jobs.CSVTransformInput
	err = json.Unmarshal(b, &input)
	if err != nil {
		return nil, fmt.Errorf("csv_transform: failed to decode payload: %w", err)
	}

	return &jobs.CSVTransformJob{Input: input}, nil
}

func RegisterAll(r *Registry) {
	r.Register("file_encrypt", fileEncryptPayloadConstructor)
	r.Register("csv_transform", csvTransformPayloadConstructor)
}
