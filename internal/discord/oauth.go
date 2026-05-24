package discord

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"newapiguard/internal/webutil"
)

const (
	discordAuthorizeURL = "https://discord.com/oauth2/authorize"
	discordTokenURL     = "https://discord.com/api/v10/oauth2/token"
	discordMeURL        = "https://discord.com/api/v10/users/@me"
)

type discordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name"`
	Discriminator string `json:"discriminator"`
}

type discordGuildMember struct {
	GuildID string
	User    discordUser `json:"user"`
	Roles   []string    `json:"roles"`
}

type oauthPayload struct {
	Sub           string `json:"sub"`
	PreferredName string `json:"preferred_username"`
	Name          string `json:"name"`
	Email         string `json:"email"`
}

type accessPolicy struct {
	Logic      string         `json:"logic"`
	Conditions []policyItem   `json:"conditions"`
	Groups     []accessPolicy `json:"groups"`
}

type policyItem struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

func (h *Handler) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	responseType := r.URL.Query().Get("response_type")
	state := r.URL.Query().Get("state")
	scope := r.URL.Query().Get("scope")

	if responseType != "code" || clientID == "" || redirectURI == "" || state == "" {
		webutil.WriteError(w, http.StatusBadRequest, "参数无效")
		return
	}
	if clientID != h.settings.GetString("oauth_client_id") {
		webutil.WriteError(w, http.StatusUnauthorized, "client_id 不匹配")
		return
	}
	if h.settings.GetString("discord_client_id") == "" || h.settings.GetString("discord_client_secret") == "" {
		webutil.WriteError(w, http.StatusServiceUnavailable, "未配置 Discord 凭据")
		return
	}

	pendingState := webutil.RandomToken(24)
	if _, err := h.db.Exec(`INSERT INTO oauth_pending_states(state, client_id, redirect_uri, original_state, scope, expire_at)
		VALUES(?, ?, ?, ?, ?, datetime('now', ?))`,
		pendingState, clientID, redirectURI, state, scope, fmt.Sprintf("+%d seconds", h.settings.GetInt("oauth_state_ttl_seconds", 300))); err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	callback := h.publicBaseURL(r) + "/guard/oauth/callback/discord"
	values := url.Values{}
	values.Set("client_id", h.settings.GetString("discord_client_id"))
	values.Set("redirect_uri", callback)
	values.Set("response_type", "code")
	values.Set("scope", strings.Join(h.discordScopes(), " "))
	values.Set("state", pendingState)

	http.Redirect(w, r, discordAuthorizeURL+"?"+values.Encode(), http.StatusFound)
}

