package newapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

type UserSelf struct {
	UserID int64 `json:"user_id"`
	Quota  int   `json:"quota"`
}

func (c *Client) SearchToken(ctx context.Context, adminToken, token string) (int64, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/token/search?keyword="+urlQueryEscape(token), nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/user/self", nil)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/user/topup/complete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

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
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/api/user/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

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
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/user/", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

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
	return 0, fmt.Errorf("未返回 user_id")
}

func copyInterestingHeaders(dst, src http.Header) {
	for _, key := range []string{"Authorization", "Cookie", "New-Api-User"} {
		if value := src.Get(key); value != "" {
			dst.Set(key, value)
		}
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
