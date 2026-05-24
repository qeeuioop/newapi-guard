package newapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	mu          sync.RWMutex
	baseURL     string
	adminUserID string
	http        *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) SetBaseURL(baseURL string) {
	c.mu.Lock()
	c.baseURL = strings.TrimRight(baseURL, "/")
	c.mu.Unlock()
}

func (c *Client) SetAdminUserID(userID string) {
	c.mu.Lock()
	c.adminUserID = strings.TrimSpace(userID)
	c.mu.Unlock()
}

func (c *Client) BaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL
}

func (c *Client) adminUserIDValue() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.adminUserID
}

type UserSelf struct {
	UserID int64 `json:"user_id"`
	Quota  int   `json:"quota"`
}

type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Status      int    `json:"status"`
	Quota       int    `json:"quota"`
	Role        int    `json:"role"`
	Group       string `json:"group"`
	Email       string `json:"email"`
	CreatedAt   int64  `json:"created_at"`
	LastLoginAt int64  `json:"last_login_at"`
}

type UserPage struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Total    int    `json:"total"`
	Items    []User `json:"items"`
}

func (c *Client) SearchToken(ctx context.Context, adminToken, token string) (int64, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL()+"/api/token/search?keyword="+urlQueryEscape(token), nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	c.setAdminHeaders(req.Header)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, false, nil
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, false, err
	}

	for _, key := range []string{"user_id", "id", "newapi_user_id"} {
		if value, ok := payload[key]; ok {
			if id, ok := toInt64(value); ok {
				return id, true, nil
			}
		}
	}

	if data, ok := payload["data"].(map[string]any); ok {
		for _, key := range []string{"user_id", "id", "newapi_user_id"} {
			if value, ok := data[key]; ok {
				if id, ok := toInt64(value); ok {
					return id, true, nil
				}
			}
		}
	}

	return 0, false, nil
}

func (c *Client) GetUserSelf(ctx context.Context, headers http.Header) (*UserSelf, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL()+"/api/user/self", nil)
	if err != nil {
		return nil, err
	}
	copyInterestingHeaders(req.Header, headers)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("newapi 状态码异常: %d", resp.StatusCode)
	}

	var payload struct {
		Data struct {
			UserID int64 `json:"user_id"`
			Quota  int   `json:"quota"`
		} `json:"data"`
		UserID int64 `json:"user_id"`
		Quota  int   `json:"quota"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	result := &UserSelf{UserID: payload.UserID, Quota: payload.Quota}
	if result.UserID == 0 && payload.Data.UserID != 0 {
		result.UserID = payload.Data.UserID
	}
	if result.Quota == 0 && payload.Data.Quota != 0 {
		result.Quota = payload.Data.Quota
	}
	if result.UserID == 0 {
		return nil, fmt.Errorf("未能识别当前用户")
	}
	return result, nil
}

func (c *Client) TopupUser(ctx context.Context, adminToken string, userID int64, quota int) error {
	body, _ := json.Marshal(map[string]any{
		"user_id": userID,
		"quota":   quota,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL()+"/api/user/topup/complete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	c.setAdminHeaders(req.Header)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("充值失败: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) UpdateUserStatus(ctx context.Context, adminToken string, userID int64, status int) error {
	body, _ := json.Marshal(map[string]any{
		"id":     userID,
		"status": status,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.BaseURL()+"/api/user/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	c.setAdminHeaders(req.Header)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("更新用户状态失败: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) CreateUser(ctx context.Context, adminToken, username, password string) (int64, error) {
	body, _ := json.Marshal(map[string]any{
		"username": username,
		"password": password,
		"role":     1,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL()+"/api/user/", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	c.setAdminHeaders(req.Header)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("创建用户失败: %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}
	if value, ok := payload["user_id"]; ok {
		if id, ok := toInt64(value); ok {
			return id, nil
		}
	}
	if data, ok := payload["data"].(map[string]any); ok {
		if value, ok := data["user_id"]; ok {
			if id, ok := toInt64(value); ok {
				return id, nil
			}
		}
	}
	users, _, err := c.SearchUsers(ctx, adminToken, username, 1, 20)
	if err != nil {
		return 0, err
	}
	for _, user := range users {
		if user.Username == username {
			return user.ID, nil
		}
	}
	return 0, fmt.Errorf("创建成功但未定位到 user_id")
}

func (c *Client) ListUsers(ctx context.Context, adminToken string, page, pageSize int) ([]User, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/user/?p=%d&page_size=%d", c.BaseURL(), page, pageSize), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	c.setAdminHeaders(req.Header)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, 0, fmt.Errorf("获取用户列表失败: %d", resp.StatusCode)
	}

	var payload struct {
		Success bool     `json:"success"`
		Message string   `json:"message"`
		Data    UserPage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, err
	}
	if !payload.Success {
		return nil, 0, fmt.Errorf("获取用户列表失败: %s", payload.Message)
	}
	return payload.Data.Items, payload.Data.Total, nil
}

func (c *Client) GetUser(ctx context.Context, adminToken string, userID int64) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/user/%d", c.BaseURL(), userID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	c.setAdminHeaders(req.Header)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("获取用户详情失败: %d", resp.StatusCode)
	}

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    User   `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.Success {
		return nil, fmt.Errorf("获取用户详情失败: %s", payload.Message)
	}
	return &payload.Data, nil
}

func (c *Client) SearchUsers(ctx context.Context, adminToken, keyword string, page, pageSize int) ([]User, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/user/search?keyword=%s&p=%d&page_size=%d", c.BaseURL(), urlQueryEscape(keyword), page, pageSize), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	c.setAdminHeaders(req.Header)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, 0, fmt.Errorf("搜索用户失败: %d", resp.StatusCode)
	}

	var payload struct {
		Success bool     `json:"success"`
		Message string   `json:"message"`
		Data    UserPage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, err
	}
	if !payload.Success {
		return nil, 0, fmt.Errorf("搜索用户失败: %s", payload.Message)
	}
	return payload.Data.Items, payload.Data.Total, nil
}

func copyInterestingHeaders(dst, src http.Header) {
	for _, key := range []string{"Authorization", "Cookie", "New-Api-User"} {
		if value := src.Get(key); value != "" {
			dst.Set(key, value)
		}
	}
}

func (c *Client) setAdminHeaders(headers http.Header) {
	if userID := c.adminUserIDValue(); userID != "" {
		headers.Set("New-Api-User", userID)
	}
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(
		" ", "%20",
		"+", "%2B",
		"&", "%26",
		"=", "%3D",
		"?", "%3F",
		"#", "%23",
	)
	return replacer.Replace(value)
}