func (h *Handler) handleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		state := r.URL.Query().Get("state")
		var redirectURI, originalState string
		_ = h.db.QueryRow(`SELECT redirect_uri, original_state FROM oauth_pending_states WHERE state=?`, state).Scan(&redirectURI, &originalState)
		h.redirectError(w, r, redirectURI, originalState, errMsg, r.URL.Query().Get("error_description"))
		return
	}

	discordCode := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if discordCode == "" || state == "" {
		webutil.WriteError(w, http.StatusBadRequest, "参数无效")
		return
	}

	var redirectURI, originalState string
	if err := h.db.QueryRow(`SELECT redirect_uri, original_state FROM oauth_pending_states WHERE state=? AND expire_at > CURRENT_TIMESTAMP`, state).
		Scan(&redirectURI, &originalState); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "state 无效或已过期")
		return
	}

	token, err := h.exchangeDiscordToken(r, discordCode)
	if err != nil {
		h.redirectError(w, r, redirectURI, originalState, "access_denied", err.Error())
		return
	}
	upstreamUser, err := h.fetchDiscordUser(r, token)
	if err != nil {
		h.redirectError(w, r, redirectURI, originalState, "access_denied", err.Error())
		return
	}
	member, err := h.fetchDiscordGuildMember(r, token)
	if err != nil {
		h.redirectError(w, r, redirectURI, originalState, "access_denied", err.Error())
		return
	}
	if ok, reason := h.isAllowed(member); !ok {
		h.redirectError(w, r, redirectURI, originalState, "access_denied", reason)
		return
	}

	payload := oauthPayload{
		Sub:           "discord:" + upstreamUser.ID,
		PreferredName: "dc_" + upstreamUser.ID,
		Name:          displayName(upstreamUser),
		Email:         "",
	}
	code := webutil.RandomToken(24)
	codeTTL := time.Duration(h.settings.GetInt("oauth_code_ttl_seconds", 120)) * time.Second
	rawPayload, _ := json.Marshal(payload)
	_, err = h.db.Exec(`INSERT INTO oauth_authorization_codes(code, client_id, redirect_uri, discord_id, discord_name, payload, expire_at)
		VALUES(?, ?, ?, ?, ?, ?, datetime('now', ?))`,
		code, h.settings.GetString("oauth_client_id"), redirectURI, upstreamUser.ID, payload.Name, string(rawPayload), fmt.Sprintf("+%d seconds", int(codeTTL.Seconds())))
	if err != nil {
		h.redirectError(w, r, redirectURI, originalState, "server_error", err.Error())
		return
	}
	_, _ = h.db.Exec(`DELETE FROM oauth_pending_states WHERE state=?`, state)

	values := url.Values{}
	values.Set("code", code)
	values.Set("state", originalState)
	http.Redirect(w, r, redirectURI+"?"+values.Encode(), http.StatusFound)
}

func (h *Handler) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		webutil.WriteError(w, http.StatusBadRequest, "grant_type 无效")
		return
	}
	if r.PostForm.Get("client_id") != h.settings.GetString("oauth_client_id") || r.PostForm.Get("client_secret") != h.settings.GetString("oauth_client_secret") {
		webutil.WriteError(w, http.StatusUnauthorized, "客户端凭据无效")
		return
	}
	code := r.PostForm.Get("code")
	if code == "" {
		webutil.WriteError(w, http.StatusBadRequest, "code 不能为空")
		return
	}

	var payloadStr, storedClientID, storedRedirectURI string
	if err := h.db.QueryRow(`SELECT payload, client_id, redirect_uri FROM oauth_authorization_codes WHERE code=? AND used_at IS NULL AND expire_at > CURRENT_TIMESTAMP`, code).
		Scan(&payloadStr, &storedClientID, &storedRedirectURI); err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "code 无效或已过期")
		return
	}
	if storedClientID != r.PostForm.Get("client_id") || storedRedirectURI != r.PostForm.Get("redirect_uri") {
		webutil.WriteError(w, http.StatusBadRequest, "redirect_uri 或 client_id 不匹配")
		return
	}
	accessToken := webutil.RandomToken(24)
	tokenTTL := h.settings.GetInt("oauth_token_ttl_seconds", 600)
	_, err := h.db.Exec(`INSERT INTO oauth_access_tokens(access_token, payload, expire_at) VALUES(?, ?, datetime('now', ?))`,
		accessToken, payloadStr, fmt.Sprintf("+%d seconds", tokenTTL))
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, _ = h.db.Exec(`UPDATE oauth_authorization_codes SET used_at=CURRENT_TIMESTAMP WHERE code=?`, code)
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   tokenTTL,
		"scope":        "openid profile email",
	})
}

func (h *Handler) handleUserinfo(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		webutil.WriteError(w, http.StatusUnauthorized, "缺少访问令牌")
		return
	}
	var payloadStr string
	if err := h.db.QueryRow(`SELECT payload FROM oauth_access_tokens WHERE access_token=? AND expire_at > CURRENT_TIMESTAMP`, token).Scan(&payloadStr); err != nil {
		webutil.WriteError(w, http.StatusUnauthorized, "访问令牌无效")
		return
	}
	var payload oauthPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"sub":                payload.Sub,
		"preferred_username": payload.PreferredName,
		"name":               payload.Name,
		"email":              payload.Email,
	})
}

