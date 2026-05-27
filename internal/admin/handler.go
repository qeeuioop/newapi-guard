package admin

import (
	"database/sql"
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

type SessionProvider interface {
	Create() string
	Validate(token string) bool
	Delete(token string)
	Middleware(next http.Handler) http.Handler
	TTL() time.Duration
}

type Handler struct {
	env      config.Env
	db       *sql.DB
	settings *settings.Store
	cache    *cache.Store
	sessions SessionProvider
	newapi   *newapi.Client
	tokens   *newapi.TokenResolver
	limiter  *LoginLimiter
}

func NewHandler(env config.Env, db *sql.DB, settingsStore *settings.Store, cacheStore *cache.Store, sessions SessionProvider, newapiClient *newapi.Client, tokenResolver *newapi.TokenResolver) *Handler {
	return &Handler{
		env:      env,
		db:       db,
		settings: settingsStore,
		cache:    cacheStore,
		sessions: sessions,
		newapi:   newapiClient,
		tokens:   tokenResolver,
		limiter:  NewLoginLimiter(5, 15*time.Minute),
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
	ip := clientIP(r)
	if h.limiter.IsLocked(ip) {
		webutil.WriteError(w, http.StatusTooManyRequests, "登录尝试过多，请稍后再试")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := webutil.ReadJSON(r, &req); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	storedPassword := h.settings.GetString("admin_password")
	if req.Password == "" || storedPassword == "" || !webutil.ConstantTimeEqual(req.Password, storedPassword) {
		h.limiter.RecordFailure(ip)
		webutil.WriteError(w, http.StatusUnauthorized, "密码错误")
		return
	}
	h.limiter.ClearFailures(ip)
	token := h.sessions.Create()
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"token":     token,
		"expire_in": int(h.sessions.TTL() / time.Second),
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

func parseIntDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return fallback
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
