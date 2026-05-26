package proxy

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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
	env           config.Env
	db            *sql.DB
	settings      *settings.Store
	cache         *cache.Store
	newapi        *newapi.Client
	proxy         *httputil.ReverseProxy
	tokenCacheTTL time.Duration
}

func NewHandler(env config.Env, db *sql.DB, settingsStore *settings.Store, cacheStore *cache.Store, newapiClient *newapi.Client) *Handler {
	rp := &httputil.ReverseProxy{}
	rp.FlushInterval = -1
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		webutil.WriteError(w, http.StatusBadGateway, "上游服务不可用")
	}
	rp.Director = func(r *http.Request) {
		target, err := url.Parse(strings.TrimRight(newapiClient.BaseURL(), "/"))
		if err != nil || target == nil || target.Host == "" {
			log.Printf("[proxy] newapi_base_url 解析失败: %v", err)
			r.URL.Scheme = ""
			r.URL.Host = ""
			r.Host = ""
			return
		}
		r.URL.Scheme = target.Scheme
		r.URL.Host = target.Host
		r.URL.Path = singleJoin(target.Path, "/v1"+r.URL.Path)
		r.Host = target.Host
	}
	return &Handler{
		env:           env,
		db:            db,
		settings:      settingsStore,
		cache:         cacheStore,
		newapi:        newapiClient,
		proxy:         rp,
		tokenCacheTTL: env.TokenCacheTTL,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		h.proxy.ServeHTTP(w, r)
		return
	}

	token := extractToken(r)
	if token == "" {
		h.proxy.ServeHTTP(w, r)
		return
	}

	if r.Method == http.MethodGet {
		userID, ok, err := h.resolveUserID(r.Context(), token)
		if err != nil {
			webutil.WriteError(w, http.StatusBadGateway, err.Error())
			return
		}
		if ok && h.isUserBanned(userID) {
			h.writeAccessDenied(w, "账户已封禁")
			return
		}
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

	if h.isUserBanned(userID) {
		h.writeAccessDenied(w, "账户已封禁")
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
		h.writeRateLimitError(w, fmt.Sprintf("请求过于频繁，请稍后再试（%d/%d 次/分钟）", count, limit))
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
	if err := h.db.QueryRow(`SELECT newapi_user_id FROM token_cache WHERE token_key=? AND cached_at > datetime('now', ?)`, token, ttlSQLiteModifier(h.tokenCacheTTL)).Scan(&userID); err == nil {
		h.cache.SetToken(token, userID, h.tokenCacheTTL)
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
	h.cache.SetToken(token, resolved, h.tokenCacheTTL)
	return resolved, true, nil
}

func ttlSQLiteModifier(ttl time.Duration) string {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return fmt.Sprintf("-%d seconds", int(ttl.Seconds()))
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
		if err := h.banUser(r.Context(), userID, "ua_violation", r.UserAgent(), r.RemoteAddr); err != nil {
			h.writeAccessDenied(w, fmt.Sprintf("未在特定客户端内使用，账号已封禁（%d/%d）", count, max))
			return
		}
		h.writeAccessDenied(w, fmt.Sprintf("账户已封禁（未在特定客户端内使用，%d/%d）", count, max))
		return
	}
	h.writeAccessDenied(w, fmt.Sprintf("未在特定客户端内使用（%d/%d）", count, max))
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
		if err := h.newapi.UpdateUserStatus(ctx, adminToken, userID, 2); err != nil {
			return err
		}
	}
	var discordID sql.NullString
	_ = h.db.QueryRow(`SELECT discord_id FROM users WHERE newapi_user_id=?`, userID).Scan(&discordID)

	duration := h.settings.GetString("ua_auto_ban_duration")
	if duration == "" {
		duration = "permanent"
	}
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

	_, err := h.db.Exec(`INSERT INTO bans(newapi_user_id, discord_id, reason, violation_ua, client_ip, duration, expire_at, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, userID, discordID.String, reason, ua, ip, duration, expireAt)
	if err == nil {
		h.bumpDailyStat("new_bans")
	}
	return err
}

var allowedStatFields = map[string]bool{
	"total_requests": true,
	"blocked_ua":     true,
	"blocked_rpm":    true,
	"checkins":       true,
	"new_users":      true,
	"new_bans":       true,
}

func (h *Handler) bumpDailyStat(field string) {
	if !allowedStatFields[field] {
		return
	}
	today := time.Now().Format("2006-01-02")
	_, _ = h.db.Exec(`INSERT INTO daily_stats(date) VALUES(?) ON CONFLICT(date) DO NOTHING`, today)
	_, _ = h.db.Exec(`UPDATE daily_stats SET `+field+` = `+field+` + 1 WHERE date=?`, today)
}

func (h *Handler) isUserBanned(userID int64) bool {
	var marker int
	err := h.db.QueryRow(`SELECT 1
		FROM bans
		WHERE newapi_user_id=?
		  AND unbanned_at IS NULL
		  AND (expire_at IS NULL OR expire_at > CURRENT_TIMESTAMP)
		LIMIT 1`, userID).Scan(&marker)
	return err == nil
}

func (h *Handler) writeAccessDenied(w http.ResponseWriter, message string) {
	webutil.WriteJSON(w, http.StatusForbidden, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "access_denied",
		},
	})
}

func (h *Handler) writeRateLimitError(w http.ResponseWriter, message string) {
	webutil.WriteJSON(w, http.StatusTooManyRequests, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "rate_limit_error",
		},
	})
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
