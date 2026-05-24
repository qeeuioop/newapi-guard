package admin

import (
	"database/sql"
	"encoding/json"
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
	case r.URL.Path == "/guard/api/whitelist" && r.Method == http.MethodGet:
		h.withAuth(http.HandlerFunc(h.handleWhitelist)).ServeHTTP(w, r)
	case strings.HasPrefix(r.URL.Path, "/guard/api/whitelist/"):
		h.withAuth(http.HandlerFunc(h.handleWhitelistToggle)).ServeHTTP(w, r)
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
		"success": true,
		"token":   token,
		"expire_in": int(h.sessions.ttl / time.Second),
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	today := time.Now().Format("2006-01-02")
	var totalUsers, whitelistCount, activeBans int
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_whitelist=1`).Scan(&whitelistCount)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM bans WHERE unbanned_at IS NULL AND (expire_at IS NULL OR expire_at > CURRENT_TIMESTAMP)`).Scan(&activeBans)

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
		"today":          stats,
		"total_users":    totalUsers,
		"active_bans":    activeBans,
		"whitelist_count": whitelistCount,
	})
}

func (h *Handler) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"rpm_limit":            h.settings.GetInt("rpm_limit", 3),
			"ua_ban_strikes":       h.settings.GetInt("ua_ban_strikes", 3),
			"allowed_ua":           h.settings.GetStringSlice("allowed_ua"),
			"checkin_quota":        h.settings.GetInt("checkin_quota", 500000),
			"checkin_threshold":    h.settings.GetInt("checkin_threshold", 200000),
			"oauth_client_id":      h.settings.GetString("oauth_client_id"),
			"oauth_client_secret":   h.settings.GetString("oauth_client_secret"),
			"oauth_provider_slug":   h.settings.GetString("oauth_provider_slug"),
			"discord_client_id":     h.settings.GetString("discord_client_id"),
			"discord_client_secret": h.settings.GetString("discord_client_secret"),
			"discord_guild_id":      h.settings.GetString("discord_guild_id"),
			"discord_access_policy": h.settings.GetString("discord_access_policy"),
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
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) handleUsers(w http.ResponseWriter, r *http.Request) {
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	size := parseIntDefault(r.URL.Query().Get("size"), 20)
	search := strings.TrimSpace(r.URL.Query().Get("search"))

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
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var items []map[string]any
	for rows.Next() {
		var userID int64
		var discordID, discordName sql.NullString
		var whitelist int
		var createdAt string
		if err := rows.Scan(&userID, &discordID, &discordName, &whitelist, &createdAt); err != nil {
			webutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, map[string]any{
			"newapi_user_id": userID,
			"discord_id":     discordID.String,
			"discord_name":   discordName.String,
			"is_whitelist":   whitelist == 1,
			"created_at":     createdAt,
		})
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"page":    page,
		"size":    size,
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
