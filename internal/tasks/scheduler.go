package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"newapiguard/internal/cache"
	"newapiguard/internal/newapi"
	"newapiguard/internal/settings"
)

func Start(ctx context.Context, db *sql.DB, settingsStore *settings.Store, runtimeCache *cache.Store, client *newapi.Client, tokenCacheTTL time.Duration) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runtimeCache.Cleanup()
				cleanupExpiredTokenCache(db, tokenCacheTTL)
				cleanupExpiredBans(db, settingsStore, client)
				cleanupExpiredOAuth(db)
			}
		}
	}()
}

func cleanupExpiredTokenCache(db *sql.DB, ttl time.Duration) {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	_, _ = db.Exec(`DELETE FROM token_cache WHERE cached_at <= datetime('now', ?)`, fmt.Sprintf("-%d seconds", int(ttl.Seconds())))
}

func cleanupExpiredBans(db *sql.DB, settingsStore *settings.Store, client *newapi.Client) {
	rows, err := db.Query(`SELECT id, newapi_user_id FROM bans WHERE unbanned_at IS NULL AND expire_at IS NOT NULL AND expire_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return
	}
	defer rows.Close()

	type expiredBan struct{ banID, userID int64 }
	var expired []expiredBan
	for rows.Next() {
		var b expiredBan
		if err := rows.Scan(&b.banID, &b.userID); err != nil {
			continue
		}
		expired = append(expired, b)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[tasks] cleanupExpiredBans scan error: %v", err)
		return
	}

	adminToken := settingsStore.GetString("newapi_admin_token")
	for _, b := range expired {
		if adminToken != "" {
			if err := client.UpdateUserStatus(context.Background(), adminToken, b.userID, 1); err != nil {
				log.Printf("[tasks] unban upstream user %d failed: %v", b.userID, err)
			}
		}
		_, _ = db.Exec(`UPDATE bans SET unbanned_at=CURRENT_TIMESTAMP WHERE id=?`, b.banID)
		_, _ = db.Exec(`DELETE FROM ua_strikes WHERE newapi_user_id=?`, b.userID)
	}
}

func cleanupExpiredOAuth(db *sql.DB) {
	_, _ = db.Exec(`DELETE FROM oauth_pending_states WHERE expire_at <= CURRENT_TIMESTAMP`)
	_, _ = db.Exec(`DELETE FROM oauth_authorization_codes WHERE expire_at <= CURRENT_TIMESTAMP OR used_at IS NOT NULL`)
	_, _ = db.Exec(`DELETE FROM oauth_access_tokens WHERE expire_at <= CURRENT_TIMESTAMP`)
}
