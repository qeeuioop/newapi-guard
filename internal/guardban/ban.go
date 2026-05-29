package guardban

import (
	"database/sql"
	"time"
)

func ExpireAtForDuration(duration string, now time.Time) (string, any) {
	switch duration {
	case "7d":
		return duration, now.Add(7 * 24 * time.Hour).Format(time.RFC3339)
	case "30d":
		return duration, now.Add(30 * 24 * time.Hour).Format(time.RFC3339)
	default:
		return "permanent", nil
	}
}

func NullableString(value sql.NullString) any {
	if !value.Valid || value.String == "" {
		return nil
	}
	return value.String
}
