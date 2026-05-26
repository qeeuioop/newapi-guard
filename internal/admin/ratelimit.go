package admin

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
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
	if ip := normalizedIP(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if ip := normalizedIP(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if ip := normalizedIP(parts[0]); ip != "" {
			return ip
		}
	}
	if ip := normalizedIP(r.RemoteAddr); ip != "" {
		return ip
	}
	return "unknown"
}

func normalizedIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(value, "[]")
}
