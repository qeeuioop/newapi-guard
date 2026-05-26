package admin

import (
	"database/sql"
	"net/http"
	"strings"

	"newapiguard/internal/webutil"
)

func (h *Handler) handleBanLogs(w http.ResponseWriter, r *http.Request) {
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	size := parseIntDefault(r.URL.Query().Get("size"), 50)
	offset := (page - 1) * size
	rows, err := h.db.Query(`SELECT id, newapi_user_id, discord_id, reason, violation_ua, client_ip, duration, expire_at, unbanned_at, created_at
		FROM bans ORDER BY created_at DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var id, userID int64
		var discordID sql.NullString
		var reason, duration, createdAt string
		var violationUA, clientIP, expireAt, unbannedAt sql.NullString
		if err := rows.Scan(&id, &userID, &discordID, &reason, &violationUA, &clientIP, &duration, &expireAt, &unbannedAt, &createdAt); err != nil {
			webutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, map[string]any{
			"id":             id,
			"newapi_user_id": userID,
			"discord_id":     discordID.String,
			"reason":         reason,
			"violation_ua":   violationUA.String,
			"client_ip":      clientIP.String,
			"duration":       duration,
			"expire_at":      expireAt.String,
			"unbanned_at":    unbannedAt.String,
			"created_at":     createdAt,
		})
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "page": page, "size": size, "items": items})
}

func (h *Handler) handleCheckinLogs(w http.ResponseWriter, r *http.Request) {
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	size := parseIntDefault(r.URL.Query().Get("size"), 50)
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	offset := (page - 1) * size

	query := `SELECT id, newapi_user_id, quota_added, checked_at FROM checkin_records`
	args := []any{}
	if userID != "" {
		query += ` WHERE newapi_user_id=?`
		args = append(args, userID)
	}
	query += ` ORDER BY checked_at DESC LIMIT ? OFFSET ?`
	args = append(args, size, offset)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var id, uid, quota int64
		var checkedAt string
		if err := rows.Scan(&id, &uid, &quota, &checkedAt); err != nil {
			webutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, map[string]any{
			"id":             id,
			"newapi_user_id": uid,
			"quota_added":    quota,
			"checked_at":     checkedAt,
		})
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "page": page, "size": size, "items": items})
}

func (h *Handler) handleStatsLogs(w http.ResponseWriter, r *http.Request) {
	days := parseIntDefault(r.URL.Query().Get("days"), 30)
	rows, err := h.db.Query(`SELECT date, total_requests, blocked_ua, blocked_rpm, checkins, new_users, new_bans
		FROM daily_stats ORDER BY date DESC LIMIT ?`, days)
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var date string
		var total, ua, rpm, checkins, newUsers, newBans int
		if err := rows.Scan(&date, &total, &ua, &rpm, &checkins, &newUsers, &newBans); err != nil {
			webutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, map[string]any{
			"date":           date,
			"total_requests": total,
			"blocked_ua":     ua,
			"blocked_rpm":    rpm,
			"checkins":       checkins,
			"new_users":      newUsers,
			"new_bans":       newBans,
		})
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "items": items})
}
