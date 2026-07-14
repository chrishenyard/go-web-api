package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/chrishenyard/go-web-api/models"
)

// UserStore defines the persistence interface used by the handlers.
// The in-memory implementation below is intentionally simple so the focus
// remains on the authentication architecture rather than storage concerns.
type UserStore interface {
	FindByID(id string) *models.User
	FindByUsername(username string) *models.User
	All() []*models.User
	Save(u *models.User)
	Delete(id string) bool
}

// MemoryUserStore is a thread-safe in-memory implementation of UserStore.
type MemoryUserStore struct {
	mu    sync.RWMutex
	users map[string]*models.User
}

// NewMemoryUserStore creates an empty MemoryUserStore.
func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{users: make(map[string]*models.User)}
}

func (s *MemoryUserStore) FindByID(id string) *models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users[id]
}

func (s *MemoryUserStore) FindByUsername(username string) *models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Username == username {
			return u
		}
	}
	return nil
}

func (s *MemoryUserStore) All() []*models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out
}

func (s *MemoryUserStore) Save(u *models.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.ID] = u
}

func (s *MemoryUserStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return false
	}
	delete(s.users, id)
	return true
}

// newID generates a random hex string suitable for use as a user ID.
func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
