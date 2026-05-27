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

const currentSchemaVersion = 1

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}
	var version int
	if err := db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&version); err != nil {
		_, _ = db.Exec(`INSERT INTO schema_version(version) VALUES(0)`)
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			newapi_user_id INTEGER PRIMARY KEY,
			username TEXT,
			display_name TEXT,
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
			client_id TEXT NOT NULL,
			redirect_uri TEXT NOT NULL,
			discord_id TEXT NOT NULL,
			discord_name TEXT,
			payload TEXT NOT NULL,
			expire_at DATETIME NOT NULL,
			used_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS oauth_pending_states (
			state TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			redirect_uri TEXT NOT NULL,
			original_state TEXT NOT NULL,
			scope TEXT,
			expire_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS oauth_access_tokens (
			access_token TEXT PRIMARY KEY,
			payload TEXT NOT NULL,
			expire_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS oauth_identity_links (
			discord_id TEXT PRIMARY KEY,
			discord_name TEXT,
			preferred_username TEXT,
			newapi_user_id INTEGER,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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

	if err := ensureColumn(db, "oauth_authorization_codes", "client_id", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := ensureColumn(db, "oauth_authorization_codes", "redirect_uri", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "username", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := ensureColumn(db, "users", "display_name", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}

	if _, err := db.Exec(`UPDATE schema_version SET version=?`, currentSchemaVersion); err != nil {
		return err
	}
	return nil
}

func ensureColumn(db *sql.DB, table, column, def string) error {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, def))
	return err
}
