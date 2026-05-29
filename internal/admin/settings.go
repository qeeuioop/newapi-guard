package admin

import (
	"encoding/json"
	"fmt"
	"log"
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
	"newapi_base_url":   true, "newapi_admin_token": true, "newapi_admin_user_id": true,
	"public_base_url": true, "allowed_origins": true, "admin_password": true,
	"oauth_client_id": true, "oauth_client_secret": true, "oauth_allowed_redirect_uris": true, "oauth_provider_slug": true,
	"oauth_state_ttl_seconds": true, "oauth_code_ttl_seconds": true, "oauth_token_ttl_seconds": true,
	"discord_client_id": true, "discord_client_secret": true, "discord_guild_id": true,
	"discord_oauth_scopes": true, "discord_access_policy": true,
	"ua_auto_ban_duration": true, "prompt_cache_enabled": true,
	"prompt_cache_debug": true,
}

func maskSecret(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}

func (h *Handler) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"rpm_limit":                   h.settings.GetInt("rpm_limit", 3),
		"ua_ban_strikes":              h.settings.GetInt("ua_ban_strikes", 3),
		"allowed_ua":                  h.settings.GetStringSlice("allowed_ua"),
		"checkin_threshold":           h.settings.GetInt("checkin_threshold", 0),
		"newapi_base_url":             h.settings.GetString("newapi_base_url"),
		"newapi_admin_token":          maskSecret(h.settings.GetString("newapi_admin_token")),
		"newapi_admin_user_id":        h.settings.GetString("newapi_admin_user_id"),
		"public_base_url":             h.settings.GetString("public_base_url"),
		"allowed_origins":             h.settings.GetStringSlice("allowed_origins"),
		"admin_password":              maskSecret(h.settings.GetString("admin_password")),
		"oauth_client_id":             h.settings.GetString("oauth_client_id"),
		"oauth_client_secret":         maskSecret(h.settings.GetString("oauth_client_secret")),
		"oauth_allowed_redirect_uris": h.settings.GetStringSlice("oauth_allowed_redirect_uris"),
		"oauth_provider_slug":         h.settings.GetString("oauth_provider_slug"),
		"oauth_state_ttl_seconds":     h.settings.GetInt("oauth_state_ttl_seconds", 300),
		"oauth_code_ttl_seconds":      h.settings.GetInt("oauth_code_ttl_seconds", 120),
		"oauth_token_ttl_seconds":     h.settings.GetInt("oauth_token_ttl_seconds", 600),
		"discord_client_id":           h.settings.GetString("discord_client_id"),
		"discord_client_secret":       maskSecret(h.settings.GetString("discord_client_secret")),
		"discord_guild_id":            h.settings.GetString("discord_guild_id"),
		"discord_oauth_scopes":        h.settings.GetStringSlice("discord_oauth_scopes"),
		"discord_access_policy":       h.settings.GetString("discord_access_policy"),
		"ua_auto_ban_duration":        h.settings.GetString("ua_auto_ban_duration"),
		"prompt_cache_enabled":        h.settings.GetBool("prompt_cache_enabled", true),
		"prompt_cache_debug":          h.settings.GetBool("prompt_cache_debug", false),
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}

func validateSettings(updates map[string]string) error {
	intRules := map[string]struct {
		min int
		msg string
	}{
		"rpm_limit":               {min: 1, msg: "用户级 RPM 限制必须大于 0"},
		"ua_ban_strikes":          {min: 1, msg: "UA 违规封禁阈值必须大于 0"},
		"checkin_threshold":       {min: 0, msg: "签到限额不能小于 0"},
		"oauth_state_ttl_seconds": {min: 60, msg: "State TTL 不能小于 60 秒"},
		"oauth_code_ttl_seconds":  {min: 60, msg: "Code TTL 不能小于 60 秒"},
		"oauth_token_ttl_seconds": {min: 60, msg: "Token TTL 不能小于 60 秒"},
	}
	for key, rule := range intRules {
		if raw, ok := updates[key]; ok {
			var value int
			if err := json.Unmarshal([]byte(raw), &value); err != nil {
				if _, scanErr := fmt.Sscanf(raw, "%d", &value); scanErr != nil {
					return fmt.Errorf("%s 格式无效", key)
				}
			}
			if value < rule.min {
				return fmt.Errorf("%s", rule.msg)
			}
		}
	}

	for _, key := range []string{"allowed_ua", "allowed_origins", "oauth_allowed_redirect_uris", "discord_oauth_scopes"} {
		if raw, ok := updates[key]; ok {
			var items []string
			if err := json.Unmarshal([]byte(raw), &items); err != nil {
				return fmt.Errorf("%s 必须是字符串数组", key)
			}
		}
	}
	if raw, ok := updates["discord_access_policy"]; ok {
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return fmt.Errorf("discord_access_policy 必须是有效 JSON")
		}
	}
	if raw, ok := updates["ua_auto_ban_duration"]; ok {
		switch raw {
		case "permanent", "7d", "30d":
		default:
			return fmt.Errorf("UA 自动封禁时长无效")
		}
	}
	return nil
}

func (h *Handler) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := webutil.ReadJSON(r, &payload); err != nil {
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
	if err := validateSettings(updates); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if pwd, ok := updates["admin_password"]; ok {
		hashed, err := webutil.HashPassword(pwd)
		if err != nil {
			log.Printf("[settings] 密码哈希失败: %v", err)
			webutil.WriteError(w, http.StatusInternalServerError, "密码处理失败")
			return
		}
		updates["admin_password"] = hashed
	}
	if len(updates) == 0 {
		webutil.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}
	if err := h.settings.Update(updates); err != nil {
		log.Printf("[admin] 保存设置失败: %v", err)
		webutil.WriteError(w, http.StatusInternalServerError, "保存设置失败")
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
