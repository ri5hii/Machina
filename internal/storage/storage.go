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

func NewStore() *JobStore {
	return &JobStore{
		jobRecord: make(map[string]*JobRecord),
	}
}

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

func (s *JobStore) Get(id string) (*JobRecord, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	record, ok := s.jobRecord[id]
	return record, ok
}

func (s *JobStore) List() []*JobRecord {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	records := make([]*JobRecord, 0, len(s.jobRecord))
	for _, r := range s.jobRecord {
		records = append(records, r)
	}
	return records
}

func (s *JobStore) SetStatus(id string, status string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.jobRecord[id]
	if ok {
		record.JobStatus = status
		record.UpdatedAt = time.Now()
	}
}

func (s *JobStore) SetError(id string, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.jobRecord[id]
	if ok {
		record.Err = err
		record.UpdatedAt = time.Now()
	}
}

func (s *JobStore) SetResult(id string, result any) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.jobRecord[id]
	if ok {
		record.Result = result
		record.UpdatedAt = time.Now()
	}
}
