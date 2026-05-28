package settings

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
)

type Store struct {
	db     *sql.DB
	mu     sync.RWMutex
	values map[string]string
}

var defaults = map[string]string{
	"rpm_limit":                   "3",
	"ua_ban_strikes":              "3",
	"checkin_threshold":           "0",
	"newapi_base_url":             "http://new-api:3000",
	"newapi_admin_user_id":        "",
	"public_base_url":             "",
	"allowed_origins":             `[]`,
	"oauth_client_id":             "",
	"oauth_client_secret":         "",
	"oauth_allowed_redirect_uris": `[]`,
	"oauth_provider_slug":         "guard-discord",
	"oauth_state_ttl_seconds":     "300",
	"oauth_code_ttl_seconds":      "120",
	"oauth_token_ttl_seconds":     "600",
	"discord_client_id":           "",
	"discord_client_secret":       "",
	"discord_guild_id":            "",
	"discord_oauth_scopes":        `["identify","guilds.members.read"]`,
	"discord_access_policy":       `{"logic":"and","conditions":[],"groups":[]}`,
	"admin_password":              "",
	"newapi_admin_token":          "",
	"ua_auto_ban_duration":        "permanent",
	"prompt_cache_enabled":        "true",
}

func NewStore(db *sql.DB) (*Store, error) {
	store := &Store{db: db, values: map[string]string{}}
	if err := store.ensureDefaults(); err != nil {
		return nil, err
	}
	if err := store.Reload(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) ensureDefaults() error {
	for key, value := range defaults {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO config(key, value) VALUES(?, ?)`, key, value); err != nil {
			return fmt.Errorf("初始化配置 %s 失败: %w", key, err)
		}
	}
	return nil
}

func (s *Store) Reload() error {
	rows, err := s.db.Query(`SELECT key, value FROM config`)
	if err != nil {
		return err
	}
	defer rows.Close()

	values := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return err
		}
		values[key] = value
	}

	s.mu.Lock()
	s.values = values
	s.mu.Unlock()
	return rows.Err()
}

func (s *Store) Update(updates map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for key, value := range updates {
		if _, err := tx.Exec(`INSERT INTO config(key, value) VALUES(?, ?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

func (s *Store) GetString(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[key]
}

func (s *Store) GetInt(key string, fallback int) int {
	raw := s.GetString(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func (s *Store) GetBool(key string, fallback bool) bool {
	raw := s.GetString(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func (s *Store) GetStringSlice(key string) []string {
	var items []string
	_ = json.Unmarshal([]byte(s.GetString(key)), &items)
	return items
}

func (s *Store) GetJSON(key string, target any) error {
	return json.Unmarshal([]byte(s.GetString(key)), target)
}
