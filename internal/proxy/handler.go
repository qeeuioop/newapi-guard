package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"newapiguard/internal/cache"
	"newapiguard/internal/config"
	"newapiguard/internal/guardban"
	"newapiguard/internal/newapi"
	"newapiguard/internal/promptcache"
	"newapiguard/internal/settings"
	"newapiguard/internal/webutil"
)

const maxMessagesRequestBody = 20 << 20 // 20 MB

type Handler struct {
	env           config.Env
	db            *sql.DB
	settings      *settings.Store
	cache         *cache.Store
	newapi        *newapi.Client
	tokens        *newapi.TokenResolver
	proxy         *httputil.ReverseProxy
	tokenCacheTTL time.Duration
}

func NewHandler(env config.Env, db *sql.DB, settingsStore *settings.Store, cacheStore *cache.Store, newapiClient *newapi.Client, tokenResolver *newapi.TokenResolver) *Handler {
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
		tokens:        tokenResolver,
		proxy:         rp,
		tokenCacheTTL: env.TokenCacheTTL,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.bumpDailyStat("total_requests")

	if r.Method == http.MethodOptions {
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
		log.Printf("[proxy] resolve token failed: %v", err)
		webutil.WriteError(w, http.StatusBadGateway, "无法验证 API Key")
		return
	}
	if !ok {
		h.writeAccessDenied(w, "无法识别 API Key")
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

	if r.Method != http.MethodGet {
		// Keep GET (for example /v1/models) lenient: model-list/read requests should not count as UA violations.
		// Actual API calls use POST and still enter UA strike / auto-ban handling.
		if !h.allowedUA(r.UserAgent()) {
			h.handleUAViolation(w, r, userID)
			return
		}
	}

	limit := h.settings.GetInt("rpm_limit", 3)
	key := fmt.Sprintf("%d:%s", userID, time.Now().Format("200601021504"))
	count := h.cache.IncrementRPM(key, time.Minute)
	if count > limit {
		h.bumpDailyStat("blocked_rpm")
		h.writeRateLimitError(w, fmt.Sprintf("请求过于频繁，请稍后再试（%d/%d 次/分钟）", count, limit))
		return
	}

	if err := h.maybeInjectPromptCache(w, r); err != nil {
		log.Printf("[proxy] prompt cache injection failed: %v", err)
		webutil.WriteError(w, http.StatusBadRequest, "请求体无效或过大")
		return
	}

	h.proxy.ServeHTTP(w, r)
}

func (h *Handler) maybeInjectPromptCache(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost || r.URL.Path != "/messages" {
		return nil
	}
	if !h.settings.GetBool("prompt_cache_enabled", true) {
		return nil
	}

	bodyBytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxMessagesRequestBody))
	r.Body.Close()
	if err != nil {
		return err
	}

	newBody, report, changed := promptcache.Inject(bodyBytes, promptcache.Options{})
	if h.settings.GetBool("prompt_cache_debug", false) {
		promptcache.LogReport("/v1"+r.URL.Path, report)
	}
	if !changed {
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		r.ContentLength = int64(len(bodyBytes))
		r.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
		return nil
	}

	r.Body = io.NopCloser(bytes.NewReader(newBody))
	r.ContentLength = int64(len(newBody))
	r.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
	return nil
}

