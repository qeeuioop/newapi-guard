package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func Migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			newapi_user_id INTEGER PRIMARY KEY,
			discord_id TEXT UNIQUE,
			discord_name TEXT,
			is_whitelist INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS token_cache (
			token_key TEXT PRIMARY KEY,
			newapi_user_id INTEGER NOT NULL,
			cached_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (newapi_user_id) REFERENCES users(newapi_user_id)
		);`,
		`CREATE TABLE IF NOT EXISTS ua_strikes (
			newapi_user_id INTEGER PRIMARY KEY,
			count INTEGER NOT NULL DEFAULT 0,
			last_ua TEXT,
			last_strike_at DATETIME,
			FOREIGN KEY (newapi_user_id) REFERENCES users(newapi_user_id)
		);`,
		`CREATE TABLE IF NOT EXISTS bans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			newapi_user_id INTEGER NOT NULL,
			discord_id TEXT,
			reason TEXT NOT NULL,
			violation_ua TEXT,
			client_ip TEXT,
			duration TEXT NOT NULL,
			expire_at DATETIME,
			unbanned_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (newapi_user_id) REFERENCES users(newapi_user_id)
		);`,
		`CREATE TABLE IF NOT EXISTS checkin_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			newapi_user_id INTEGER NOT NULL,
			quota_added INTEGER NOT NULL,
			checked_at DATE NOT NULL,
			UNIQUE(newapi_user_id, checked_at),
			FOREIGN KEY (newapi_user_id) REFERENCES users(newapi_user_id)
		);`,
		`CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
			code TEXT PRIMARY KEY,
			discord_id TEXT NOT NULL,
			discord_name TEXT,
			payload TEXT NOT NULL,
			expire_at DATETIME NOT NULL,
			used_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS daily_stats (
			date TEXT PRIMARY KEY,
			total_requests INTEGER NOT NULL DEFAULT 0,
			blocked_ua INTEGER NOT NULL DEFAULT 0,
			blocked_rpm INTEGER NOT NULL DEFAULT 0,
			checkins INTEGER NOT NULL DEFAULT 0,
			new_users INTEGER NOT NULL DEFAULT 0,
			new_bans INTEGER NOT NULL DEFAULT 0
		);`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
