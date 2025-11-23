package state

import "sync"

type StartTracker struct {
	mu      sync.RWMutex
	started map[int64]bool
}

func NewStartTracker() *StartTracker {
	return &StartTracker{
		started: make(map[int64]bool),
	}
}

func (s *StartTracker) MarkStarted(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started[chatID] = true
}

func (s *StartTracker) IsStarted(chatID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started[chatID]
}
