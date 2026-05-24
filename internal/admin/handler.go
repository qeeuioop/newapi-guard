package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"newapiguard/internal/cache"
	"newapiguard/internal/config"
	"newapiguard/internal/newapi"
	"newapiguard/internal/settings"
	"newapiguard/internal/webutil"
)

type Handler struct {
	env      config.Env
	db       *sql.DB
	settings *settings.Store
	cache    *cache.Store
	sessions *SessionStore
	newapi   *newapi.Client
}

func NewHandler(env config.Env, db *sql.DB, settingsStore *settings.Store, cacheStore *cache.Store, sessions *SessionStore, newapiClient *newapi.Client) *Handler {
	return &Handler{
		env:      env,
		db:       db,
		settings: settingsStore,
		cache:    cacheStore,
		sessions: sessions,
		newapi:   newapiClient,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/guard/admin/" || r.URL.Path == "/guard/admin/index.html":
		http.ServeFile(w, r, h.env.WebDir+"/index.html")
	case strings.HasPrefix(r.URL.Path, "/guard/static/"):
		http.StripPrefix("/guard/static/", http.FileServer(http.Dir(h.env.WebDir))).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/auth/login" && r.Method == http.MethodPost:
		h.handleLogin(w, r)
	case r.URL.Path == "/guard/api/auth/logout" && r.Method == http.MethodPost:
		h.handleLogout(w, r)
	case r.URL.Path == "/guard/api/dashboard" && r.Method == http.MethodGet:
		h.withAuth(http.HandlerFunc(h.handleDashboard)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/settings" && r.Method == http.MethodGet:
		h.withAuth(http.HandlerFunc(h.handleSettingsGet)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/settings" && r.Method == http.MethodPut:
		h.withAuth(http.HandlerFunc(h.handleSettingsPut)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/users" && r.Method == http.MethodGet:
		h.withAuth(http.HandlerFunc(h.handleUsers)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/users" && r.Method == http.MethodPost:
		h.withAuth(http.HandlerFunc(h.handleCreateUser)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/whitelist" && r.Method == http.MethodGet:
		h.withAuth(http.HandlerFunc(h.handleWhitelist)).ServeHTTP(w, r)
	case strings.HasPrefix(r.URL.Path, "/guard/api/whitelist/"):
		h.withAuth(http.HandlerFunc(h.handleWhitelistToggle)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/bans" && r.Method == http.MethodGet:
		h.withAuth(http.HandlerFunc(h.handleBans)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/bans" && r.Method == http.MethodPost:
		h.withAuth(http.HandlerFunc(h.handleCreateBan)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/bans/unban" && r.Method == http.MethodPost:
		h.withAuth(http.HandlerFunc(h.handleUnbanByUser)).ServeHTTP(w, r)
	case strings.HasPrefix(r.URL.Path, "/guard/api/bans/") && strings.HasSuffix(r.URL.Path, "/unban") && r.Method == http.MethodPost:
		h.withAuth(http.HandlerFunc(h.handleUnban)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/logs/bans" && r.Method == http.MethodGet:
		h.withAuth(http.HandlerFunc(h.handleBanLogs)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/logs/checkins" && r.Method == http.MethodGet:
		h.withAuth(http.HandlerFunc(h.handleCheckinLogs)).ServeHTTP(w, r)
	case r.URL.Path == "/guard/api/logs/stats" && r.Method == http.MethodGet:
		h.withAuth(http.HandlerFunc(h.handleStatsLogs)).ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) withAuth(next http.Handler) http.Handler {
	return h.sessions.Middleware(next)
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := webutil.ReadJSON(r, &req); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	if req.Password == "" || req.Password != h.settings.GetString("admin_password") {
		webutil.WriteError(w, http.StatusUnauthorized, "密码错误")
		return
	}
	token := h.sessions.Create()
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"token":     token,
		"expire_in": int(h.sessions.ttl / time.Second),
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	} else {
		token = ""
	}
	h.sessions.Delete(token)
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	today := time.Now().Format("2006-01-02")
	var totalUsers, whitelistCount int
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_whitelist=1`).Scan(&whitelistCount)
	activeBans := h.countActiveBans(r.Context())

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

	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"today":           stats,
		"total_users":     totalUsers,
		"active_bans":     activeBans,
		"whitelist_count": whitelistCount,
	})
}

func (h *Handler) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"rpm_limit":               h.settings.GetInt("rpm_limit", 3),
			"ua_ban_strikes":          h.settings.GetInt("ua_ban_strikes", 3),
			"allowed_ua":              h.settings.GetStringSlice("allowed_ua"),
			"checkin_quota":           h.settings.GetInt("checkin_quota", 500000),
			"checkin_threshold":       h.settings.GetInt("checkin_threshold", 200000),
			"newapi_base_url":         h.settings.GetString("newapi_base_url"),
			"newapi_admin_token":      h.settings.GetString("newapi_admin_token"),
			"newapi_admin_user_id":    h.settings.GetString("newapi_admin_user_id"),
			"public_base_url":         h.settings.GetString("public_base_url"),
			"admin_password":          h.settings.GetString("admin_password"),
			"oauth_client_id":         h.settings.GetString("oauth_client_id"),
			"oauth_client_secret":     h.settings.GetString("oauth_client_secret"),
			"oauth_provider_slug":     h.settings.GetString("oauth_provider_slug"),
			"oauth_state_ttl_seconds": h.settings.GetInt("oauth_state_ttl_seconds", 300),
			"oauth_code_ttl_seconds":  h.settings.GetInt("oauth_code_ttl_seconds", 120),
			"oauth_token_ttl_seconds": h.settings.GetInt("oauth_token_ttl_seconds", 600),
			"discord_client_id":       h.settings.GetString("discord_client_id"),
			"discord_client_secret":   h.settings.GetString("discord_client_secret"),
			"discord_guild_id":        h.settings.GetString("discord_guild_id"),
			"discord_oauth_scopes":    h.settings.GetStringSlice("discord_oauth_scopes"),
			"discord_access_policy":   h.settings.GetString("discord_access_policy"),
		},
	})
}

func (h *Handler) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	updates := map[string]string{}
	for key, value := range payload {
		switch v := value.(type) {
		case string:
			updates[key] = v
		default:
			data, _ := json.Marshal(v)
			updates[key] = string(data)
		}
	}
	if err := h.settings.Update(updates); err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if newAPIURL, ok := updates["newapi_base_url"]; ok && strings.TrimSpace(newAPIURL) != "" {
		h.newapi.SetBaseURL(newAPIURL)
	}
	if newAPIAdminUserID, ok := updates["newapi_admin_user_id"]; ok {
		h.newapi.SetAdminUserID(newAPIAdminUserID)
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

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
		_ = h.ensureLocalUserExists(remoteUser.ID)
		items = append(items, h.buildUserItem(&remoteUser, localMap[remoteUser.ID]))
	}

	if search != "" {
		localMatches, err := h.queryLocalUsers(1, maxInt(size*3, 100), search)
		if err == nil {
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

func (h *Handler) handleWhitelist(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT newapi_user_id, discord_id, discord_name, created_at FROM users WHERE is_whitelist=1 ORDER BY created_at DESC`)
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var items []map[string]any
	for rows.Next() {
		var userID int64
		var discordID, discordName sql.NullString
		var createdAt string
		if err := rows.Scan(&userID, &discordID, &discordName, &createdAt); err != nil {
			webutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, map[string]any{
			"newapi_user_id": userID,
			"discord_id":     discordID.String,
			"discord_name":   discordName.String,
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

func parseIntDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return fallback
}

type localUserRecord struct {
	UserID      int64
	DiscordID   string
	DiscordName string
	IsWhitelist bool
	CreatedAt   string
}

func (h *Handler) queryLocalUsers(page, size int, search string) ([]localUserRecord, error) {
	query := `SELECT newapi_user_id, discord_id, discord_name, is_whitelist, created_at FROM users`
	args := []any{}
	if search != "" {
		query += ` WHERE CAST(newapi_user_id AS TEXT) LIKE ? OR discord_id LIKE ? OR discord_name LIKE ?`
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern, pattern)
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
		var discordID, discordName sql.NullString
		var whitelist int
		if err := rows.Scan(&record.UserID, &discordID, &discordName, &whitelist, &record.CreatedAt); err != nil {
			return nil, err
		}
		record.DiscordID = discordID.String
		record.DiscordName = discordName.String
		record.IsWhitelist = whitelist == 1
		items = append(items, record)
	}
	return items, rows.Err()
}

func (h *Handler) loadLocalUserMap() (map[int64]localUserRecord, error) {
	rows, err := h.db.Query(`SELECT newapi_user_id, discord_id, discord_name, is_whitelist, created_at FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := map[int64]localUserRecord{}
	for rows.Next() {
		var record localUserRecord
		var discordID, discordName sql.NullString
		var whitelist int
		if err := rows.Scan(&record.UserID, &discordID, &discordName, &whitelist, &record.CreatedAt); err != nil {
			return nil, err
		}
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
		"discord_id":     localUser.DiscordID,
		"discord_name":   localUser.DiscordName,
		"is_whitelist":   localUser.IsWhitelist,
		"created_at":     localUser.CreatedAt,
	}
	if remoteUser == nil {
		return item
	}
	item["newapi_user_id"] = remoteUser.ID
	item["username"] = remoteUser.Username
	item["display_name"] = remoteUser.DisplayName
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

func (h *Handler) ensureLocalUserExists(userID int64) error {
	if userID <= 0 {
		return nil
	}
	_, err := h.db.Exec(`INSERT INTO users(newapi_user_id) VALUES(?) ON CONFLICT(newapi_user_id) DO NOTHING`, userID)
	return err
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

func (h *Handler) countActiveBans(ctx context.Context) int {
	adminToken := h.settings.GetString("newapi_admin_token")
	if adminToken == "" {
		var activeBans int
		_ = h.db.QueryRow(`SELECT COUNT(*) FROM bans WHERE unbanned_at IS NULL AND (expire_at IS NULL OR expire_at > CURRENT_TIMESTAMP)`).Scan(&activeBans)
		return activeBans
	}

	remoteUsers, err := h.fetchAllNewAPIUsers(ctx, adminToken)
	if err != nil {
		var activeBans int
		_ = h.db.QueryRow(`SELECT COUNT(*) FROM bans WHERE unbanned_at IS NULL AND (expire_at IS NULL OR expire_at > CURRENT_TIMESTAMP)`).Scan(&activeBans)
		return activeBans
	}
	count := 0
	for _, user := range remoteUsers {
		if user.Status == 2 {
			count++
		}
	}
	return count
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

func nullable(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func (h *Handler) handleBans(w http.ResponseWriter, r *http.Request) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = "active"
	}
	if status == "active" {
		h.handleActiveBans(w, r)
		return
	}

	query := `SELECT id, newapi_user_id, discord_id, reason, violation_ua, client_ip, duration, expire_at, unbanned_at, created_at
		FROM bans`
	query += ` ORDER BY created_at DESC`

	rows, err := h.db.Query(query)
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
