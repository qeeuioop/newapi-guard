package admin

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"newapiguard/internal/newapi"
	"newapiguard/internal/webutil"
)

func (h *Handler) handleBans(w http.ResponseWriter, r *http.Request) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = "active"
	}
	if status == "active" {
		h.handleActiveBans(w, r)
		return
	}

	rows, err := h.db.Query(`SELECT id, newapi_user_id, discord_id, reason, violation_ua, client_ip, duration, expire_at, unbanned_at, created_at
		FROM bans ORDER BY created_at DESC`)
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var items []map[string]any
	for rows.Next() {
		var (
			id          int64
			userID      int64
			discordID   sql.NullString
			reason      string
			violationUA sql.NullString
			clientIP    sql.NullString
			duration    string
			expireAt    sql.NullString
			unbannedAt  sql.NullString
			createdAt   string
		)
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
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "items": items})
}

func (h *Handler) handleActiveBans(w http.ResponseWriter, r *http.Request) {
	adminToken := h.settings.GetString("newapi_admin_token")
	if adminToken == "" {
		webutil.WriteError(w, http.StatusServiceUnavailable, "未配置 New API 管理员令牌")
		return
	}

	remoteUsers, err := h.fetchAllNewAPIUsers(r.Context(), adminToken)
	if err != nil {
		webutil.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}

	contexts, err := h.loadBanContexts()
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := []map[string]any{}
	for _, user := range remoteUsers {
		if user.Status != 2 {
			continue
		}
		_ = h.ensureLocalUserExists(user.ID)
		contextItem, ok := contexts[user.ID]
		item := map[string]any{
			"newapi_user_id": user.ID,
			"username":       user.Username,
			"display_name":   user.DisplayName,
			"status":         user.Status,
			"quota":          user.Quota,
			"group":          user.Group,
			"email":          user.Email,
		}
		if ok {
			for key, value := range contextItem {
				item[key] = value
			}
		} else {
			item["reason"] = "无上下文（可能直接在 New API 后台封禁）"
			item["context_missing"] = true
		}
		items = append(items, item)
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "items": items})
}

func (h *Handler) handleCreateBan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserRef   string `json:"user_ref"`
		UserID    int64  `json:"newapi_user_id"`
		DiscordID string `json:"discord_id"`
		Reason    string `json:"reason"`
		Duration  string `json:"duration"`
	}
	if err := webutil.ReadJSON(r, &req); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	userID, err := h.resolveUserID(req.UserRef, req.UserID, req.DiscordID)
	if err != nil || userID <= 0 || req.Reason == "" {
		if err != nil {
			webutil.WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			webutil.WriteError(w, http.StatusBadRequest, "参数不完整")
		}
		return
	}
	if req.Duration == "" {
		req.Duration = "permanent"
	}
	if err := h.createBan(r, userID, req.Reason, req.Duration); err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) handleUnban(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/guard/api/bans/"), "/unban")
	banID, err := strconv.ParseInt(strings.TrimSuffix(path, "/"), 10, 64)
	if err != nil || banID <= 0 {
		webutil.WriteError(w, http.StatusBadRequest, "封禁 ID 无效")
		return
	}
	if err := h.unbanByID(r, banID); err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) handleUnbanByUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserRef   string `json:"user_ref"`
		UserID    int64  `json:"newapi_user_id"`
		DiscordID string `json:"discord_id"`
	}
	if err := webutil.ReadJSON(r, &req); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	userID, err := h.resolveUserID(req.UserRef, req.UserID, req.DiscordID)
	if err != nil || userID <= 0 {
		if err != nil {
			webutil.WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			webutil.WriteError(w, http.StatusBadRequest, "用户 ID 无效")
		}
		return
	}
	if err := h.unbanByUserID(r, userID, nil); err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) resolveUserID(userRef string, newapiUserID int64, discordID string) (int64, error) {
	if newapiUserID > 0 {
		return newapiUserID, nil
	}

	adminToken := h.settings.GetString("newapi_admin_token")
	discordID = strings.TrimSpace(discordID)
	if discordID != "" {
		var resolved int64
		if err := h.db.QueryRow(`SELECT newapi_user_id FROM users WHERE discord_id=?`, discordID).Scan(&resolved); err == nil {
			return resolved, nil
		}
		if adminToken != "" {
			keyword := "dc_" + discordID
			if users, _, err := h.newapi.SearchUsers(context.Background(), adminToken, keyword, 1, 20); err == nil {
				for _, user := range users {
					if user.Username == keyword {
						return user.ID, nil
					}
				}
			}
		}
		if parsed, err := strconv.ParseInt(discordID, 10, 64); err == nil && parsed > 0 {
			return parsed, nil
		}
		return 0, fmt.Errorf("未找到对应的 Discord 用户")
	}

	userRef = strings.TrimSpace(userRef)
	if userRef == "" {
		return 0, fmt.Errorf("缺少用户标识")
	}

	var resolved int64
	if err := h.db.QueryRow(`SELECT newapi_user_id FROM users WHERE discord_id=?`, userRef).Scan(&resolved); err == nil {
		return resolved, nil
	}
	if adminToken != "" {
		if users, _, err := h.newapi.SearchUsers(context.Background(), adminToken, userRef, 1, 20); err == nil {
			for _, user := range users {
				if user.Username == userRef || strconv.FormatInt(user.ID, 10) == userRef {
					return user.ID, nil
				}
			}
		}
	}

	parsed, err := strconv.ParseInt(userRef, 10, 64)
	if err == nil && parsed > 0 {
		return parsed, nil
	}
	return 0, fmt.Errorf("无法解析用户标识")
}

