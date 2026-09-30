package channels

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Create(ctx context.Context, input CreateInput) (Channel, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return Channel{}, err }
	defer func(){ _ = tx.Rollback(ctx) }()
	var result Channel
	const insert = `
		INSERT INTO channels(owner_id,slug,title,description,moderation_status)
		VALUES($1::uuid,$2,$3,$4,'pending')
		RETURNING id::text,owner_id::text,slug,title,description,moderation_status,created_at`
	if err := tx.QueryRow(ctx, insert, input.OwnerID, input.Slug, input.Title, input.Description).Scan(
		&result.ID,&result.OwnerID,&result.Slug,&result.Title,&result.Description,&result.ModerationStatus,&result.CreatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err,&pgErr) && pgErr.Code=="23505" { return Channel{}, ErrSlugTaken }
		return Channel{}, err
	}
	if _,err:=tx.Exec(ctx,`INSERT INTO channel_subscribers(channel_id,user_id) VALUES($1::uuid,$2::uuid)`,result.ID,input.OwnerID);err!=nil{return Channel{},err}
	var caseID string
	if err:=tx.QueryRow(ctx,`INSERT INTO moderation_cases(target_type,target_id,severity,status,reason) VALUES('group',$1::uuid,'low','open','public_channel_review') RETURNING id::text`,result.ID).Scan(&caseID);err!=nil{return Channel{},err}
	if _,err:=tx.Exec(ctx,`INSERT INTO moderation_actions(case_id,action,target_type,target_id,reason) VALUES($1::uuid,'case_created','group',$2::uuid,'public_channel_review')`,caseID,result.ID);err!=nil{return Channel{},err}
	if err:=tx.Commit(ctx);err!=nil{return Channel{},err}
	result.SubscribersCount=1;result.Subscribed=true;result.Mine=true
	return result,nil
}

func (s *PostgresStore) Get(ctx context.Context, viewerID, slug string) (Channel, error) {
	const query=`
		SELECT c.id::text,c.owner_id::text,c.slug,c.title,c.description,c.moderation_status,c.created_at,
		       (SELECT count(*) FROM channel_subscribers x WHERE x.channel_id=c.id),
		       EXISTS(SELECT 1 FROM channel_subscribers x WHERE x.channel_id=c.id AND x.user_id=$1::uuid),
		       c.owner_id=$1::uuid
		FROM channels c
		WHERE c.slug=$2
		  AND (c.owner_id=$1::uuid OR c.moderation_status='approved')
		  AND NOT EXISTS(SELECT 1 FROM user_blocks b WHERE (b.blocker_id=$1::uuid AND b.blocked_id=c.owner_id) OR (b.blocker_id=c.owner_id AND b.blocked_id=$1::uuid))`
	var item Channel
	if err:=s.pool.QueryRow(ctx,query,viewerID,slug).Scan(&item.ID,&item.OwnerID,&item.Slug,&item.Title,&item.Description,&item.ModerationStatus,&item.CreatedAt,&item.SubscribersCount,&item.Subscribed,&item.Mine);err!=nil{
		if errors.Is(err,pgx.ErrNoRows){return Channel{},ErrNotFound};return Channel{},err
	}
	return item,nil
}

func (s *PostgresStore) Discover(ctx context.Context, viewerID string, limit int) ([]Channel, error) {
	rows,err:=s.pool.Query(ctx,`
		SELECT c.id::text,c.owner_id::text,c.slug,c.title,c.description,c.moderation_status,c.created_at,
		       (SELECT count(*) FROM channel_subscribers x WHERE x.channel_id=c.id),
		       EXISTS(SELECT 1 FROM channel_subscribers x WHERE x.channel_id=c.id AND x.user_id=$1::uuid),
		       c.owner_id=$1::uuid
		FROM channels c
		WHERE c.moderation_status='approved'
		  AND NOT EXISTS(SELECT 1 FROM user_blocks b WHERE (b.blocker_id=$1::uuid AND b.blocked_id=c.owner_id) OR (b.blocker_id=c.owner_id AND b.blocked_id=$1::uuid))
		ORDER BY (SELECT count(*) FROM channel_subscribers x WHERE x.channel_id=c.id) DESC,c.updated_at DESC,c.id DESC
		LIMIT $2`,viewerID,limit)
	if err!=nil{return nil,err};defer rows.Close()
	items:=make([]Channel,0,limit)
	for rows.Next(){var item Channel;if err:=rows.Scan(&item.ID,&item.OwnerID,&item.Slug,&item.Title,&item.Description,&item.ModerationStatus,&item.CreatedAt,&item.SubscribersCount,&item.Subscribed,&item.Mine);err!=nil{return nil,err};items=append(items,item)}
	return items,rows.Err()
}

