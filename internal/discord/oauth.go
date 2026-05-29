package discord

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
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

var discordHTTPClient = &http.Client{Timeout: 15 * time.Second}

type discordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name"`
	Discriminator string `json:"discriminator"`
}

type discordGuildMember struct {
	GuildID string
	Nick    string      `json:"nick"`
	User    discordUser `json:"user"`
	Roles   []string    `json:"roles"`
}

type oauthPayload struct {
	Sub           string `json:"sub"`
	PreferredName string `json:"preferred_username"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
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
	if !h.allowedRedirectURI(redirectURI) {
		webutil.WriteError(w, http.StatusBadRequest, "redirect_uri 不在允许列表中")
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
		log.Printf("[oauth] 保存登录状态失败: %v", err)
		webutil.WriteError(w, http.StatusInternalServerError, "内部服务错误")
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
		_, _ = h.db.Exec(`DELETE FROM oauth_pending_states WHERE state=?`, state)
		h.redirectError(w, r, redirectURI, originalState, errMsg, r.URL.Query().Get("error_description"))
		return
	}

	discordCode := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if discordCode == "" || state == "" {
		h.renderCallbackResult(w, r, callbackResultPage{
			Title:       "登录失败",
			Message:     "登录参数无效，无法继续。",
			Description: "缺少 code 或 state 参数。",
			RedirectURL: h.publicBaseURL(r),
			Success:     false,
		})
		return
	}

	var redirectURI, originalState string
	if err := h.db.QueryRow(`SELECT redirect_uri, original_state FROM oauth_pending_states WHERE state=? AND expire_at > CURRENT_TIMESTAMP`, state).
		Scan(&redirectURI, &originalState); err != nil {
		h.renderCallbackResult(w, r, callbackResultPage{
			Title:       "登录失败",
			Message:     "登录状态已失效，请重新发起登录。",
			Description: "state 无效或已过期。",
			RedirectURL: h.publicBaseURL(r),
			Success:     false,
		})
		return
	}
	_, _ = h.db.Exec(`DELETE FROM oauth_pending_states WHERE state=?`, state)

	token, err := h.exchangeDiscordToken(r, discordCode)
	if err != nil {
		log.Printf("[oauth] Discord token 交换失败: %v", err)
		h.redirectError(w, r, redirectURI, originalState, "access_denied", "Discord 登录验证失败")
		return
	}
	upstreamUser, err := h.fetchDiscordUser(r, token)
	if err != nil {
		log.Printf("[oauth] Discord 用户信息获取失败: %v", err)
		h.redirectError(w, r, redirectURI, originalState, "access_denied", "Discord 登录验证失败")
		return
	}
	member, err := h.fetchDiscordGuildMember(r, token)
	if err != nil {
		log.Printf("[oauth] Discord 身份组信息获取失败: %v", err)
		h.redirectError(w, r, redirectURI, originalState, "access_denied", "Discord 登录验证失败")
		return
	}
	if ok, reason := h.isAllowed(member); !ok {
		h.redirectError(w, r, redirectURI, originalState, "access_denied", reason)
		return
	}

	if h.tokens != nil {
		var existingUserID sql.NullInt64
		_ = h.db.QueryRow(`SELECT newapi_user_id FROM oauth_identity_links WHERE discord_id=?`, upstreamUser.ID).Scan(&existingUserID)
		if !existingUserID.Valid || existingUserID.Int64 <= 0 {
			_ = h.db.QueryRow(`SELECT newapi_user_id FROM users WHERE discord_id=?`, upstreamUser.ID).Scan(&existingUserID)
		}
		if existingUserID.Valid && existingUserID.Int64 > 0 {
			deleted, err := h.tokens.IsUserDeleted(r.Context(), existingUserID.Int64)
			if err != nil {
				log.Printf("[oauth] 检查用户删除状态失败: %v (discord_id=%s, user_id=%d)", err, upstreamUser.ID, existingUserID.Int64)
			}
			if deleted {
				log.Printf("[oauth] 阻止已删号用户重新注册: discord_id=%s, old_user_id=%d", upstreamUser.ID, existingUserID.Int64)
				h.redirectError(w, r, redirectURI, originalState, "access_denied", "账号已注销，无法重新注册。如需恢复请联系管理员。")
				return
			}
		}
	}

	displayName := displayName(member, upstreamUser)
	payload := oauthPayload{
		Sub:           "discord:" + upstreamUser.ID,
		PreferredName: oauthUsername(upstreamUser),
		Name:          displayName,
		DisplayName:   displayName,
		Email:         "",
	}
	_, _ = h.db.Exec(`INSERT INTO oauth_identity_links(discord_id, discord_name, preferred_username, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(discord_id) DO UPDATE SET
			discord_name=excluded.discord_name,
			preferred_username=excluded.preferred_username,
			updated_at=CURRENT_TIMESTAMP`, upstreamUser.ID, payload.DisplayName, payload.PreferredName)
	code := webutil.RandomToken(24)
	codeTTL := time.Duration(h.settings.GetInt("oauth_code_ttl_seconds", 120)) * time.Second
	rawPayload, _ := json.Marshal(payload)
	_, err = h.db.Exec(`INSERT INTO oauth_authorization_codes(code, client_id, redirect_uri, discord_id, discord_name, payload, expire_at)
		VALUES(?, ?, ?, ?, ?, ?, datetime('now', ?))`,
		code, h.settings.GetString("oauth_client_id"), redirectURI, upstreamUser.ID, payload.Name, string(rawPayload), fmt.Sprintf("+%d seconds", int(codeTTL.Seconds())))
	if err != nil {
		log.Printf("[oauth] 保存授权码失败: %v", err)
		h.redirectError(w, r, redirectURI, originalState, "server_error", "内部服务错误")
		return
	}
	values := url.Values{}
	values.Set("code", code)
	values.Set("state", originalState)
	h.renderCallbackResult(w, r, callbackResultPage{
		Title:       "登录成功",
		Message:     "身份组验证成功，登录成功",
		Description: "3 秒后将继续跳转并完成 NewAPI 登录。",
		RedirectURL: redirectURI + "?" + values.Encode(),
		Success:     true,
	})
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
	configuredClientID := h.settings.GetString("oauth_client_id")
	storedSecret := h.settings.GetString("oauth_client_secret")
	reqClientID := r.PostForm.Get("client_id")
	reqSecret := r.PostForm.Get("client_secret")
	if reqClientID == "" || configuredClientID == "" || reqClientID != configuredClientID || reqSecret == "" || storedSecret == "" || !webutil.ConstantTimeEqual(reqSecret, storedSecret) {
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
		log.Printf("[oauth] 保存访问令牌失败: %v", err)
		webutil.WriteError(w, http.StatusInternalServerError, "内部服务错误")
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
		log.Printf("[oauth] 用户信息解析失败: %v", err)
		webutil.WriteError(w, http.StatusInternalServerError, "内部服务错误")
		return
	}
	webutil.WriteJSON(w, http.StatusOK, map[string]any{
		"sub":                payload.Sub,
		"preferred_username": payload.PreferredName,
		"name":               payload.Name,
		"display_name":       payload.DisplayName,
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

	resp, err := discordHTTPClient.Do(req)
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
	resp, err := discordHTTPClient.Do(req)
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
	resp, err := discordHTTPClient.Do(req)
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
	return false, "无要求身份组，登录失败"
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

func (h *Handler) allowedRedirectURI(redirectURI string) bool {
	if strings.TrimSpace(redirectURI) == "" {
		return false
	}
	allowed := h.settings.GetStringSlice("oauth_allowed_redirect_uris")
	if len(allowed) == 0 {
		publicBaseURL := strings.TrimRight(strings.TrimSpace(h.settings.GetString("public_base_url")), "/")
		providerSlug := strings.Trim(strings.TrimSpace(h.settings.GetString("oauth_provider_slug")), "/")
		if publicBaseURL == "" || providerSlug == "" {
			return false
		}
		allowed = []string{publicBaseURL + "/oauth/" + providerSlug}
	}
	for _, item := range allowed {
		if redirectURI == strings.TrimSpace(item) {
			return true
		}
	}
	return false
}

func (h *Handler) publicBaseURL(r *http.Request) string {
	if v := strings.TrimSpace(h.settings.GetString("public_base_url")); v != "" {
		return strings.TrimRight(v, "/")
	}
	log.Printf("[oauth] 警告: public_base_url 未配置，使用请求头推断（可能被伪造）")
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}
	host := r.Host
	return scheme + "://" + strings.TrimRight(host, "/")
}

func (h *Handler) redirectError(w http.ResponseWriter, r *http.Request, redirectURI, originalState, code, description string) {
	values := url.Values{}
	values.Set("error", code)
	if description != "" {
		values.Set("error_description", description)
	}
	values.Set("state", originalState)
	target := h.publicBaseURL(r)
	if redirectURI != "" {
		target = redirectURI + "?" + values.Encode()
	}
	message := "Discord 登录失败"
	if description == "无要求身份组，登录失败" {
		message = description
	}
	h.renderCallbackResult(w, r, callbackResultPage{
		Title:       "登录失败",
		Message:     message,
		Description: fallbackString(description, "3 秒后将返回登录页。"),
		RedirectURL: target,
		Success:     false,
	})
}

func oauthUsername(user *discordUser) string {
	if strings.TrimSpace(user.Username) != "" {
		return user.Username
	}
	return user.ID
}

func displayName(member *discordGuildMember, user *discordUser) string {
	if member != nil && strings.TrimSpace(member.Nick) != "" {
		return member.Nick
	}
	if user == nil {
		return ""
	}
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

type callbackResultPage struct {
	Title       string
	Message     string
	Description string
	RedirectURL string
	Success     bool
}

func (h *Handler) renderCallbackResult(w http.ResponseWriter, r *http.Request, page callbackResultPage) {
	if strings.TrimSpace(page.RedirectURL) == "" {
		page.RedirectURL = h.publicBaseURL(r)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	const tpl = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>{{ .Title }}</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f4f0e8;
      --panel: rgba(255, 250, 242, 0.92);
      --text: #1f2937;
      --muted: #6b7280;
      --success: #1d6b4f;
      --error: #9f1239;
      --accent: #c26d2d;
      --border: rgba(31, 41, 55, 0.12);
      --shadow: 0 30px 80px rgba(31, 41, 55, 0.16);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 24px;
      font-family: "Noto Sans SC", "Microsoft YaHei", sans-serif;
      color: var(--text);
      background:
        radial-gradient(circle at top left, rgba(194, 109, 45, 0.22), transparent 34%),
        radial-gradient(circle at bottom right, rgba(34, 197, 94, 0.18), transparent 30%),
        linear-gradient(135deg, #fcfaf6 0%, #efe6d7 100%);
    }
    .panel {
      width: min(560px, 100%);
      padding: 36px 30px;
      border-radius: 28px;
      background: var(--panel);
      border: 1px solid var(--border);
      box-shadow: var(--shadow);
      backdrop-filter: blur(14px);
    }
    .badge {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      padding: 8px 14px;
      border-radius: 999px;
      font-size: 13px;
      font-weight: 700;
      letter-spacing: 0.04em;
      background: {{ if .Success }}rgba(29, 107, 79, 0.12){{ else }}rgba(159, 18, 57, 0.12){{ end }};
      color: {{ if .Success }}var(--success){{ else }}var(--error){{ end }};
    }
    h1 {
      margin: 18px 0 12px;
      font-size: clamp(28px, 4vw, 38px);
      line-height: 1.18;
    }
    p {
      margin: 0;
      line-height: 1.75;
      font-size: 15px;
      color: var(--muted);
    }
    .countdown {
      margin-top: 24px;
      padding: 18px 20px;
      border-radius: 18px;
      background: rgba(255,255,255,0.72);
      border: 1px solid rgba(31, 41, 55, 0.08);
      font-size: 15px;
    }
    .countdown strong {
      color: var(--accent);
      font-size: 24px;
      padding: 0 2px;
    }
    .link {
      margin-top: 18px;
      display: inline-block;
      color: var(--text);
    }
  </style>
</head>
<body>
  <main class="panel">
    <div class="badge">{{ if .Success }}验证通过{{ else }}验证失败{{ end }}</div>
    <h1>{{ .Message }}</h1>
    <p>{{ .Description }}</p>
    <div class="countdown">页面将在 <strong id="countdown">3</strong> 秒后自动跳转。</div>
    <a class="link" href="{{ .RedirectURL }}">如果没有自动跳转，请点这里继续</a>
  </main>
  <script>
    const redirectURL = {{ .RedirectURLJSON }};
    const countdownNode = document.getElementById("countdown");
    let seconds = 3;
    const timer = window.setInterval(() => {
      seconds -= 1;
      if (seconds <= 0) {
        window.clearInterval(timer);
        window.location.href = redirectURL;
        return;
      }
      countdownNode.textContent = String(seconds);
    }, 1000);
  </script>
</body>
</html>`

	redirectURLJSON, _ := json.Marshal(page.RedirectURL)
	data := struct {
		Title           string
		Message         string
		Description     string
		RedirectURL     string
		RedirectURLJSON template.JS
		Success         bool
	}{
		Title:           page.Title,
		Message:         page.Message,
		Description:     page.Description,
		RedirectURL:     page.RedirectURL,
		RedirectURLJSON: template.JS(redirectURLJSON),
		Success:         page.Success,
	}

	_ = template.Must(template.New("callback-result").Parse(tpl)).Execute(w, data)
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
