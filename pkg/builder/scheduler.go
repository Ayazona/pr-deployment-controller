package builder

import (
	"sync"
)

// scheduler is responsible for blocking jobs if a newer job already is running
type scheduler struct {
	nextJobID int64
	lock      *sync.Mutex

	jobs map[string]int64
}

func newScheduler() *scheduler {
	return &scheduler{
		nextJobID: 1,
		lock:      &sync.Mutex{},

		jobs: map[string]int64{},
	}
}

// getNextJobID increases the nex jobID value and returns the value
func (s *scheduler) getNextJobID() int64 {
	s.lock.Lock()
	defer s.lock.Unlock()

	nextJobID := s.nextJobID
	s.nextJobID = nextJobID + 1

	return nextJobID
}

// scheduleJob returns an error if another job with an higher ID exists
func (s *scheduler) scheduleJob(name string, id int64) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	lastID, ok := s.jobs[name]
	if !ok {
		// No previous job id found, set job id and return

		s.jobs[name] = id
		return nil
	}

	if lastID > id {
		return ErrJobOutdated
	}

	// New id higher than the prevoius id, store id and return
	s.jobs[name] = id

	return nil
}