func (h *Handler) resolveUserID(ctx context.Context, token string) (int64, bool, error) {
	if userID, ok := h.cache.GetToken(token); ok {
		return userID, true, nil
	}

	var userID int64
	tokenHash := hashToken(token)
	if err := h.db.QueryRow(`SELECT newapi_user_id FROM token_cache WHERE token_key=? AND cached_at > datetime('now', ?)`, tokenHash, ttlSQLiteModifier(h.tokenCacheTTL)).Scan(&userID); err == nil {
		h.cache.SetToken(token, userID, h.tokenCacheTTL)
		return userID, true, nil
	}

	resolvedToken, ok, err := h.tokens.ResolveToken(ctx, token)
	resolved := resolvedToken.UserID
	if err != nil {
		return 0, false, err
	}
	if ok {
		displayName := resolvedToken.DisplayName
		if displayName == "" {
			displayName = resolvedToken.Name
		}
		_, _ = h.db.Exec(`INSERT INTO users(newapi_user_id, username, display_name) VALUES(?, ?, ?)
			ON CONFLICT(newapi_user_id) DO UPDATE SET
				username=CASE WHEN excluded.username != '' THEN excluded.username ELSE users.username END,
				display_name=CASE WHEN excluded.display_name != '' THEN excluded.display_name ELSE users.display_name END`, resolved, resolvedToken.Username, displayName)
	}
	if !ok {
		adminToken := h.settings.GetString("newapi_admin_token")
		if adminToken == "" {
			return 0, false, nil
		}
		resolved, ok, err = h.newapi.SearchToken(ctx, adminToken, token)
		if err != nil || !ok {
			return 0, false, err
		}
	}
	_, _ = h.db.Exec(`INSERT INTO users(newapi_user_id) VALUES(?) ON CONFLICT(newapi_user_id) DO NOTHING`, resolved)
	_, _ = h.db.Exec(`INSERT INTO token_cache(token_key, newapi_user_id) VALUES(?, ?)
		ON CONFLICT(token_key) DO UPDATE SET newapi_user_id=excluded.newapi_user_id, cached_at=CURRENT_TIMESTAMP`, tokenHash, resolved)
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
	lowerUA := strings.ToLower(ua)
	for _, prefix := range allowed {
		if strings.HasPrefix(lowerUA, strings.ToLower(prefix)) {
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
		if err := h.banUser(r.Context(), userID, "ua_violation", r.UserAgent(), webutil.ClientIP(r)); err != nil {
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
	_ = h.db.QueryRow(`INSERT INTO ua_strikes(newapi_user_id, count, last_ua, last_strike_at)
		VALUES(?, 1, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(newapi_user_id) DO UPDATE SET
			count=ua_strikes.count+1,
			last_ua=excluded.last_ua,
			last_strike_at=excluded.last_strike_at
		RETURNING count`, userID, ua).Scan(&count)
	return count
}

func (h *Handler) banUser(ctx context.Context, userID int64, reason, ua, ip string) error {
	adminToken := h.settings.GetString("newapi_admin_token")
	if adminToken != "" {
		if err := h.newapi.UpdateUserStatus(ctx, adminToken, userID, 2); err != nil && !newapi.IsNotFound(err) {
			return err
		}
	}
	var discordID sql.NullString
	_ = h.db.QueryRow(`SELECT discord_id FROM users WHERE newapi_user_id=?`, userID).Scan(&discordID)

	duration := h.settings.GetString("ua_auto_ban_duration")
	if duration == "" {
		duration = "permanent"
	}
	duration, expireAt := guardban.ExpireAtForDuration(duration, time.Now())

	_, err := h.db.Exec(`INSERT INTO bans(newapi_user_id, discord_id, reason, violation_ua, client_ip, duration, expire_at, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, userID, guardban.NullableString(discordID), reason, ua, ip, duration, expireAt)
	if err == nil {
		h.bumpDailyStat("new_bans")
	}
	return err
}

func (h *Handler) bumpDailyStat(field string) {
	today := time.Now().Format("2006-01-02")
	_, _ = h.db.Exec(`INSERT INTO daily_stats(date) VALUES(?) ON CONFLICT(date) DO NOTHING`, today)
	var stmt string
	switch field {
	case "total_requests":
		stmt = `UPDATE daily_stats SET total_requests = total_requests + 1 WHERE date=?`
	case "blocked_ua":
		stmt = `UPDATE daily_stats SET blocked_ua = blocked_ua + 1 WHERE date=?`
	case "blocked_rpm":
		stmt = `UPDATE daily_stats SET blocked_rpm = blocked_rpm + 1 WHERE date=?`
	case "checkins":
		stmt = `UPDATE daily_stats SET checkins = checkins + 1 WHERE date=?`
	case "new_users":
		stmt = `UPDATE daily_stats SET new_users = new_users + 1 WHERE date=?`
	case "new_bans":
		stmt = `UPDATE daily_stats SET new_bans = new_bans + 1 WHERE date=?`
	default:
		return
	}
	_, _ = h.db.Exec(stmt, today)
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

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
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
