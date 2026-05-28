package discord

import (
	"database/sql"
	"net/http"

	"newapiguard/internal/config"
	"newapiguard/internal/newapi"
	"newapiguard/internal/settings"
)

type Handler struct {
	env      config.Env
	db       *sql.DB
	settings *settings.Store
	newapi   *newapi.Client
	tokens   *newapi.TokenResolver
}

func NewHandler(env config.Env, db *sql.DB, settingsStore *settings.Store, newapiClient *newapi.Client, tokenResolver *newapi.TokenResolver) *Handler {
	return &Handler{
		env:      env,
		db:       db,
		settings: settingsStore,
		newapi:   newapiClient,
		tokens:   tokenResolver,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/guard/oauth/authorize":
		if r.URL.Query().Get("state") != "" && r.URL.Query().Get("code") != "" && r.URL.Query().Get("client_id") == "" {
			h.handleDiscordCallback(w, r)
			return
		}
		h.handleAuthorize(w, r)
	case "/guard/oauth/callback/discord":
		h.handleDiscordCallback(w, r)
	case "/guard/oauth/token":
		h.handleToken(w, r)
	case "/guard/oauth/userinfo":
		h.handleUserinfo(w, r)
	default:
		http.NotFound(w, r)
	}
}
