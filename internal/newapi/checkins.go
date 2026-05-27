package newapi

import "context"

type CheckinRecord struct {
	ID           int64
	UserID       int64
	QuotaAwarded int64
	CheckinDate  string
	CreatedAt    int64
}

func (r *TokenResolver) ListCheckins(ctx context.Context, userID int64, limit, offset int) ([]CheckinRecord, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.quota_awarded, c.checkin_date::text, c.created_at
		FROM checkins c
		JOIN users u ON u.id = c.user_id AND u.deleted_at IS NULL
		WHERE ($1::bigint = 0 OR c.user_id = $1)
		ORDER BY c.checkin_date DESC, c.id DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CheckinRecord{}
	for rows.Next() {
		var item CheckinRecord
		if err := rows.Scan(&item.ID, &item.UserID, &item.QuotaAwarded, &item.CheckinDate, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
