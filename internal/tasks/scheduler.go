package tasks

import (
	"context"
	"database/sql"
	"time"

	"newapiguard/internal/cache"
	"newapiguard/internal/newapi"
	"newapiguard/internal/settings"
)

func Start(ctx context.Context, db *sql.DB, settingsStore *settings.Store, runtimeCache *cache.Store, client *newapi.Client) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runtimeCache.Cleanup()
				cleanupExpiredBans(db, settingsStore, client)
			}
		}
	}()
}

func cleanupExpiredBans(db *sql.DB, settingsStore *settings.Store, client *newapi.Client) {
	rows, err := db.Query(`SELECT id, newapi_user_id FROM bans WHERE unbanned_at IS NULL AND expire_at IS NOT NULL AND expire_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return
	}
	defer rows.Close()

	adminToken := settingsStore.GetString("newapi_admin_token")
	for rows.Next() {
		var banID, userID int64
		if err := rows.Scan(&banID, &userID); err != nil {
			continue
		}
		if adminToken != "" {
			_ = client.UpdateUserStatus(context.Background(), adminToken, userID, 1)
		}
		_, _ = db.Exec(`UPDATE bans SET unbanned_at=CURRENT_TIMESTAMP WHERE id=?`, banID)
		_, _ = db.Exec(`DELETE FROM ua_strikes WHERE newapi_user_id=?`, userID)
	}
}
