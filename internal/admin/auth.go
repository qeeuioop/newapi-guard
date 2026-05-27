package admin

import (
	"net/http"
	"sync"
	"time"

	"newapiguard/internal/webutil"
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
	s := &SessionStore{
		sessions: map[string]Session{},
		ttl:      ttl,
	}
	go s.cleanupLoop()
	return s
}

func (s *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for token, sess := range s.sessions {
			if now.After(sess.ExpiresAt) {
				delete(s.sessions, token)
			}
		}
		s.mu.Unlock()
	}
}

func (s *SessionStore) Create() string {
	token := webutil.RandomToken(32)
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

func (s *SessionStore) TTL() time.Duration {
	return s.ttl
}
