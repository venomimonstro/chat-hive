package communities

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Create(ctx context.Context, input CreateInput) (Community, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return Community{}, err }
	defer func(){ _ = tx.Rollback(ctx) }()
	var chatID string
	if err := tx.QueryRow(ctx, `INSERT INTO chats(kind,title,description,created_by) VALUES('group',$1,$2,$3::uuid) RETURNING id::text`, input.Title,input.Description,input.OwnerID).Scan(&chatID); err != nil { return Community{}, err }
	if _, err := tx.Exec(ctx, `INSERT INTO chat_members(chat_id,user_id,role) VALUES($1::uuid,$2::uuid,'owner')`, chatID,input.OwnerID); err != nil { return Community{}, err }
	status := "not_required"
	if input.Visibility == "public" { status = "pending" }
	var result Community
	const insert = `
		INSERT INTO communities(owner_id,chat_id,slug,title,description,visibility,moderation_status)
		VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7)
		RETURNING id::text,chat_id::text,slug,title,description,visibility,moderation_status,created_at`
	if err := tx.QueryRow(ctx,insert,input.OwnerID,chatID,input.Slug,input.Title,input.Description,input.Visibility,status).Scan(&result.ID,&result.ChatID,&result.Slug,&result.Title,&result.Description,&result.Visibility,&result.ModerationStatus,&result.CreatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err,&pgErr) && pgErr.Code=="23505" { return Community{},ErrSlugTaken }
		return Community{},err
	}
	if _,err:=tx.Exec(ctx,`INSERT INTO community_members(community_id,user_id,role) VALUES($1::uuid,$2::uuid,'owner')`,result.ID,input.OwnerID);err!=nil{return Community{},err}
	if input.Visibility=="public" {
		var caseID string
		if err:=tx.QueryRow(ctx,`INSERT INTO moderation_cases(target_type,target_id,severity,status,reason) VALUES('group',$1::uuid,'low','open','public_community_review') RETURNING id::text`,chatID).Scan(&caseID);err!=nil{return Community{},err}
		if _,err:=tx.Exec(ctx,`INSERT INTO moderation_actions(case_id,action,target_type,target_id,reason) VALUES($1::uuid,'case_created','group',$2::uuid,'public_community_review')`,caseID,chatID);err!=nil{return Community{},err}
	}
	if err:=tx.Commit(ctx);err!=nil{return Community{},err}
	result.Role="owner";result.MembersCount=1
	return result,nil
}

func (s *PostgresStore) Get(ctx context.Context, viewerID, slug string) (Community, error) {
	const query=`
		SELECT c.id::text,c.chat_id::text,c.slug,c.title,c.description,c.visibility,c.moderation_status,c.created_at,
		       COALESCE(cm.role,''),(SELECT count(*) FROM community_members x WHERE x.community_id=c.id AND x.left_at IS NULL)
		FROM communities c
		LEFT JOIN community_members cm ON cm.community_id=c.id AND cm.user_id=$1::uuid AND cm.left_at IS NULL
		WHERE c.slug=$2
		  AND (cm.user_id IS NOT NULL OR (c.visibility='public' AND c.moderation_status='approved'))
		  AND NOT EXISTS(SELECT 1 FROM user_blocks b WHERE (b.blocker_id=$1::uuid AND b.blocked_id=c.owner_id) OR (b.blocker_id=c.owner_id AND b.blocked_id=$1::uuid))`
	var item Community
	if err:=s.pool.QueryRow(ctx,query,viewerID,slug).Scan(&item.ID,&item.ChatID,&item.Slug,&item.Title,&item.Description,&item.Visibility,&item.ModerationStatus,&item.CreatedAt,&item.Role,&item.MembersCount);err!=nil{
		if errors.Is(err,pgx.ErrNoRows){return Community{},ErrNotFound};return Community{},err
	}
	return item,nil
}

