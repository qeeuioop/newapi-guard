package webutil

import (
	"net"
	"net/http"
	"strings"
)

func ClientIP(r *http.Request) string {
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