func (s *PostgresStore) SetSubscription(ctx context.Context, userID, slug string, enabled bool) (Channel, error) {
	tx,err:=s.pool.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return Channel{},err};defer func(){_=tx.Rollback(ctx)}()
	var channelID,ownerID,status string
	if err:=tx.QueryRow(ctx,`SELECT id::text,owner_id::text,moderation_status FROM channels WHERE slug=$1 FOR UPDATE`,slug).Scan(&channelID,&ownerID,&status);err!=nil{if errors.Is(err,pgx.ErrNoRows){return Channel{},ErrNotFound};return Channel{},err}
	if status!="approved" && ownerID!=userID{return Channel{},ErrForbidden}
	if enabled {
		if _,err:=tx.Exec(ctx,`INSERT INTO channel_subscribers(channel_id,user_id) VALUES($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`,channelID,userID);err!=nil{return Channel{},err}
	} else {
		if ownerID==userID{return Channel{},ErrForbidden}
		if _,err:=tx.Exec(ctx,`DELETE FROM channel_subscribers WHERE channel_id=$1::uuid AND user_id=$2::uuid`,channelID,userID);err!=nil{return Channel{},err}
	}
	if err:=tx.Commit(ctx);err!=nil{return Channel{},err}
	return s.Get(ctx,userID,slug)
}

func (s *PostgresStore) CreatePost(ctx context.Context, userID, slug, body string) (Post, error) {
	tx,err:=s.pool.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return Post{},err};defer func(){_=tx.Rollback(ctx)}()
	var channelID,ownerID string
	if err:=tx.QueryRow(ctx,`SELECT id::text,owner_id::text FROM channels WHERE slug=$1 FOR UPDATE`,slug).Scan(&channelID,&ownerID);err!=nil{if errors.Is(err,pgx.ErrNoRows){return Post{},ErrNotFound};return Post{},err}
	if ownerID!=userID{return Post{},ErrForbidden}
	var post Post
	if err:=tx.QueryRow(ctx,`INSERT INTO posts(author_id,kind,body,visibility) VALUES($1::uuid,'post',$2,'public') RETURNING id::text,kind,body,created_at`,userID,body).Scan(&post.ID,&post.Kind,&post.Body,&post.CreatedAt);err!=nil{return Post{},err}
	if _,err:=tx.Exec(ctx,`INSERT INTO channel_posts(channel_id,post_id) VALUES($1::uuid,$2::uuid)`,channelID,post.ID);err!=nil{return Post{},err}
	if _,err:=tx.Exec(ctx,`UPDATE channels SET updated_at=now() WHERE id=$1::uuid`,channelID);err!=nil{return Post{},err}
	if err:=tx.Commit(ctx);err!=nil{return Post{},err}
	return post,nil
}

func (s *PostgresStore) ListPosts(ctx context.Context, viewerID, slug string, limit int) ([]Post, error) {
	var channelID,ownerID,status string
	if err:=s.pool.QueryRow(ctx,`SELECT id::text,owner_id::text,moderation_status FROM channels WHERE slug=$1`,slug).Scan(&channelID,&ownerID,&status);err!=nil{if errors.Is(err,pgx.ErrNoRows){return nil,ErrNotFound};return nil,err}
	if ownerID!=viewerID && status!="approved"{return nil,ErrNotFound}
	rows,err:=s.pool.Query(ctx,`SELECT p.id::text,p.kind,p.body,p.created_at FROM channel_posts cp JOIN posts p ON p.id=cp.post_id WHERE cp.channel_id=$1::uuid AND p.deleted_at IS NULL ORDER BY cp.created_at DESC,p.id DESC LIMIT $2`,channelID,limit)
	if err!=nil{return nil,err};defer rows.Close()
	items:=make([]Post,0,limit);for rows.Next(){var item Post;if err:=rows.Scan(&item.ID,&item.Kind,&item.Body,&item.CreatedAt);err!=nil{return nil,err};items=append(items,item)}
	return items,rows.Err()
}
