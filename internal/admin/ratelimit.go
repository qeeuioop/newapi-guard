package admin

import (
	"net/http"
	"sync"
	"time"

	"newapiguard/internal/webutil"
)

type loginAttempt struct {
	failures    int
	lockedUntil time.Time
}

type LoginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
	maxFails int
	lockout  time.Duration
}

func NewLoginLimiter(maxFails int, lockout time.Duration) *LoginLimiter {
	limiter := &LoginLimiter{
		attempts: map[string]*loginAttempt{},
		maxFails: maxFails,
		lockout:  lockout,
	}
	go limiter.cleanupLoop()
	return limiter
}

func (l *LoginLimiter) IsLocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, ok := l.attempts[ip]
	if !ok {
		return false
	}
	if time.Now().After(attempt.lockedUntil) {
		delete(l.attempts, ip)
		return false
	}
	return attempt.failures >= l.maxFails
}

func (l *LoginLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	attempt, ok := l.attempts[ip]
	if !ok || now.After(attempt.lockedUntil) {
		attempt = &loginAttempt{}
		l.attempts[ip] = attempt
	}
	attempt.failures++
	attempt.lockedUntil = now.Add(l.lockout)
}

func (l *LoginLimiter) ClearFailures(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.attempts, ip)
}

func (l *LoginLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.cleanup()
	}
}

func (l *LoginLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for ip, attempt := range l.attempts {
		if now.After(attempt.lockedUntil) {
			delete(l.attempts, ip)
		}
	}
}

func clientIP(r *http.Request) string {
	return webutil.ClientIP(r)
}
