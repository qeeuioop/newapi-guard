package admin

import (
	"database/sql"
	"net/http"
	"sync"
	"time"

	"newapiguard/internal/webutil"
)

type PersistentSessionStore struct {
	mu       sync.RWMutex
	db       *sql.DB
	sessions map[string]Session
	ttl      time.Duration
}

func NewPersistentSessionStore(db *sql.DB, ttl time.Duration) *PersistentSessionStore {
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS admin_sessions (
		token TEXT PRIMARY KEY,
		expires_at DATETIME NOT NULL
	)`)
	_, _ = db.Exec(`DELETE FROM admin_sessions WHERE expires_at <= CURRENT_TIMESTAMP`)

	store := &PersistentSessionStore{
		db:       db,
		sessions: map[string]Session{},
		ttl:      ttl,
	}
	store.loadFromDB()
	return store
}

func (s *PersistentSessionStore) loadFromDB() {
	rows, err := s.db.Query(`SELECT token, expires_at FROM admin_sessions WHERE expires_at > CURRENT_TIMESTAMP`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var token, expiresAt string
		if err := rows.Scan(&token, &expiresAt); err != nil {
			continue
		}
		t, err := time.Parse("2006-01-02 15:04:05", expiresAt)
		if err != nil {
			t, err = time.Parse(time.RFC3339, expiresAt)
			if err != nil {
				continue
			}
		}
		s.sessions[token] = Session{Token: token, ExpiresAt: t}
	}
}

func (s *PersistentSessionStore) Create() string {
	token := webutil.RandomToken(32)
	expiresAt := time.Now().Add(s.ttl)

	s.mu.Lock()
	s.sessions[token] = Session{Token: token, ExpiresAt: expiresAt}
	s.mu.Unlock()

	_, _ = s.db.Exec(`INSERT INTO admin_sessions(token, expires_at) VALUES(?, ?)`, token, expiresAt.Format(time.RFC3339))
	return token
}

func (s *PersistentSessionStore) Validate(token string) bool {
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(session.ExpiresAt) {
		s.Delete(token)
		return false
	}
	return true
}

func (s *PersistentSessionStore) Delete(token string) {
	if token == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	_, _ = s.db.Exec(`DELETE FROM admin_sessions WHERE token=?`, token)
}

func (s *PersistentSessionStore) Middleware(next http.Handler) http.Handler {
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

func (s *PersistentSessionStore) TTL() time.Duration {
	return s.ttl
}
