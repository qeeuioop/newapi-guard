package admin

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

type Session struct {
	Token     string
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{
		sessions: map[string]Session{},
		ttl:      ttl,
	}
}

func (s *SessionStore) Create() string {
	token := randomToken(32)
	s.mu.Lock()
	s.sessions[token] = Session{
		Token:     token,
		ExpiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return token
}

func (s *SessionStore) Validate(token string) bool {
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(session.ExpiresAt) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return false
	}
	return true
}

func (s *SessionStore) Delete(token string) {
	if token == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (s *SessionStore) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		} else {
			token = ""
		}
		if !s.Validate(token) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"message":"未授权"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func randomToken(length int) string {
	raw := make([]byte, length)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}