func (h *Handler) exchangeDiscordToken(r *http.Request, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", h.settings.GetString("discord_client_id"))
	form.Set("client_secret", h.settings.GetString("discord_client_secret"))
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", h.publicBaseURL(r)+"/guard/oauth/callback/discord")

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, discordTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Discord token 交换失败: %s", strings.TrimSpace(string(body)))
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("Discord 未返回 access_token")
	}
	return payload.AccessToken, nil
}

func (h *Handler) fetchDiscordUser(r *http.Request, accessToken string) (*discordUser, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, discordMeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Discord 用户资料获取失败")
	}
	var user discordUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *Handler) fetchDiscordGuildMember(r *http.Request, accessToken string) (*discordGuildMember, error) {
	guildID := h.settings.GetString("discord_guild_id")
	if guildID == "" {
		return nil, fmt.Errorf("未配置 Discord 服务器 ID")
	}
	endpoint := fmt.Sprintf("https://discord.com/api/v10/users/@me/guilds/%s/member", guildID)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Discord 身份组资料获取失败")
	}
	var member discordGuildMember
	if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
		return nil, err
	}
	member.GuildID = guildID
	return &member, nil
}

func (h *Handler) isAllowed(member *discordGuildMember) (bool, string) {
	var policy accessPolicy
	if err := h.settings.GetJSON("discord_access_policy", &policy); err != nil {
		return false, "准入规则格式无效"
	}
	if len(policy.Conditions) == 0 && len(policy.Groups) == 0 {
		return true, ""
	}
	if evaluatePolicy(policy, member) {
		return true, ""
	}
	return false, "不满足准入规则"
}

func evaluatePolicy(policy accessPolicy, member *discordGuildMember) bool {
	results := []bool{}
	for _, cond := range policy.Conditions {
		results = append(results, evaluateCondition(cond, member))
	}
	for _, group := range policy.Groups {
		results = append(results, evaluatePolicy(group, member))
	}
	if strings.ToLower(policy.Logic) == "or" {
		for _, result := range results {
			if result {
				return true
			}
		}
		return len(results) == 0
	}
	for _, result := range results {
		if !result {
			return false
		}
	}
	return true
}

func evaluateCondition(cond policyItem, member *discordGuildMember) bool {
	switch strings.ToLower(cond.Field) {
	case "guild_id":
		return cond.Op == "eq" && cond.Value == member.GuildID
	case "roles":
		if cond.Op != "contains" {
			return false
		}
		for _, role := range member.Roles {
			if role == cond.Value {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (h *Handler) discordScopes() []string {
	scopes := h.settings.GetStringSlice("discord_oauth_scopes")
	if len(scopes) == 0 {
		return []string{"identify", "guilds.members.read"}
	}
	return scopes
}

func (h *Handler) publicBaseURL(r *http.Request) string {
	if v := strings.TrimSpace(h.settings.GetString("public_base_url")); v != "" {
		return strings.TrimRight(v, "/")
	}
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}
	host := r.Host
	return scheme + "://" + strings.TrimRight(host, "/")
}

func (h *Handler) redirectError(w http.ResponseWriter, r *http.Request, redirectURI, originalState, code, description string) {
	if redirectURI == "" {
		webutil.WriteError(w, http.StatusForbidden, description)
		return
	}
	values := url.Values{}
	values.Set("error", code)
	if description != "" {
		values.Set("error_description", description)
	}
	values.Set("state", originalState)
	http.Redirect(w, r, redirectURI+"?"+values.Encode(), http.StatusFound)
}

func displayName(user *discordUser) string {
	if user.GlobalName != "" {
		return user.GlobalName
	}
	if user.Username != "" {
		if user.Discriminator != "" && user.Discriminator != "0" {
			return user.Username + "#" + user.Discriminator
		}
		return user.Username
	}
	return user.ID
}
