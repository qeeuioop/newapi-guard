package admin

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"newapiguard/internal/newapi"
	"newapiguard/internal/webutil"
)

func (h *Handler) handleUsers(w http.ResponseWriter, r *http.Request) {
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	size := parseIntDefault(r.URL.Query().Get("size"), 20)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	adminToken := h.settings.GetString("newapi_admin_token")

	if adminToken == "" {
		localUsers, err := h.queryLocalUsers(page, size, search)
		if err != nil {
			webutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items := make([]map[string]any, 0, len(localUsers))
		for _, localUser := range localUsers {
			items = append(items, h.buildUserItem(nil, localUser))
		}
		webutil.WriteJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"page":    page,
			"size":    size,
			"items":   items,
		})
		return
	}

	localMap, err := h.loadLocalUserMap()
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.syncDiscordOAuthBindings(r.Context()); err != nil {
		webutil.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	localMap, err = h.loadLocalUserMap()
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var (
		remoteUsers []newapi.User
		total       int
	)
	if search != "" {
		remoteUsers, total, err = h.newapi.SearchUsers(r.Context(), adminToken, search, page, size)
	} else {
		remoteUsers, total, err = h.newapi.ListUsers(r.Context(), adminToken, page, size)
	}
	if err != nil {
		webutil.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}

	items := make([]map[string]any, 0, len(remoteUsers))
	seen := map[int64]struct{}{}
	for _, remoteUser := range remoteUsers {
		seen[remoteUser.ID] = struct{}{}
		_ = h.upsertLocalUserIdentity(remoteUser.ID, remoteUser.Username, remoteUser.DisplayName)
		items = append(items, h.buildUserItem(&remoteUser, localMap[remoteUser.ID]))
	}

	if search != "" {
		localMatches, localErr := h.queryLocalUsers(1, maxInt(size*3, 100), search)
		if localErr == nil {
			for _, localUser := range localMatches {
				if _, ok := seen[localUser.UserID]; ok {
					continue
				}
				remoteUser, remoteErr := h.newapi.GetUser(r.Context(), adminToken, localUser.UserID)
				if remoteErr != nil {
					items = append(items, h.buildUserItem(nil, localUser))
					continue
				}
				_ = h.ensureLocalUserExists(remoteUser.ID)
				items = append(items, h.buildUserItem(remoteUser, localUser))
				seen[localUser.UserID] = struct{}{}
			}
		}
	}

	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"page":    page,
		"size":    size,
		"total":   total,
		"items":   items,
	})
}

func (h *Handler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode         string `json:"mode"`
		Username     string `json:"username"`
		Password     string `json:"password"`
		DiscordID    string `json:"discord_id"`
		DiscordName  string `json:"discord_name"`
		InitialQuota int    `json:"initial_quota"`
		IsWhitelist  bool   `json:"is_whitelist"`
	}
	if err := webutil.ReadJSON(r, &req); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "请求体无效")
		return
	}

	adminToken := h.settings.GetString("newapi_admin_token")
	if adminToken == "" {
		webutil.WriteError(w, http.StatusServiceUnavailable, "未配置 New API 管理员令牌")
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	var username, password string
	switch mode {
	case "password":
		username = strings.TrimSpace(req.Username)
		password = req.Password
		if username == "" || password == "" {
			webutil.WriteError(w, http.StatusBadRequest, "用户名或密码不能为空")
			return
		}
	case "discord":
		if strings.TrimSpace(req.DiscordID) == "" {
			webutil.WriteError(w, http.StatusBadRequest, "Discord ID 不能为空")
			return
		}
		username = "dc_" + strings.TrimSpace(req.DiscordID)
		password = webutil.RandomToken(12)
	default:
		webutil.WriteError(w, http.StatusBadRequest, "不支持的创建模式")
		return
	}

	userID, err := h.newapi.CreateUser(r.Context(), adminToken, username, password)
	if err != nil {
		webutil.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}

	_, err = h.db.Exec(`INSERT INTO users(newapi_user_id, discord_id, discord_name, is_whitelist, created_at)
		VALUES(?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(newapi_user_id) DO UPDATE SET
			discord_id=excluded.discord_id,
			discord_name=excluded.discord_name,
			is_whitelist=excluded.is_whitelist`,
		userID, nullable(req.DiscordID), nullable(req.DiscordName), boolToInt(req.IsWhitelist))
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.cache.SetWhitelist(userID, req.IsWhitelist)

	if req.InitialQuota > 0 {
		if err := h.newapi.TopupUser(r.Context(), adminToken, userID, req.InitialQuota); err != nil {
			webutil.WriteError(w, http.StatusBadGateway, err.Error())
			return
		}
	}

	today := time.Now().Format("2006-01-02")
	_, _ = h.db.Exec(`INSERT INTO daily_stats(date) VALUES(?) ON CONFLICT(date) DO NOTHING`, today)
	_, _ = h.db.Exec(`UPDATE daily_stats SET new_users = new_users + 1 WHERE date=?`, today)

	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"newapi_user_id": userID,
			"username":       username,
			"password":       password,
		},
	})
}

