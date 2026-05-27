package admin

import (
	"context"
	"database/sql"
	"log"

	"newapiguard/internal/newapi"
)

func (h *Handler) reconcileUsers(ctx context.Context, adminToken string) map[int64]int64 {
	mapping := map[int64]int64{}
	if adminToken == "" {
		return mapping
	}
	remoteUsers, err := h.fetchAllNewAPIUsers(ctx, adminToken)
	if err != nil {
		return mapping
	}
	remoteByID := map[int64]newapi.User{}
	remoteByUsername := map[string]newapi.User{}
	remoteByDisplayName := map[string]newapi.User{}
	displayNameCounts := map[string]int{}
	for _, user := range remoteUsers {
		if user.DisplayName != "" {
			displayNameCounts[user.DisplayName]++
		}
	}
	for _, user := range remoteUsers {
		remoteByID[user.ID] = user
		if user.Username != "" {
			remoteByUsername[user.Username] = user
		}
		if user.DisplayName != "" && displayNameCounts[user.DisplayName] == 1 {
			remoteByDisplayName[user.DisplayName] = user
		}
		_ = h.upsertLocalUserIdentity(user.ID, user.Username, user.DisplayName)
	}
	linkRows, err := h.db.Query(`SELECT discord_id, discord_name, preferred_username FROM oauth_identity_links`)
	if err == nil {
		defer linkRows.Close()
		type linkEntry struct {
			discordID, discordName, preferredUsername string
		}
		var links []linkEntry
		for linkRows.Next() {
			var discordID, discordName, preferredUsername sql.NullString
			if err := linkRows.Scan(&discordID, &discordName, &preferredUsername); err != nil {
				continue
			}
			links = append(links, linkEntry{discordID.String, discordName.String, preferredUsername.String})
		}
		for _, link := range links {
			if user, ok := remoteByUsername[link.preferredUsername]; ok {
				_, _ = h.db.Exec(`INSERT INTO users(newapi_user_id, username, display_name, discord_id, discord_name)
					VALUES(?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))
					ON CONFLICT(newapi_user_id) DO UPDATE SET
						username=CASE WHEN excluded.username != '' THEN excluded.username ELSE users.username END,
						display_name=CASE WHEN excluded.display_name != '' THEN excluded.display_name ELSE users.display_name END,
						discord_id=CASE WHEN excluded.discord_id IS NOT NULL THEN excluded.discord_id ELSE users.discord_id END,
						discord_name=CASE WHEN excluded.discord_name IS NOT NULL THEN excluded.discord_name ELSE users.discord_name END`, user.ID, user.Username, user.DisplayName, link.discordID, link.discordName)
				_, _ = h.db.Exec(`UPDATE oauth_identity_links SET newapi_user_id=?, updated_at=CURRENT_TIMESTAMP WHERE discord_id=?`, user.ID, link.discordID)
			}
		}
	}

	rows, err := h.db.Query(`SELECT newapi_user_id, username, display_name, discord_id, discord_name FROM users`)
	if err != nil {
		return mapping
	}
	defer rows.Close()
	for rows.Next() {
		var userID int64
		var username, displayName, discordID, discordName sql.NullString
		if err := rows.Scan(&userID, &username, &displayName, &discordID, &discordName); err != nil {
			continue
		}
		if _, ok := remoteByID[userID]; ok {
			continue
		}
		if username.String != "" {
			if user, ok := remoteByUsername[username.String]; ok {
				mapping[userID] = user.ID
				_ = h.mergeLocalUserIDs(userID, user.ID, user.Username, user.DisplayName, discordID.String, discordName.String)
				continue
			}
		}
		if displayName.String != "" {
			if user, ok := remoteByDisplayName[displayName.String]; ok {
				mapping[userID] = user.ID
				_ = h.mergeLocalUserIDs(userID, user.ID, user.Username, user.DisplayName, discordID.String, discordName.String)
				continue
			}
		}
	}
	return mapping
}

func (h *Handler) mergeLocalUserIDs(fromID, toID int64, username, displayName, discordID, discordName string) error {
	if fromID <= 0 || toID <= 0 || fromID == toID {
		return nil
	}
	tx, err := h.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			log.Printf("[reconcile] 合并用户 %d -> %d 失败: %v", fromID, toID, err)
		}
	}()
	if _, err = tx.Exec(`INSERT INTO users(newapi_user_id, username, display_name, discord_id, discord_name)
		VALUES(?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))
		ON CONFLICT(newapi_user_id) DO UPDATE SET
			username=CASE WHEN excluded.username != '' THEN excluded.username ELSE users.username END,
			display_name=CASE WHEN excluded.display_name != '' THEN excluded.display_name ELSE users.display_name END,
			discord_id=CASE WHEN excluded.discord_id IS NOT NULL THEN excluded.discord_id ELSE users.discord_id END,
			discord_name=CASE WHEN excluded.discord_name IS NOT NULL THEN excluded.discord_name ELSE users.discord_name END`, toID, username, displayName, discordID, discordName); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE bans SET newapi_user_id=? WHERE newapi_user_id=?`, toID, fromID); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE token_cache SET newapi_user_id=? WHERE newapi_user_id=?`, toID, fromID); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM ua_strikes WHERE newapi_user_id=?`, fromID); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE bans SET unbanned_at=CURRENT_TIMESTAMP WHERE newapi_user_id=? AND unbanned_at IS NULL AND id NOT IN (
		SELECT id FROM bans WHERE newapi_user_id=? AND unbanned_at IS NULL ORDER BY created_at DESC LIMIT 1
	)`, toID, toID); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM users WHERE newapi_user_id=?`, fromID); err != nil {
		return err
	}
	err = tx.Commit()
	return err
}
