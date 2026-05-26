package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"newapiguard/internal/webutil"
)

var sensitiveKeys = map[string]bool{
	"admin_password":        true,
	"newapi_admin_token":    true,
	"discord_client_secret": true,
	"oauth_client_secret":   true,
}

var allowedSettingKeys = map[string]bool{
	"rpm_limit": true, "ua_ban_strikes": true, "allowed_ua": true,
	"checkin_threshold": true,
	"newapi_base_url": true, "newapi_admin_token": true, "newapi_admin_user_id": true,
	"public_base_url": true, "admin_password": true,
	"oauth_client_id": true, "oauth_client_secret": true, "oauth_provider_slug": true,
	"oauth_state_ttl_seconds": true, "oauth_code_ttl_seconds": true, "oauth_token_ttl_seconds": true,
	"discord_client_id": true, "discord_client_secret": true, "discord_guild_id": true,
	"discord_oauth_scopes": true, "discord_access_policy": true,
	"ua_auto_ban_duration": true,
}

func maskSecret(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}

func (h *Handler) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"rpm_limit":               h.settings.GetInt("rpm_limit", 3),
		"ua_ban_strikes":          h.settings.GetInt("ua_ban_strikes", 3),
		"allowed_ua":              h.settings.GetStringSlice("allowed_ua"),
		"checkin_threshold":       h.settings.GetInt("checkin_threshold", 0),
		"newapi_base_url":         h.settings.GetString("newapi_base_url"),
		"newapi_admin_token":      maskSecret(h.settings.GetString("newapi_admin_token")),
		"newapi_admin_user_id":    h.settings.GetString("newapi_admin_user_id"),
		"public_base_url":         h.settings.GetString("public_base_url"),
		"admin_password":          maskSecret(h.settings.GetString("admin_password")),
		"oauth_client_id":         h.settings.GetString("oauth_client_id"),
		"oauth_client_secret":     maskSecret(h.settings.GetString("oauth_client_secret")),
		"oauth_provider_slug":     h.settings.GetString("oauth_provider_slug"),
		"oauth_state_ttl_seconds": h.settings.GetInt("oauth_state_ttl_seconds", 300),
		"oauth_code_ttl_seconds":  h.settings.GetInt("oauth_code_ttl_seconds", 120),
		"oauth_token_ttl_seconds": h.settings.GetInt("oauth_token_ttl_seconds", 600),
		"discord_client_id":       h.settings.GetString("discord_client_id"),
		"discord_client_secret":   maskSecret(h.settings.GetString("discord_client_secret")),
		"discord_guild_id":        h.settings.GetString("discord_guild_id"),
		"discord_oauth_scopes":    h.settings.GetStringSlice("discord_oauth_scopes"),
		"discord_access_policy":   h.settings.GetString("discord_access_policy"),
		"ua_auto_ban_duration":    h.settings.GetString("ua_auto_ban_duration"),
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}

func (h *Handler) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	updates := map[string]string{}
	for key, value := range payload {
		if !allowedSettingKeys[key] {
			continue
		}
		switch v := value.(type) {
		case string:
			if sensitiveKeys[key] && (v == "" || strings.HasPrefix(v, "***")) {
				continue
			}
			updates[key] = v
		default:
			data, _ := json.Marshal(v)
			updates[key] = string(data)
		}
	}
	if len(updates) == 0 {
		webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}
	if err := h.settings.Update(updates); err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if newAPIURL, ok := updates["newapi_base_url"]; ok && strings.TrimSpace(newAPIURL) != "" {
		h.newapi.SetBaseURL(newAPIURL)
	}
	if newAPIAdminUserID, ok := updates["newapi_admin_user_id"]; ok {
		h.newapi.SetAdminUserID(newAPIAdminUserID)
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}
