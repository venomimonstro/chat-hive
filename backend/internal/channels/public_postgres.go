package channels

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) GetPublic(ctx context.Context, slug string) (Channel, error) {
	const query = `
		SELECT c.id::text,c.owner_id::text,c.slug,c.title,c.description,c.moderation_status,c.created_at,
		       (SELECT count(*) FROM channel_subscribers x WHERE x.channel_id=c.id),false,false
		FROM channels c
		JOIN users u ON u.id=c.owner_id AND u.status='active'
		WHERE c.slug=$1 AND c.moderation_status='approved'`
	var item Channel
	if err:=s.pool.QueryRow(ctx,query,slug).Scan(&item.ID,&item.OwnerID,&item.Slug,&item.Title,&item.Description,&item.ModerationStatus,&item.CreatedAt,&item.SubscribersCount,&item.Subscribed,&item.Mine);err!=nil{
		if errors.Is(err,pgx.ErrNoRows){return Channel{},ErrNotFound};return Channel{},err
	}
	return item,nil
}

func (s *PostgresStore) DiscoverPublic(ctx context.Context, limit int) ([]Channel, error) {
	rows,err:=s.pool.Query(ctx,`
		SELECT c.id::text,c.owner_id::text,c.slug,c.title,c.description,c.moderation_status,c.created_at,
		       (SELECT count(*) FROM channel_subscribers x WHERE x.channel_id=c.id),false,false
		FROM channels c
		JOIN users u ON u.id=c.owner_id AND u.status='active'
		WHERE c.moderation_status='approved'
		ORDER BY (SELECT count(*) FROM channel_subscribers x WHERE x.channel_id=c.id) DESC,c.updated_at DESC,c.id DESC
		LIMIT $1`,limit)
	if err!=nil{return nil,err};defer rows.Close()
	items:=make([]Channel,0,limit)
	for rows.Next(){var item Channel;if err:=rows.Scan(&item.ID,&item.OwnerID,&item.Slug,&item.Title,&item.Description,&item.ModerationStatus,&item.CreatedAt,&item.SubscribersCount,&item.Subscribed,&item.Mine);err!=nil{return nil,err};items=append(items,item)}
	return items,rows.Err()
}

func (s *PostgresStore) ListPublicPosts(ctx context.Context, slug string, limit int) ([]Post, error) {
	var channelID string
	if err:=s.pool.QueryRow(ctx,`SELECT id::text FROM channels WHERE slug=$1 AND moderation_status='approved'`,slug).Scan(&channelID);err!=nil{
		if errors.Is(err,pgx.ErrNoRows){return nil,ErrNotFound};return nil,err
	}
	rows,err:=s.pool.Query(ctx,`
		SELECT p.id::text,p.kind,p.body,p.created_at
		FROM channel_posts cp JOIN posts p ON p.id=cp.post_id
		WHERE cp.channel_id=$1::uuid AND p.deleted_at IS NULL AND p.visibility='public'
		ORDER BY cp.created_at DESC,p.id DESC LIMIT $2`,channelID,limit)
	if err!=nil{return nil,err};defer rows.Close()
	items:=make([]Post,0,limit)
	for rows.Next(){var item Post;if err:=rows.Scan(&item.ID,&item.Kind,&item.Body,&item.CreatedAt);err!=nil{return nil,err};items=append(items,item)}
	return items,rows.Err()
}
