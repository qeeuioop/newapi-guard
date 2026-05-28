package newapi

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type ResolvedToken struct {
	UserID      int64
	Name        string
	Username    string
	DisplayName string
}

type DiscordOAuthBinding struct {
	UserID      int64
	DiscordID   string
	Username    string
	DisplayName string
}

type TokenResolver struct {
	db *sql.DB
}

func NewTokenResolver(dsn string) (*TokenResolver, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, nil
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &TokenResolver{db: db}, nil
}

func (r *TokenResolver) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *TokenResolver) ListDiscordOAuthBindings(ctx context.Context) ([]DiscordOAuthBinding, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT b.user_id, TRIM(LEADING 'discord:' FROM b.provider_user_id), COALESCE(u.username, ''), COALESCE(u.display_name, '')
		FROM user_oauth_bindings b
		JOIN users u ON u.id = b.user_id
		WHERE b.provider_user_id LIKE 'discord:%'
		  AND b.provider_user_id <> ''
		  AND (u.deleted_at IS NULL)`)
	if err != nil {
		return nil, fmt.Errorf("查询 NewAPI OAuth 绑定失败: %w", err)
	}
	defer rows.Close()

	bindings := []DiscordOAuthBinding{}
	for rows.Next() {
		var binding DiscordOAuthBinding
		if err := rows.Scan(&binding.UserID, &binding.DiscordID, &binding.Username, &binding.DisplayName); err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func (r *TokenResolver) IsUserDeleted(ctx context.Context, userID int64) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	var deletedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT deleted_at FROM users WHERE id = $1`, userID).Scan(&deletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return true, nil
		}
		return false, fmt.Errorf("查询用户删除状态失败: %w", err)
	}
	return deletedAt.Valid, nil
}

func (r *TokenResolver) Resolve(ctx context.Context, token string) (int64, bool, error) {
	resolved, ok, err := r.ResolveToken(ctx, token)
	return resolved.UserID, ok, err
}

func (r *TokenResolver) ResolveToken(ctx context.Context, token string) (ResolvedToken, bool, error) {
	if r == nil || r.db == nil {
		return ResolvedToken{}, false, nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ResolvedToken{}, false, nil
	}
	lookup := token
	withoutPrefix := strings.TrimPrefix(token, "sk-")
	var resolved ResolvedToken
	err := r.db.QueryRowContext(ctx, `
		SELECT t.user_id, COALESCE(t.name, ''), COALESCE(u.username, ''), COALESCE(u.display_name, '')
		FROM tokens t
		JOIN users u ON u.id = t.user_id
		WHERE (t.key = $1 OR t.key = $2)
		  AND t.status = 1
		  AND (t.deleted_at IS NULL)
		  AND (u.deleted_at IS NULL)
		  AND (t.expired_time = -1 OR t.expired_time > EXTRACT(EPOCH FROM NOW())::bigint)
		LIMIT 1`, lookup, withoutPrefix).Scan(&resolved.UserID, &resolved.Name, &resolved.Username, &resolved.DisplayName)
	if err == nil {
		return resolved, true, nil
	}
	if err == sql.ErrNoRows {
		return ResolvedToken{}, false, nil
	}
	return ResolvedToken{}, false, fmt.Errorf("查询 NewAPI token 失败: %w", err)
}
