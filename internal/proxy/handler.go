package proxy

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	newapi   *newapi.Client
	proxy    *httputil.ReverseProxy
}

func NewHandler(env config.Env, db *sql.DB, settingsStore *settings.Store, cacheStore *cache.Store, newapiClient *newapi.Client) *Handler {
	target, _ := url.Parse(strings.TrimRight(env.NewAPIURL, "/"))
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.FlushInterval = -1
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		webutil.WriteError(w, http.StatusBadGateway, "上游服务不可用")
	}
	rp.Director = func(r *http.Request) {
		r.URL.Scheme = target.Scheme
		r.URL.Host = target.Host
		r.URL.Path = singleJoin(target.Path, "/v1"+r.URL.Path)
		r.Host = target.Host
	}
	return &Handler{
		env:      env,
		db:       db,
		settings: settingsStore,
		cache:    cacheStore,
		newapi:   newapiClient,
		proxy:    rp,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet || r.Method == http.MethodOptions {
		h.proxy.ServeHTTP(w, r)
		return
	}

	token := extractToken(r)
	if token == "" {
		h.proxy.ServeHTTP(w, r)
		return
	}

	userID, ok, err := h.resolveUserID(r.Context(), token)
	if err != nil {
		webutil.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	if !ok {
		h.proxy.ServeHTTP(w, r)
		return
	}

	if h.cache.IsWhitelist(userID) || h.loadWhitelist(userID) {
		h.proxy.ServeHTTP(w, r)
		return
	}

	if !h.allowedUA(r.UserAgent()) {
		h.handleUAViolation(w, r, userID)
		return
	}

	limit := h.settings.GetInt("rpm_limit", 3)
	key := fmt.Sprintf("%d:%s", userID, time.Now().Format("200601021504"))
	count := h.cache.IncrementRPM(key, time.Minute)
	if count > limit {
		h.bumpDailyStat("blocked_rpm")
		webutil.WriteJSON(w, http.StatusTooManyRequests, map[string]any{
			"error": map[string]any{
				"message": fmt.Sprintf("Rate limit: %d requests/min", limit),
				"type":    "rate_limit_error",
			},
		})
		return
	}

	h.bumpDailyStat("total_requests")
	h.proxy.ServeHTTP(w, r)
}

func (h *Handler) resolveUserID(ctx context.Context, token string) (int64, bool, error) {
	if userID, ok := h.cache.GetToken(token); ok {
		return userID, true, nil
	}

	var userID int64
	if err := h.db.QueryRow(`SELECT newapi_user_id FROM token_cache WHERE token_key=?`, token).Scan(&userID); err == nil {
		h.cache.SetToken(token, userID, 5*time.Minute)
		return userID, true, nil
	}

	adminToken := h.settings.GetString("newapi_admin_token")
	if adminToken == "" {
		return 0, false, nil
	}
	resolved, ok, err := h.newapi.SearchToken(ctx, adminToken, token)
	if err != nil || !ok {
		return 0, false, err
	}
	_, _ = h.db.Exec(`INSERT INTO users(newapi_user_id) VALUES(?) ON CONFLICT(newapi_user_id) DO NOTHING`, resolved)
	_, _ = h.db.Exec(`INSERT INTO token_cache(token_key, newapi_user_id) VALUES(?, ?)
		ON CONFLICT(token_key) DO UPDATE SET newapi_user_id=excluded.newapi_user_id, cached_at=CURRENT_TIMESTAMP`, token, resolved)
	h.cache.SetToken(token, resolved, 5*time.Minute)
	return resolved, true, nil
}

func (h *Handler) loadWhitelist(userID int64) bool {
	var flag int
	if err := h.db.QueryRow(`SELECT is_whitelist FROM users WHERE newapi_user_id=?`, userID).Scan(&flag); err != nil {
		return false
	}
	h.cache.SetWhitelist(userID, flag == 1)
	return flag == 1
}

func (h *Handler) allowedUA(ua string) bool {
	allowed := h.settings.GetStringSlice("allowed_ua")
	if len(allowed) == 0 {
		return true
	}
	for _, prefix := range allowed {
		if strings.HasPrefix(ua, prefix) {
			return true
		}
	}
	return false
}

func (h *Handler) handleUAViolation(w http.ResponseWriter, r *http.Request, userID int64) {
	count := h.incrementUA(userID, r.UserAgent())
	max := h.settings.GetInt("ua_ban_strikes", 3)
	h.bumpDailyStat("blocked_ua")
	if count >= max {
		_ = h.banUser(r.Context(), userID, "ua_violation", r.UserAgent(), r.RemoteAddr)
		webutil.WriteJSON(w, http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"message": "Account banned: unauthorized client",
				"type":    "access_denied",
			},
		})
		return
	}
	webutil.WriteJSON(w, http.StatusForbidden, map[string]any{
		"error": map[string]any{
			"message": fmt.Sprintf("Unauthorized client (%d/%d)", count, max),
			"type":    "access_denied",
		},
	})
}

func (h *Handler) incrementUA(userID int64, ua string) int {
	var count int
	_ = h.db.QueryRow(`SELECT count FROM ua_strikes WHERE newapi_user_id=?`, userID).Scan(&count)
	count++
	_, _ = h.db.Exec(`INSERT INTO ua_strikes(newapi_user_id, count, last_ua, last_strike_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(newapi_user_id) DO UPDATE SET
			count=excluded.count,
			last_ua=excluded.last_ua,
			last_strike_at=excluded.last_strike_at`, userID, count, ua)
	return count
}

func (h *Handler) banUser(ctx context.Context, userID int64, reason, ua, ip string) error {
	adminToken := h.settings.GetString("newapi_admin_token")
	if adminToken != "" {
		_ = h.newapi.UpdateUserStatus(ctx, adminToken, userID, 2)
	}
	var discordID sql.NullString
	_ = h.db.QueryRow(`SELECT discord_id FROM users WHERE newapi_user_id=?`, userID).Scan(&discordID)
	_, err := h.db.Exec(`INSERT INTO bans(newapi_user_id, discord_id, reason, violation_ua, client_ip, duration, created_at)
		VALUES(?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, userID, discordID.String, reason, ua, ip, "permanent")
	if err == nil {
		h.bumpDailyStat("new_bans")
	}
	return err
}

func (h *Handler) bumpDailyStat(field string) {
	today := time.Now().Format("2006-01-02")
	_, _ = h.db.Exec(`INSERT INTO daily_stats(date) VALUES(?) ON CONFLICT(date) DO NOTHING`, today)
	_, _ = h.db.Exec(`UPDATE daily_stats SET `+field+` = `+field+` + 1 WHERE date=?`, today)
}

func extractToken(r *http.Request) string {
	if token := webutil.BearerToken(r); token != "" {
		return token
	}
	if token := r.Header.Get("x-api-key"); token != "" {
		return token
	}
	if token := r.Header.Get("api-key"); token != "" {
		return token
	}
	return ""
}

func singleJoin(a, b string) string {
	if strings.HasSuffix(a, "/") {
		a = strings.TrimSuffix(a, "/")
	}
	if !strings.HasPrefix(b, "/") {
		b = "/" + b
	}
	return a + b
}