func (h *Handler) handleWhitelist(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT newapi_user_id, username, display_name, discord_id, discord_name, created_at FROM users WHERE is_whitelist=1 ORDER BY created_at DESC`)
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var items []map[string]any
	for rows.Next() {
		var userID int64
		var username, displayName, discordID, discordName sql.NullString
		var createdAt string
		if err := rows.Scan(&userID, &username, &displayName, &discordID, &discordName, &createdAt); err != nil {
			webutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, map[string]any{
			"newapi_user_id": userID,
			"username":       username.String,
			"display_name":   displayName.String,
			"discord_id":     discordID.String,
			"discord_name":   discordName.String,
			"is_whitelist":   true,
			"created_at":     createdAt,
		})
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "items": items})
}

func (h *Handler) handleWhitelistToggle(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/guard/api/whitelist/")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || userID <= 0 {
		webutil.WriteError(w, http.StatusBadRequest, "用户 ID 无效")
		return
	}
	flag := 0
	if r.Method == http.MethodPost {
		flag = 1
	}
	if _, err := h.db.Exec(`UPDATE users SET is_whitelist=? WHERE newapi_user_id=?`, flag, userID); err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.cache.SetWhitelist(userID, flag == 1)
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

type localUserRecord struct {
	UserID      int64
	Username    string
	DisplayName string
	DiscordID   string
	DiscordName string
	IsWhitelist bool
	CreatedAt   string
}

func (h *Handler) queryLocalUsers(page, size int, search string) ([]localUserRecord, error) {
	query := `SELECT newapi_user_id, username, display_name, discord_id, discord_name, is_whitelist, created_at FROM users`
	args := []any{}
	if search != "" {
		query += ` WHERE CAST(newapi_user_id AS TEXT) LIKE ? OR username LIKE ? OR display_name LIKE ? OR discord_id LIKE ? OR discord_name LIKE ?`
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, size, (page-1)*size)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []localUserRecord{}
	for rows.Next() {
		var record localUserRecord
		var username, displayName, discordID, discordName sql.NullString
		var whitelist int
		if err := rows.Scan(&record.UserID, &username, &displayName, &discordID, &discordName, &whitelist, &record.CreatedAt); err != nil {
			return nil, err
		}
		record.Username = username.String
		record.DisplayName = displayName.String
		record.DiscordID = discordID.String
		record.DiscordName = discordName.String
		record.IsWhitelist = whitelist == 1
		items = append(items, record)
	}
	return items, rows.Err()
}

func (h *Handler) loadLocalUserMap() (map[int64]localUserRecord, error) {
	rows, err := h.db.Query(`SELECT newapi_user_id, username, display_name, discord_id, discord_name, is_whitelist, created_at FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := map[int64]localUserRecord{}
	for rows.Next() {
		var record localUserRecord
		var username, displayName, discordID, discordName sql.NullString
		var whitelist int
		if err := rows.Scan(&record.UserID, &username, &displayName, &discordID, &discordName, &whitelist, &record.CreatedAt); err != nil {
			return nil, err
		}
		record.Username = username.String
		record.DisplayName = displayName.String
		record.DiscordID = discordID.String
		record.DiscordName = discordName.String
		record.IsWhitelist = whitelist == 1
		items[record.UserID] = record
	}
	return items, rows.Err()
}