func (s *PostgresStore) Discover(ctx context.Context, viewerID string, limit int) ([]Community, error) {
	rows,err:=s.pool.Query(ctx,`
		SELECT c.id::text,c.chat_id::text,c.slug,c.title,c.description,c.visibility,c.moderation_status,c.created_at,
		       COALESCE(cm.role,''),(SELECT count(*) FROM community_members x WHERE x.community_id=c.id AND x.left_at IS NULL)
		FROM communities c
		LEFT JOIN community_members cm ON cm.community_id=c.id AND cm.user_id=$1::uuid AND cm.left_at IS NULL
		WHERE c.visibility='public' AND c.moderation_status='approved'
		  AND NOT EXISTS(SELECT 1 FROM user_blocks b WHERE (b.blocker_id=$1::uuid AND b.blocked_id=c.owner_id) OR (b.blocker_id=c.owner_id AND b.blocked_id=$1::uuid))
		ORDER BY (SELECT count(*) FROM community_members x WHERE x.community_id=c.id AND x.left_at IS NULL) DESC,c.updated_at DESC,c.id DESC
		LIMIT $2`,viewerID,limit)
	if err!=nil{return nil,err};defer rows.Close()
	items:=make([]Community,0,limit)
	for rows.Next(){var item Community;if err:=rows.Scan(&item.ID,&item.ChatID,&item.Slug,&item.Title,&item.Description,&item.Visibility,&item.ModerationStatus,&item.CreatedAt,&item.Role,&item.MembersCount);err!=nil{return nil,err};items=append(items,item)}
	return items,rows.Err()
}

func (s *PostgresStore) Join(ctx context.Context,userID,slug string)(Community,error){
	tx,err:=s.pool.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return Community{},err};defer func(){_=tx.Rollback(ctx)}()
	var id,chatID,ownerID,title,description,visibility,status string;var createdAt any
	if err:=tx.QueryRow(ctx,`SELECT id::text,chat_id::text,owner_id::text,title,description,visibility,moderation_status,created_at FROM communities WHERE slug=$1 FOR UPDATE`,slug).Scan(&id,&chatID,&ownerID,&title,&description,&visibility,&status,&createdAt);err!=nil{if errors.Is(err,pgx.ErrNoRows){return Community{},ErrNotFound};return Community{},err}
	if visibility!="public"||status!="approved"{return Community{},ErrForbidden}
	var blocked bool;if err:=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM user_blocks WHERE (blocker_id=$1::uuid AND blocked_id=$2::uuid) OR (blocker_id=$2::uuid AND blocked_id=$1::uuid))`,userID,ownerID).Scan(&blocked);err!=nil{return Community{},err};if blocked{return Community{},ErrForbidden}
	if _,err:=tx.Exec(ctx,`INSERT INTO community_members(community_id,user_id,role,left_at) VALUES($1::uuid,$2::uuid,'member',NULL) ON CONFLICT(community_id,user_id) DO UPDATE SET left_at=NULL,role=CASE WHEN community_members.role='owner' THEN 'owner' ELSE 'member' END`,id,userID);err!=nil{return Community{},err}
	if _,err:=tx.Exec(ctx,`INSERT INTO chat_members(chat_id,user_id,role,left_at) VALUES($1::uuid,$2::uuid,'member',NULL) ON CONFLICT(chat_id,user_id) DO UPDATE SET left_at=NULL,role=CASE WHEN chat_members.role='owner' THEN 'owner' ELSE 'member' END`,chatID,userID);err!=nil{return Community{},err}
	if err:=tx.Commit(ctx);err!=nil{return Community{},err}
	return s.Get(ctx,userID,slug)
}

func (s *PostgresStore) Leave(ctx context.Context,userID,slug string)error{
	tx,err:=s.pool.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var id,chatID,role string
	if err:=tx.QueryRow(ctx,`SELECT c.id::text,c.chat_id::text,cm.role FROM communities c JOIN community_members cm ON cm.community_id=c.id WHERE c.slug=$1 AND cm.user_id=$2::uuid AND cm.left_at IS NULL FOR UPDATE OF cm`,slug,userID).Scan(&id,&chatID,&role);err!=nil{if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};return err}
	if role=="owner"{return ErrForbidden}
	if _,err:=tx.Exec(ctx,`UPDATE community_members SET left_at=now() WHERE community_id=$1::uuid AND user_id=$2::uuid`,id,userID);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`UPDATE chat_members SET left_at=now() WHERE chat_id=$1::uuid AND user_id=$2::uuid`,chatID,userID);err!=nil{return err}
	return tx.Commit(ctx)
}
