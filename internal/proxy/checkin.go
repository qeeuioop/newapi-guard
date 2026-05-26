package proxy

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

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
	passthru *httputil.ReverseProxy
}

func NewCheckinHandler(env config.Env, db *sql.DB, settingsStore *settings.Store, cacheStore *cache.Store, newapiClient *newapi.Client) *CheckinHandler {
	rp := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			target, err := url.Parse(strings.TrimRight(newapiClient.BaseURL(), "/"))
			if err != nil || target == nil || target.Host == "" {
				log.Printf("[checkin] newapi_base_url 解析失败: %v", err)
				r.URL.Scheme = ""
				r.URL.Host = ""
				r.Host = ""
				return
			}
			r.URL.Scheme = target.Scheme
			r.URL.Host = target.Host
			r.Host = target.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			webutil.WriteError(w, http.StatusBadGateway, "上游服务不可用")
		},
	}
	return &CheckinHandler{
		env:      env,
		db:       db,
		settings: settingsStore,
		cache:    cacheStore,
		newapi:   newapiClient,
		passthru: rp,
	}
}

func (h *CheckinHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.passthru.ServeHTTP(w, r)
		return
	}

	threshold := h.settings.GetInt("checkin_threshold", 0)
	if threshold > 0 {
		user, err := h.newapi.GetUserSelf(r.Context(), r.Header)
		if err != nil {
			log.Printf("[checkin] GetUserSelf failed: %v", err)
			webutil.WriteError(w, http.StatusUnauthorized, "无法识别当前用户")
			return
		}

		adminToken := h.settings.GetString("newapi_admin_token")
		fullUser, err := h.newapi.GetUser(r.Context(), adminToken, user.UserID)
		if err != nil {
			log.Printf("[checkin] GetUser(%d) failed: %v", user.UserID, err)
			webutil.WriteError(w, http.StatusInternalServerError, "查询余额失败")
			return
		}

		log.Printf("[checkin] user=%d quota=%d threshold=%d blocked=%v", fullUser.ID, fullUser.Quota, threshold, fullUser.Quota >= threshold)
		if fullUser.Quota >= threshold {
			dollars := float64(threshold) / 500000.0
			webutil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("余额超过 $%.2f，无需签到", dollars))
			return
		}
	}

	h.passthru.ServeHTTP(w, r)
}
