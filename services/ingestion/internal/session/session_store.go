package session

import (
	"sync"

	"github.com/aman-churiwal/gridflow-shared/proto/gen"
)

type Session struct {
	VehicleID string
	Stream    gen.IngestionService_StreamTelemetryServer
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewSessionStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
	}
}

func (s *Store) Add(vehicleID string, session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[vehicleID] = session
}

func (s *Store) Remove(vehicleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, vehicleID)
}

func (s *Store) Get(vehicleID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[vehicleID]

	return session, ok
}