func (h *Handler) createBan(r *http.Request, userID int64, reason, duration string) error {
	if err := h.ensureLocalUserExists(userID); err != nil {
		return err
	}
	adminToken := h.settings.GetString("newapi_admin_token")
	if adminToken != "" {
		if err := h.newapi.UpdateUserStatus(r.Context(), adminToken, userID, 2); err != nil {
			return err
		}
	}

	var discordID sql.NullString
	_ = h.db.QueryRow(`SELECT discord_id FROM users WHERE newapi_user_id=?`, userID).Scan(&discordID)

	var expireAt any
	switch duration {
	case "7d":
		expireAt = time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	case "30d":
		expireAt = time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	default:
		duration = "permanent"
		expireAt = nil
	}

	if _, err := h.db.Exec(`INSERT INTO bans(newapi_user_id, discord_id, reason, duration, expire_at, created_at)
		VALUES(?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, userID, discordID.String, reason, duration, expireAt); err != nil {
		return err
	}

	today := time.Now().Format("2006-01-02")
	_, _ = h.db.Exec(`INSERT INTO daily_stats(date) VALUES(?) ON CONFLICT(date) DO NOTHING`, today)
	_, _ = h.db.Exec(`UPDATE daily_stats SET new_bans = new_bans + 1 WHERE date=?`, today)
	return nil
}

func (h *Handler) unbanByID(r *http.Request, banID int64) error {
	var userID int64
	if err := h.db.QueryRow(`SELECT newapi_user_id FROM bans WHERE id=?`, banID).Scan(&userID); err != nil {
		return err
	}
	return h.unbanByUserID(r, userID, &banID)
}

func (h *Handler) unbanByUserID(r *http.Request, userID int64, onlyBanID *int64) error {
	adminToken := h.settings.GetString("newapi_admin_token")
	if adminToken != "" {
		if err := h.newapi.UpdateUserStatus(r.Context(), adminToken, userID, 1); err != nil {
			return err
		}
	}
	if onlyBanID != nil {
		if _, err := h.db.Exec(`UPDATE bans SET unbanned_at=CURRENT_TIMESTAMP WHERE id=?`, *onlyBanID); err != nil {
			return err
		}
	} else {
		if _, err := h.db.Exec(`UPDATE bans SET unbanned_at=CURRENT_TIMESTAMP WHERE newapi_user_id=? AND unbanned_at IS NULL`, userID); err != nil {
			return err
		}
	}
	_, _ = h.db.Exec(`DELETE FROM ua_strikes WHERE newapi_user_id=?`, userID)
	return nil
}

func (h *Handler) fetchAllNewAPIUsers(ctx context.Context, adminToken string) ([]newapi.User, error) {
	page := 1
	pageSize := 100
	all := []newapi.User{}
	for {
		items, total, err := h.newapi.ListUsers(ctx, adminToken, page, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) == 0 || len(all) >= total {
			break
		}
		page++
	}
	return all, nil
}

func (h *Handler) loadBanContexts() (map[int64]map[string]any, error) {
	rows, err := h.db.Query(`SELECT b.id, b.newapi_user_id, b.discord_id, b.reason, b.violation_ua, b.client_ip, b.duration, b.expire_at, b.unbanned_at, b.created_at,
		u.discord_name
		FROM bans b
		LEFT JOIN users u ON u.newapi_user_id = b.newapi_user_id
		WHERE b.unbanned_at IS NULL
		ORDER BY b.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := map[int64]map[string]any{}
	for rows.Next() {
		var (
			id          int64
			userID      int64
			discordID   sql.NullString
			reason      string
			violationUA sql.NullString
			clientIP    sql.NullString
			duration    string
			expireAt    sql.NullString
			unbannedAt  sql.NullString
			createdAt   string
			discordName sql.NullString
		)
		if err := rows.Scan(&id, &userID, &discordID, &reason, &violationUA, &clientIP, &duration, &expireAt, &unbannedAt, &createdAt, &discordName); err != nil {
			return nil, err
		}
		if _, ok := items[userID]; ok {
			continue
		}
		items[userID] = map[string]any{
			"id":              id,
			"discord_id":      discordID.String,
			"discord_name":    discordName.String,
			"reason":          reason,
			"violation_ua":    violationUA.String,
			"client_ip":       clientIP.String,
			"duration":        duration,
			"expire_at":       expireAt.String,
			"unbanned_at":     unbannedAt.String,
			"created_at":      createdAt,
			"context_missing": false,
		}
	}
	return items, rows.Err()
}
