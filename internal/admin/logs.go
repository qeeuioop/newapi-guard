package admin

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"newapiguard/internal/newapi"
	"newapiguard/internal/webutil"
)

func (h *Handler) handleBanLogs(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r.URL.Query().Get("page"))
	size := parseSize(r.URL.Query().Get("size"), 50)
	offset := (page - 1) * size
	rows, err := h.db.Query(`SELECT b.id, b.newapi_user_id, b.discord_id, b.reason, b.violation_ua, b.client_ip, b.duration, b.expire_at, b.unbanned_at, b.created_at,
		u.username, u.display_name, u.discord_name
		FROM bans b
		LEFT JOIN users u ON u.newapi_user_id = b.newapi_user_id
		ORDER BY b.created_at DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		log.Printf("[admin] 查询封禁日志失败: %v", err)
		webutil.WriteError(w, http.StatusInternalServerError, "查询封禁日志失败")
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var id, userID int64
		var discordID sql.NullString
		var reason, duration, createdAt string
		var violationUA, clientIP, expireAt, unbannedAt, username, displayName, discordName sql.NullString
		if err := rows.Scan(&id, &userID, &discordID, &reason, &violationUA, &clientIP, &duration, &expireAt, &unbannedAt, &createdAt, &username, &displayName, &discordName); err != nil {
			log.Printf("[admin] 扫描封禁日志行失败: %v", err)
			webutil.WriteError(w, http.StatusInternalServerError, "读取封禁日志失败")
			return
		}
		items = append(items, map[string]any{
			"id":             id,
			"newapi_user_id": userID,
			"username":       username.String,
			"display_name":   displayName.String,
			"discord_id":     discordID.String,
			"discord_name":   discordName.String,
			"reason":         reason,
			"violation_ua":   violationUA.String,
			"client_ip":      clientIP.String,
			"duration":       duration,
			"expire_at":      expireAt.String,
			"unbanned_at":    unbannedAt.String,
			"created_at":     createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[admin] 遍历封禁日志失败: %v", err)
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "page": page, "size": size, "items": items})
}

func (h *Handler) handleCheckinLogs(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r.URL.Query().Get("page"))
	size := parseSize(r.URL.Query().Get("size"), 50)
	userFilter := strings.TrimSpace(r.URL.Query().Get("user_id"))
	offset := (page - 1) * size

	var filterUserID int64
	if userFilter != "" {
		_ = h.db.QueryRow(`SELECT newapi_user_id FROM users WHERE CAST(newapi_user_id AS TEXT)=? OR username=? OR display_name=? OR discord_id=? OR discord_name=? LIMIT 1`, userFilter, userFilter, userFilter, userFilter, userFilter).Scan(&filterUserID)
	}
	if userFilter != "" && filterUserID == 0 {
		webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "page": page, "size": size, "items": []any{}})
		return
	}

	records, err := h.tokens.ListCheckins(r.Context(), filterUserID, size, offset)
	if err != nil {
		log.Printf("[admin] 查询签到记录失败: %v", err)
		webutil.WriteError(w, http.StatusBadGateway, "查询签到记录失败")
		return
	}
	identityMap := h.batchLocalIdentities(records)
	items := []map[string]any{}
	for _, record := range records {
		identity := identityMap[record.UserID]
		items = append(items, map[string]any{
			"id":             record.ID,
			"newapi_user_id": record.UserID,
			"username":       identity["username"],
			"display_name":   identity["display_name"],
			"discord_id":     identity["discord_id"],
			"discord_name":   identity["discord_name"],
			"quota_added":    record.QuotaAwarded,
			"checked_at":     record.CheckinDate,
			"created_at":     record.CreatedAt,
		})
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "page": page, "size": size, "items": items})
}

func (h *Handler) batchLocalIdentities(records []newapi.CheckinRecord) map[int64]map[string]string {
	result := map[int64]map[string]string{}
	if len(records) == 0 {
		return result
	}
	ids := make([]any, 0, len(records))
	placeholders := make([]string, 0, len(records))
	for _, r := range records {
		if _, ok := result[r.UserID]; ok {
			continue
		}
		result[r.UserID] = map[string]string{"username": "", "display_name": "", "discord_id": "", "discord_name": ""}
		ids = append(ids, r.UserID)
		placeholders = append(placeholders, "?")
	}
	if len(ids) == 0 {
		return result
	}
	rows, err := h.db.Query(`SELECT newapi_user_id, username, display_name, discord_id, discord_name FROM users WHERE newapi_user_id IN (`+strings.Join(placeholders, ",")+`)`, ids...)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var uid int64
		var username, displayName, discordID, discordName sql.NullString
		if err := rows.Scan(&uid, &username, &displayName, &discordID, &discordName); err != nil {
			continue
		}
		result[uid] = map[string]string{
			"username":     username.String,
			"display_name": displayName.String,
			"discord_id":   discordID.String,
			"discord_name": discordName.String,
		}
	}
	return result
}

func (h *Handler) localIdentity(userID int64) map[string]string {
	identity := map[string]string{"username": "", "display_name": "", "discord_id": "", "discord_name": ""}
	var username, displayName, discordID, discordName sql.NullString
	_ = h.db.QueryRow(`SELECT username, display_name, discord_id, discord_name FROM users WHERE newapi_user_id=?`, userID).Scan(&username, &displayName, &discordID, &discordName)
	identity["username"] = username.String
	identity["display_name"] = displayName.String
	identity["discord_id"] = discordID.String
	identity["discord_name"] = discordName.String
	return identity
}

func (h *Handler) handleStatsLogs(w http.ResponseWriter, r *http.Request) {
	days := min(max(parseIntDefault(r.URL.Query().Get("days"), 30), 1), 365)
	rows, err := h.db.Query(`SELECT date, total_requests, blocked_ua, blocked_rpm, checkins, new_users, new_bans
		FROM daily_stats ORDER BY date DESC LIMIT ?`, days)
	if err != nil {
		log.Printf("[admin] 查询统计日志失败: %v", err)
		webutil.WriteError(w, http.StatusInternalServerError, "查询统计数据失败")
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var date string
		var total, ua, rpm, checkins, newUsers, newBans int
		if err := rows.Scan(&date, &total, &ua, &rpm, &checkins, &newUsers, &newBans); err != nil {
			log.Printf("[admin] 扫描统计日志行失败: %v", err)
			webutil.WriteError(w, http.StatusInternalServerError, "读取统计数据失败")
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
	if err := rows.Err(); err != nil {
		log.Printf("[admin] 遍历统计日志失败: %v", err)
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "items": items})
}
