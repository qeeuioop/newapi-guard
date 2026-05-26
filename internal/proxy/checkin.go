package proxy

import (
	"database/sql"
	"fmt"
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
			target, _ := url.Parse(strings.TrimRight(newapiClient.BaseURL(), "/"))
			r.URL.Scheme = target.Scheme
			r.URL.Host = target.Host
			r.Host = target.Host
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
			webutil.WriteError(w, http.StatusUnauthorized, "无法识别当前用户")
			return
		}
		if user.Quota >= threshold {
			dollars := float64(threshold) / 500000.0
			webutil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("余额超过 $%.2f，无需签到", dollars))
			return
		}
	}

	h.passthru.ServeHTTP(w, r)
}

