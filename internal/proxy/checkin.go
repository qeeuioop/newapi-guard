package proxy

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"newapiguard/internal/cache"
	"newapiguard/internal/config"
	"newapiguard/internal/newapi"
	"newapiguard/internal/settings"
	"newapiguard/internal/webutil"
)

type CheckinHandler struct {
	env      config.Env
	db       *sql.DB
	settings *settings.Store
	cache    *cache.Store
	newapi   *newapi.Client
	proxy    *Handler
}

func NewCheckinHandler(env config.Env, db *sql.DB, settingsStore *settings.Store, cacheStore *cache.Store, newapiClient *newapi.Client) *CheckinHandler {
	return &CheckinHandler{
		env:      env,
		db:       db,
		settings: settingsStore,
		cache:    cacheStore,
		newapi:   newapiClient,
		proxy:    NewHandler(env, db, settingsStore, cacheStore, newapiClient),
	}
}

func (h *CheckinHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.proxy.proxy.ServeHTTP(w, r)
		return
	}

	user, err := h.newapi.GetUserSelf(r.Context(), r.Header)
	if err != nil {
		webutil.WriteError(w, http.StatusUnauthorized, "无法识别当前用户")
		return
	}

	today := time.Now().Format("2006-01-02")
	var exists int
	_ = h.db.QueryRow(`SELECT 1 FROM checkin_records WHERE newapi_user_id=? AND checked_at=?`, user.UserID, today).Scan(&exists)
	if exists == 1 {
		webutil.WriteError(w, http.StatusBadRequest, "今日已签到")
		return
	}

	threshold := h.settings.GetInt("checkin_threshold", 200000)
	if user.Quota >= threshold {
		webutil.WriteError(w, http.StatusBadRequest, "余额充足，无需签到")
		return
	}

	quota := h.settings.GetInt("checkin_quota", 500000)
	adminToken := h.settings.GetString("newapi_admin_token")
	if adminToken == "" {
		webutil.WriteError(w, http.StatusServiceUnavailable, "未配置 New API 管理员令牌")
		return
	}
	if err := h.newapi.TopupUser(context.Background(), adminToken, user.UserID, quota); err != nil {
		webutil.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}

	if _, err := h.db.Exec(`INSERT INTO checkin_records(newapi_user_id, quota_added, checked_at) VALUES(?, ?, ?)`, user.UserID, quota, today); err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.bumpDailyStat("checkins")
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "签到成功",
		"data": map[string]any{
			"quota_awarded": quota,
			"checkin_date":  today,
		},
	})
}

func (h *CheckinHandler) bumpDailyStat(field string) {
	today := time.Now().Format("2006-01-02")
	_, _ = h.db.Exec(`INSERT INTO daily_stats(date) VALUES(?) ON CONFLICT(date) DO NOTHING`, today)
	_, _ = h.db.Exec(`UPDATE daily_stats SET `+field+` = `+field+` + 1 WHERE date=?`, today)
}
