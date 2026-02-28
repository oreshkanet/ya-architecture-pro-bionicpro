package session

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type SessionData struct {
	UserID       string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type Store interface {
	Get(ctx context.Context, sessionID string) (*SessionData, error)
	Set(ctx context.Context, sessionID string, data *SessionData, ttl time.Duration) error
	Delete(ctx context.Context, sessionID string) error
	Rotate(ctx context.Context, oldSessionID string) (string, *SessionData, error)
}

type inMemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionData
	ttls     map[string]time.Time
}

func NewInMemoryStore() Store {
	s := &inMemoryStore{
		sessions: make(map[string]*SessionData),
		ttls:     make(map[string]time.Time),
	}
	go s.cleanupLoop()
	return s
}

func (s *inMemoryStore) Get(_ context.Context, sessionID string) (*SessionData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	sessionExp, hasTTL := s.ttls[sessionID]
	if hasTTL && time.Now().After(sessionExp) {
		log.Printf("[store] session expired (ttl was %s)", sessionExp)
		return nil, nil
	}
	if hasTTL {
		log.Printf("[store] session ok (valid until %s)", sessionExp)
	} else {
		log.Printf("[store] session ok (no ttl set)")
	}
	return data, nil
}

func (s *inMemoryStore) Set(_ context.Context, sessionID string, data *SessionData, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = data
	s.ttls[sessionID] = time.Now().Add(ttl)
	return nil
}

func (s *inMemoryStore) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	delete(s.ttls, sessionID)
	return nil
}

func (s *inMemoryStore) Rotate(_ context.Context, oldSessionID string) (string, *SessionData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.sessions[oldSessionID]
	if !ok {
		return "", nil, nil
	}
	newID := uuid.New().String()
	s.sessions[newID] = data
	if exp, ok := s.ttls[oldSessionID]; ok {
		s.ttls[newID] = exp
	} else {
		s.ttls[newID] = time.Now().Add(30 * time.Minute)
	}
	delete(s.sessions, oldSessionID)
	delete(s.ttls, oldSessionID)
	return newID, data, nil
}

func (s *inMemoryStore) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, exp := range s.ttls {
			if now.After(exp) {
				delete(s.sessions, id)
				delete(s.ttls, id)
			}
		}
		s.mu.Unlock()
	}
}
