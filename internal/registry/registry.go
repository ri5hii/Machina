package registry

import (
	"github.com/ri5hii/Machina/internal/jobs"
)

type PayloadConstructor func(payload map[string]any) (jobs.Runnable, error)

type Registry struct {
	constructors map[string]PayloadConstructor
}

func New() *Registry {
	return &Registry{
		constructors: make(map[string]PayloadConstructor),
	}
}

func (r *Registry) Register(jobType string, payloadConstructor PayloadConstructor) {
	_, exists := r.constructors[jobType]
	if exists {
		panic("registry: job type already registered: " + jobType)
	}
	r.constructors[jobType] = payloadConstructor
}

func (r *Registry) GetPayloadConstructor(jobType string) (PayloadConstructor, bool) {
	payloadConstructor, exists := r.constructors[jobType]
	return payloadConstructor, exists
}
