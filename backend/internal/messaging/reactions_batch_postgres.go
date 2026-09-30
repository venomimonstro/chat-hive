package messaging

import "context"

func (s *PostgresStore) ListReactionBatch(ctx context.Context, userID string, messageIDs []string) (ReactionBatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT mr.message_id::text,mr.reaction,count(*)::bigint,bool_or(mr.user_id=$1::uuid)
		FROM message_reactions mr
		JOIN messages m ON m.id=mr.message_id AND m.deleted_at IS NULL
		JOIN chat_members cm ON cm.chat_id=m.chat_id AND cm.user_id=$1::uuid AND cm.left_at IS NULL
		WHERE mr.message_id::text=ANY($2::text[])
		GROUP BY mr.message_id,mr.reaction
		ORDER BY mr.message_id,count(*) DESC,mr.reaction ASC`, userID, messageIDs)
	if err != nil { return nil, err }
	defer rows.Close()
	result := make(ReactionBatch, len(messageIDs))
	for _, id := range messageIDs { result[id] = []ReactionSummary{} }
	for rows.Next() {
		var messageID string
		var item ReactionSummary
		if err := rows.Scan(&messageID,&item.Reaction,&item.Count,&item.Mine); err != nil { return nil,err }
		result[messageID] = append(result[messageID],item)
	}
	return result, rows.Err()
}
