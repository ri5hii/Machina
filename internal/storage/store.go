package storage

import (
	"time"
	"sync"
	"github.com/ri5hii/Machina/internal/jobs"
)

const (
    StatusPending   = "pending"
    StatusRunning   = "running"
    StatusCompleted = "completed"
    StatusFailed    = "failed"
)

type JobRecord struct {
	ID string
	Status string
	Job jobs.Job
	Result any
	Err error
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Store struct {
	mu sync.RWMutex
	records map[string]*JobRecord
}

func New() *Store {
	return &Store {
		records : make(map[string]*JobRecord),
	}
}

func(s *Store) Add(id string, job jobs.Job) *JobRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	record := &JobRecord{
			ID:        id,
			Status:    StatusPending,
			Job:       job,
			Result:    nil,
			Err:       nil,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	s.records[id] = record
	return record
}

func(s *Store) Get(id string) (*JobRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	record, ok := s.records[id]
	return record, ok
}

func(s *Store) SetStatus(id string, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	record, ok := s.records[id]
	if !ok {
		return
	}
	record.Status = status
	record.UpdatedAt = time.Now()
}

func(s *Store) SetResult(id string, result any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	record, ok := s.records[id]
	if ok {
		record.Result = result
		record.UpdatedAt = time.Now()
	}
}

func(s *Store) SetError(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	record, ok := s.records[id]
	if ok {
		record.Err = err
		record.UpdatedAt = time.Now()
	}
}