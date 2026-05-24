package discord

import (
	"database/sql"
	"net/http"

	"newapiguard/internal/config"
	"newapiguard/internal/newapi"
	"newapiguard/internal/settings"
	"newapiguard/internal/webutil"
)

type Handler struct {
	env      config.Env
	db       *sql.DB
	settings *settings.Store
	newapi   *newapi.Client
}

func NewHandler(env config.Env, db *sql.DB, settingsStore *settings.Store, newapiClient *newapi.Client) *Handler {
	return &Handler{
		env:      env,
		db:       db,
		settings: settingsStore,
		newapi:   newapiClient,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/guard/oauth/authorize":
		webutil.WriteError(w, http.StatusNotImplemented, "Discord OAuth 尚未接入完成")
	case "/guard/oauth/token":
		webutil.WriteError(w, http.StatusNotImplemented, "Discord OAuth 尚未接入完成")
	case "/guard/oauth/userinfo":
		webutil.WriteError(w, http.StatusNotImplemented, "Discord OAuth 尚未接入完成")
	default:
		http.NotFound(w, r)
	}
}
