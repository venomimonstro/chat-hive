package communities

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) GetPublic(ctx context.Context, slug string) (Community, error) {
	const query = `
		SELECT c.id::text,c.chat_id::text,c.slug,c.title,c.description,c.visibility,c.moderation_status,c.created_at,
		       '',(SELECT count(*) FROM community_members x WHERE x.community_id=c.id AND x.left_at IS NULL)
		FROM communities c
		JOIN users u ON u.id=c.owner_id AND u.status='active'
		WHERE c.slug=$1 AND c.visibility='public' AND c.moderation_status='approved'`
	var item Community
	if err:=s.pool.QueryRow(ctx,query,slug).Scan(&item.ID,&item.ChatID,&item.Slug,&item.Title,&item.Description,&item.Visibility,&item.ModerationStatus,&item.CreatedAt,&item.Role,&item.MembersCount);err!=nil{
		if errors.Is(err,pgx.ErrNoRows){return Community{},ErrNotFound};return Community{},err
	}
	return item,nil
}

func (s *PostgresStore) DiscoverPublic(ctx context.Context, limit int) ([]Community, error) {
	rows,err:=s.pool.Query(ctx,`
		SELECT c.id::text,c.chat_id::text,c.slug,c.title,c.description,c.visibility,c.moderation_status,c.created_at,
		       '',(SELECT count(*) FROM community_members x WHERE x.community_id=c.id AND x.left_at IS NULL)
		FROM communities c
		JOIN users u ON u.id=c.owner_id AND u.status='active'
		WHERE c.visibility='public' AND c.moderation_status='approved'
		ORDER BY (SELECT count(*) FROM community_members x WHERE x.community_id=c.id AND x.left_at IS NULL) DESC,c.updated_at DESC,c.id DESC
		LIMIT $1`,limit)
	if err!=nil{return nil,err};defer rows.Close()
	items:=make([]Community,0,limit)
	for rows.Next(){var item Community;if err:=rows.Scan(&item.ID,&item.ChatID,&item.Slug,&item.Title,&item.Description,&item.Visibility,&item.ModerationStatus,&item.CreatedAt,&item.Role,&item.MembersCount);err!=nil{return nil,err};items=append(items,item)}
	return items,rows.Err()
}
