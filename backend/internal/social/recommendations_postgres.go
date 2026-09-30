package social

import "context"

func (s *PostgresStore) RecommendPeople(ctx context.Context, viewerID string, limit int) ([]Recommendation, error) {
	const query = `
		WITH viewer_interests AS (
			SELECT interest_slug FROM user_interests WHERE user_id=$1::uuid
		), mutuals AS (
			SELECT f2.followed_id AS candidate_id, count(*)::bigint AS mutual_count
			FROM follows f1
			JOIN follows f2 ON f2.follower_id=f1.followed_id
			WHERE f1.follower_id=$1::uuid AND f2.followed_id<>$1::uuid
			GROUP BY f2.followed_id
		)
		SELECT p.username,p.display_name,p.bio,
		       (SELECT count(*)::bigint FROM follows fx WHERE fx.followed_id=p.user_id) AS followers_count,
		       (SELECT count(*)::bigint FROM user_interests ui JOIN viewer_interests vi ON vi.interest_slug=ui.interest_slug WHERE ui.user_id=p.user_id) AS shared_interests,
		       COALESCE(m.mutual_count,0) AS mutual_connections,
		       CASE
			WHEN COALESCE(m.mutual_count,0)>0 THEN 'Есть общие знакомства'
			WHEN EXISTS(SELECT 1 FROM user_interests ui JOIN viewer_interests vi ON vi.interest_slug=ui.interest_slug WHERE ui.user_id=p.user_id) THEN 'У вас общие интересы'
			ELSE 'Новый человек в CHAT'
		   END AS reason
		FROM profiles p
		JOIN users u ON u.id=p.user_id AND u.status='active'
		LEFT JOIN mutuals m ON m.candidate_id=p.user_id
		WHERE p.onboarding_completed_at IS NOT NULL
		  AND p.user_id<>$1::uuid
		  AND NOT EXISTS(SELECT 1 FROM follows f WHERE f.follower_id=$1::uuid AND f.followed_id=p.user_id)
		  AND NOT EXISTS(
			SELECT 1 FROM user_blocks b
			WHERE (b.blocker_id=$1::uuid AND b.blocked_id=p.user_id)
			   OR (b.blocker_id=p.user_id AND b.blocked_id=$1::uuid)
		  )
		ORDER BY
			COALESCE(m.mutual_count,0) DESC,
			(SELECT count(*) FROM user_interests ui JOIN viewer_interests vi ON vi.interest_slug=ui.interest_slug WHERE ui.user_id=p.user_id) DESC,
			followers_count DESC,
			p.username ASC
		LIMIT $2`
	rows, err := s.pool.Query(ctx, query, viewerID, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]Recommendation, 0, limit)
	for rows.Next() {
		var item Recommendation
		if err := rows.Scan(&item.Username,&item.DisplayName,&item.Bio,&item.FollowersCount,&item.SharedInterests,&item.MutualConnections,&item.Reason); err != nil {
			return nil, err
		}
		items = append(items,item)
	}
	return items, rows.Err()
}
