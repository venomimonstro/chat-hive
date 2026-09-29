package identity

import (
	"context"
	"time"
)

func (s *PostgresStore) ListSessions(ctx context.Context, userID string, now time.Time) ([]DeviceSession, error) {
	const query = `
		SELECT id::text, user_agent, COALESCE(last_ip::text, ''), created_at, last_seen_at, expires_at
		FROM sessions
		WHERE user_id = $1::uuid AND revoked_at IS NULL AND expires_at > $2
		ORDER BY last_seen_at DESC, created_at DESC`
	rows, err := s.pool.Query(ctx, query, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DeviceSession, 0, 8)
	for rows.Next() {
		var item DeviceSession
		if err := rows.Scan(&item.SessionID, &item.UserAgent, &item.LastIP, &item.CreatedAt, &item.LastSeenAt, &item.ExpiresAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
