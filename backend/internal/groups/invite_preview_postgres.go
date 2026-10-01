package groups

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) PreviewInvite(ctx context.Context, tokenHash []byte, now time.Time) (InvitePreview, error) {
	var preview InvitePreview
	const query = `
		SELECT c.title,
		       COALESCE(c.description,''),
		       (SELECT count(*) FROM chat_members cm WHERE cm.chat_id=c.id AND cm.left_at IS NULL)
		FROM group_invites gi
		JOIN chats c ON c.id=gi.chat_id AND c.kind='group'
		WHERE gi.token_hash=$1
		  AND gi.revoked_at IS NULL
		  AND (gi.expires_at IS NULL OR gi.expires_at > $2)
		  AND (gi.max_uses IS NULL OR gi.uses_count < gi.max_uses)
		LIMIT 1`
	if err := s.pool.QueryRow(ctx, query, tokenHash, now).Scan(&preview.Title, &preview.Description, &preview.MembersCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InvitePreview{}, ErrInviteInvalid
		}
		return InvitePreview{}, err
	}
	return preview, nil
}