func (h *Handler) buildUserItem(remoteUser *newapi.User, localUser localUserRecord) map[string]any {
	item := map[string]any{
		"newapi_user_id": localUser.UserID,
		"username":       localUser.Username,
		"display_name":   localUser.DisplayName,
		"discord_id":     localUser.DiscordID,
		"discord_name":   localUser.DiscordName,
		"is_whitelist":   localUser.IsWhitelist,
		"created_at":     localUser.CreatedAt,
	}
	if remoteUser == nil {
		return item
	}
	item["newapi_user_id"] = remoteUser.ID
	if remoteUser.Username != "" {
		item["username"] = remoteUser.Username
	}
	if remoteUser.DisplayName != "" {
		item["display_name"] = remoteUser.DisplayName
	}
	item["status"] = remoteUser.Status
	item["quota"] = remoteUser.Quota
	item["group"] = remoteUser.Group
	item["email"] = remoteUser.Email
	item["last_login_at"] = remoteUser.LastLoginAt
	item["created_at_unix"] = remoteUser.CreatedAt
	if item["created_at"] == "" && remoteUser.CreatedAt > 0 {
		item["created_at"] = time.Unix(remoteUser.CreatedAt, 0).UTC().Format(time.RFC3339)
	}
	return item
}

func (h *Handler) syncDiscordOAuthBindings(ctx context.Context) error {
	bindings, err := h.tokens.ListDiscordOAuthBindings(ctx)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if binding.UserID <= 0 || binding.DiscordID == "" {
			continue
		}
		_, _ = h.db.Exec(`INSERT INTO users(newapi_user_id, username, display_name, discord_id, discord_name)
			VALUES(?, ?, ?, ?, COALESCE(NULLIF((SELECT discord_name FROM oauth_identity_links WHERE discord_id=?), ''), ?))
			ON CONFLICT(newapi_user_id) DO UPDATE SET
				username=CASE WHEN excluded.username != '' THEN excluded.username ELSE users.username END,
				display_name=CASE WHEN excluded.display_name != '' THEN excluded.display_name ELSE users.display_name END,
				discord_id=excluded.discord_id,
				discord_name=CASE WHEN excluded.discord_name != '' THEN excluded.discord_name ELSE users.discord_name END`,
			binding.UserID, binding.Username, binding.DisplayName, binding.DiscordID, binding.DiscordID, binding.DisplayName)
		_, _ = h.db.Exec(`UPDATE oauth_identity_links SET newapi_user_id=?, updated_at=CURRENT_TIMESTAMP WHERE discord_id=?`, binding.UserID, binding.DiscordID)
	}
	return nil
}

func (h *Handler) ensureLocalUserExists(userID int64) error {
	if userID <= 0 {
		return nil
	}
	_, err := h.db.Exec(`INSERT INTO users(newapi_user_id) VALUES(?) ON CONFLICT(newapi_user_id) DO NOTHING`, userID)
	return err
}

func (h *Handler) upsertLocalUserIdentity(userID int64, username, displayName string) error {
	if userID <= 0 {
		return nil
	}
	_, err := h.db.Exec(`INSERT INTO users(newapi_user_id, username, display_name) VALUES(?, ?, ?)
		ON CONFLICT(newapi_user_id) DO UPDATE SET
			username=CASE WHEN excluded.username != '' THEN excluded.username ELSE users.username END,
			display_name=CASE WHEN excluded.display_name != '' THEN excluded.display_name ELSE users.display_name END`, userID, username, displayName)
	return err
}
