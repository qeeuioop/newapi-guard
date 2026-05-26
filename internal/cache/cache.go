package cache

import (
	"sync"
	"time"
)

type TokenEntry struct {
	UserID    int64
	ExpiresAt time.Time
}

type RPMEntry struct {
	Count     int
	ExpiresAt time.Time
}

type whitelistEntry struct {
	ok        bool
	expiresAt time.Time
}

const whitelistTTL = 5 * time.Minute

type Store struct {
	tokens sync.Map
	users  sync.Map
	rpm    sync.Map
	rpmMu  sync.Mutex
}

func New() *Store {
	return &Store{}
}

func (s *Store) SetToken(token string, userID int64, ttl time.Duration) {
	s.tokens.Store(token, TokenEntry{UserID: userID, ExpiresAt: time.Now().Add(ttl)})
}

func (s *Store) GetToken(token string) (int64, bool) {
	value, ok := s.tokens.Load(token)
	if !ok {
		return 0, false
	}
	entry := value.(TokenEntry)
	if time.Now().After(entry.ExpiresAt) {
		s.tokens.Delete(token)
		return 0, false
	}
	return entry.UserID, true
}

func (s *Store) SetWhitelist(userID int64, ok bool) {
	s.users.Store(userID, whitelistEntry{ok: ok, expiresAt: time.Now().Add(whitelistTTL)})
}

func (s *Store) IsWhitelist(userID int64) bool {
	value, ok := s.users.Load(userID)
	if !ok {
		return false
	}
	entry, ok := value.(whitelistEntry)
	if !ok {
		s.users.Delete(userID)
		return false
	}
	if time.Now().After(entry.expiresAt) {
		s.users.Delete(userID)
		return false
	}
	return entry.ok
}

func (s *Store) DeleteWhitelist(userID int64) {
	s.users.Delete(userID)
}

func (s *Store) IncrementRPM(key string, ttl time.Duration) int {
	s.rpmMu.Lock()
	defer s.rpmMu.Unlock()

	now := time.Now()
	value, ok := s.rpm.Load(key)
	var entry RPMEntry
	if ok {
		entry = value.(RPMEntry)
	}
	if !ok || now.After(entry.ExpiresAt) {
		entry = RPMEntry{Count: 0, ExpiresAt: now.Add(ttl)}
	}
	entry.Count++
	s.rpm.Store(key, entry)
	return entry.Count
}

func (s *Store) Cleanup() {
	now := time.Now()
	s.tokens.Range(func(key, value any) bool {
		if entry, ok := value.(TokenEntry); ok && now.After(entry.ExpiresAt) {
			s.tokens.Delete(key)
		}
		return true
	})
	s.rpm.Range(func(key, value any) bool {
		if entry, ok := value.(RPMEntry); ok && now.After(entry.ExpiresAt) {
			s.rpm.Delete(key)
		}
		return true
	})
	s.users.Range(func(key, value any) bool {
		if entry, ok := value.(whitelistEntry); ok && now.After(entry.expiresAt) {
			s.users.Delete(key)
		}
		return true
	})
}
