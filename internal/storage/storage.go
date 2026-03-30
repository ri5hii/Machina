package storage

import (
	"sync"
	"time"

	"github.com/ri5hii/Machina/internal/jobs"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type JobRecord struct {
	RecordID  string
	Job       jobs.JobRunType
	JobStatus string
	Result    any
	Err       error
	CreatedAt time.Time
	UpdatedAt time.Time
}

type JobStore struct {
	mutex     sync.RWMutex
	jobRecord map[string]*JobRecord
}

// NewStore creates an in-memory job store for tracking lifecycle state and results.
func NewStore() *JobStore {
	return &JobStore{
		jobRecord: make(map[string]*JobRecord),
	}
}

// Add records a newly accepted job in pending state.
func (s *JobStore) Add(id string, job jobs.JobRunType) *JobRecord {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	jobrecord := &JobRecord{
		RecordID:  id,
		Job:       job,
		JobStatus: StatusPending,
		Result:    nil,
		Err:       nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.jobRecord[id] = jobrecord
	return jobrecord
}

// Get fetches one stored job record by id.
func (s *JobStore) Get(id string) (*JobRecord, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	record, ok := s.jobRecord[id]
	return record, ok
}

// List returns all stored job records in insertion-independent order.
func (s *JobStore) List() []*JobRecord {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	records := make([]*JobRecord, 0, len(s.jobRecord))
	for _, r := range s.jobRecord {
		records = append(records, r)
	}
	return records
}

// SetStatus updates the lifecycle status for one stored job.
func (s *JobStore) SetStatus(id string, status string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.jobRecord[id]
	if ok {
		record.JobStatus = status
		record.UpdatedAt = time.Now()
	}
}

// SetError records the terminal error for one stored job.
func (s *JobStore) SetError(id string, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.jobRecord[id]
	if ok {
		record.Err = err
		record.UpdatedAt = time.Now()
	}
}

// SetResult records the final result for one stored job.
func (s *JobStore) SetResult(id string, result any) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.jobRecord[id]
	if ok {
		record.Result = result
		record.UpdatedAt = time.Now()
	}
}
