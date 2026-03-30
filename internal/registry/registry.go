package registry

import (
	"github.com/ri5hii/Machina/internal/jobs"
)

type PayloadConstructor func(payload map[string]any) (jobs.JobRunType, error)

type Registry struct {
	constructors map[string]PayloadConstructor
}

func New() *Registry {
	return &Registry{
		constructors: make(map[string]PayloadConstructor),
	}
}

func (reg *Registry) Register(jobType string, payloadConstructor PayloadConstructor) {
	_, exists := reg.constructors[jobType]
	if exists {
		panic("Registry: Job type already registered: " + jobType)
	}
	reg.constructors[jobType] = payloadConstructor
}

func (reg *Registry) GetPayloadConstructor(jobType string) (PayloadConstructor, bool) {
	payloadConstructor, exists := reg.constructors[jobType]
	return payloadConstructor, exists
}
