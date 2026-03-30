package registry

import (
	"sort"

	"github.com/ri5hii/Machina/internal/jobs"
)

type PayloadConstructor func(payload map[string]any) (jobs.JobRunType, error)

type Registry struct {
	constructors map[string]PayloadConstructor
}

// New creates an empty registry for payload constructor lookups.
func New() *Registry {
	return &Registry{
		constructors: make(map[string]PayloadConstructor),
	}
}

// Register adds a payload constructor for a unique runtime job type.
func (reg *Registry) Register(jobType string, payloadConstructor PayloadConstructor) {
	_, exists := reg.constructors[jobType]
	if exists {
		panic("Registry: Job type already registered: " + jobType)
	}
	reg.constructors[jobType] = payloadConstructor
}

// GetPayloadConstructor looks up the constructor for a submitted runtime job type.
func (reg *Registry) GetPayloadConstructor(jobType string) (PayloadConstructor, bool) {
	payloadConstructor, exists := reg.constructors[jobType]
	return payloadConstructor, exists
}

// JobTypes returns the registered runtime job types in sorted order.
func (reg *Registry) JobTypes() []string {
	jobTypes := make([]string, 0, len(reg.constructors))
	for jobType := range reg.constructors {
		jobTypes = append(jobTypes, jobType)
	}
	sort.Strings(jobTypes)
	return jobTypes
}
