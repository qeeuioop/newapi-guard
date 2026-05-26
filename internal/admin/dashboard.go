package admin

import (
	"net/http"
	"time"

	"newapiguard/internal/webutil"
)

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	today := time.Now().Format("2006-01-02")
	var totalUsers, whitelistCount int
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_whitelist=1`).Scan(&whitelistCount)

	type Stats struct {
		TotalRequests int `json:"total_requests"`
		BlockedUA     int `json:"blocked_ua"`
		BlockedRPM    int `json:"blocked_rpm"`
		Checkins      int `json:"checkins"`
		NewUsers      int `json:"new_users"`
		NewBans       int `json:"new_bans"`
	}
	var stats Stats
	_ = h.db.QueryRow(`SELECT total_requests, blocked_ua, blocked_rpm, checkins, new_users, new_bans FROM daily_stats WHERE date=?`, today).
		Scan(&stats.TotalRequests, &stats.BlockedUA, &stats.BlockedRPM, &stats.Checkins, &stats.NewUsers, &stats.NewBans)

	adminToken := h.settings.GetString("newapi_admin_token")
	activeBans := 0
	if adminToken != "" {
		if users, total, err := h.newapi.ListUsers(r.Context(), adminToken, 1, 1); err == nil {
			_ = users
			totalUsers = total
		}
	}
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM bans WHERE unbanned_at IS NULL AND (expire_at IS NULL OR expire_at > CURRENT_TIMESTAMP)`).Scan(&activeBans)

	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"today":           stats,
		"total_users":     totalUsers,
		"active_bans":     activeBans,
		"whitelist_count": whitelistCount,
	})
}
