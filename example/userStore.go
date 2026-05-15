package main

import "sync"

// userStore lưu map socketID -> username (thread-safe)
type userStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func (s *userStore) set(id, name string) {
	s.mu.Lock()
	s.data[id] = name
	s.mu.Unlock()
}

func (s *userStore) get(id string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[id]
}

func (s *userStore) delete(id string) {
	s.mu.Lock()
	delete(s.data, id)
	s.mu.Unlock()
}

func (s *userStore) list() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.data))
	for _, name := range s.data {
		names = append(names, name)
	}
	return names
}

func (s *userStore) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}
