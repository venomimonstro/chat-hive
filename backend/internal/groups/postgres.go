package groups

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Create(ctx context.Context, ownerID, title, description string) (Group, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return Group{}, err }
	defer func(){ _ = tx.Rollback(ctx) }()
	var chatID string
	if err := tx.QueryRow(ctx, `INSERT INTO chats(kind,title,created_by) VALUES('group',$1,$2::uuid) RETURNING id::text`, title, ownerID).Scan(&chatID); err != nil { return Group{}, err }
	if _, err := tx.Exec(ctx, `INSERT INTO chat_members(chat_id,user_id,role) VALUES($1::uuid,$2::uuid,'owner')`, chatID, ownerID); err != nil { return Group{}, err }
	if description != "" {
		if _, err := tx.Exec(ctx, `UPDATE chats SET description=$2 WHERE id=$1::uuid`, chatID, description); err != nil { return Group{}, err }
	}
	if err := tx.Commit(ctx); err != nil { return Group{}, err }
	return Group{ChatID: chatID, Title: title, Description: description, Role: "owner", MembersCount: 1}, nil
}

func (s *PostgresStore) Get(ctx context.Context, userID, chatID string) (Group, error) {
	var g Group
	const q = `SELECT c.id::text,c.title,COALESCE(c.description,''),cm.role,(SELECT count(*) FROM chat_members x WHERE x.chat_id=c.id AND x.left_at IS NULL) FROM chats c JOIN chat_members cm ON cm.chat_id=c.id WHERE c.id=$1::uuid AND c.kind='group' AND cm.user_id=$2::uuid AND cm.left_at IS NULL`
	if err := s.pool.QueryRow(ctx,q,chatID,userID).Scan(&g.ChatID,&g.Title,&g.Description,&g.Role,&g.MembersCount); err != nil {
		if errors.Is(err,pgx.ErrNoRows){ return Group{},ErrNotFound }
		return Group{},err
	}
	return g,nil
}

func (s *PostgresStore) ListMembers(ctx context.Context,userID,chatID string,limit int)([]Member,error){
	var allowed bool
	if err:=s.pool.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM chat_members WHERE chat_id=$1::uuid AND user_id=$2::uuid AND left_at IS NULL)`,chatID,userID).Scan(&allowed);err!=nil{return nil,err}
	if !allowed{return nil,ErrNotFound}
	rows,err:=s.pool.Query(ctx,`SELECT cm.user_id::text,p.username,p.display_name,cm.role FROM chat_members cm JOIN profiles p ON p.user_id=cm.user_id WHERE cm.chat_id=$1::uuid AND cm.left_at IS NULL ORDER BY CASE cm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,p.display_name LIMIT $2`,chatID,limit)
	if err!=nil{return nil,err};defer rows.Close()
	items:=make([]Member,0,limit)
	for rows.Next(){var m Member;if err:=rows.Scan(&m.UserID,&m.Username,&m.DisplayName,&m.Role);err!=nil{return nil,err};items=append(items,m)}
	return items,rows.Err()
}

func (s *PostgresStore) SetMemberRole(ctx context.Context,actorID,chatID,targetUsername,role string) error{
	tx,err:=s.pool.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var actorRole,targetRole,targetID string
	if err:=tx.QueryRow(ctx,`SELECT role FROM chat_members WHERE chat_id=$1::uuid AND user_id=$2::uuid AND left_at IS NULL FOR UPDATE`,chatID,actorID).Scan(&actorRole);err!=nil{if errors.Is(err,pgx.ErrNoRows){return ErrForbidden};return err}
	if actorRole!="owner"{return ErrForbidden}
	if err:=tx.QueryRow(ctx,`SELECT cm.user_id::text,cm.role FROM chat_members cm JOIN profiles p ON p.user_id=cm.user_id WHERE cm.chat_id=$1::uuid AND lower(p.username)=lower($2) AND cm.left_at IS NULL FOR UPDATE`,chatID,targetUsername).Scan(&targetID,&targetRole);err!=nil{if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};return err}
	if targetRole=="owner"||targetID==actorID{return ErrForbidden}
	if _,err:=tx.Exec(ctx,`UPDATE chat_members SET role=$3 WHERE chat_id=$1::uuid AND user_id=$2::uuid`,chatID,targetID,role);err!=nil{return err}
	return tx.Commit(ctx)
}

func (s *PostgresStore) TransferOwnership(ctx context.Context, actorID, chatID, targetUsername string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return err }
	defer func(){ _ = tx.Rollback(ctx) }()

	var actorRole string
	if err := tx.QueryRow(ctx, `SELECT role FROM chat_members WHERE chat_id=$1::uuid AND user_id=$2::uuid AND left_at IS NULL FOR UPDATE`, chatID, actorID).Scan(&actorRole); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return ErrForbidden }
		return err
	}
	if actorRole != "owner" { return ErrForbidden }

	var targetID, targetRole string
	if err := tx.QueryRow(ctx, `SELECT cm.user_id::text,cm.role FROM chat_members cm JOIN profiles p ON p.user_id=cm.user_id WHERE cm.chat_id=$1::uuid AND lower(p.username)=lower($2) AND cm.left_at IS NULL FOR UPDATE`, chatID, targetUsername).Scan(&targetID,&targetRole); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return ErrNotFound }
		return err
	}
	if targetID == actorID || targetRole == "owner" { return ErrForbidden }

	if _, err := tx.Exec(ctx, `UPDATE chat_members SET role='member' WHERE chat_id=$1::uuid AND user_id=$2::uuid`, chatID, actorID); err != nil { return err }
	if _, err := tx.Exec(ctx, `UPDATE chat_members SET role='owner' WHERE chat_id=$1::uuid AND user_id=$2::uuid`, chatID, targetID); err != nil { return err }
	if _, err := tx.Exec(ctx, `UPDATE chats SET created_by=$2::uuid,updated_at=now() WHERE id=$1::uuid`, chatID, targetID); err != nil { return err }
	return tx.Commit(ctx)
}

func (s *PostgresStore) RemoveMember(ctx context.Context,actorID,chatID,targetUsername string) error{
	tx,err:=s.pool.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var actorRole,targetRole,targetID string
	if err:=tx.QueryRow(ctx,`SELECT role FROM chat_members WHERE chat_id=$1::uuid AND user_id=$2::uuid AND left_at IS NULL FOR UPDATE`,chatID,actorID).Scan(&actorRole);err!=nil{if errors.Is(err,pgx.ErrNoRows){return ErrForbidden};return err}
	if actorRole!="owner"&&actorRole!="admin"{return ErrForbidden}
	if err:=tx.QueryRow(ctx,`SELECT cm.user_id::text,cm.role FROM chat_members cm JOIN profiles p ON p.user_id=cm.user_id WHERE cm.chat_id=$1::uuid AND lower(p.username)=lower($2) AND cm.left_at IS NULL FOR UPDATE`,chatID,targetUsername).Scan(&targetID,&targetRole);err!=nil{if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};return err}
	if targetRole=="owner"||targetID==actorID||(actorRole=="admin"&&targetRole=="admin"){return ErrForbidden}
	if _,err:=tx.Exec(ctx,`UPDATE chat_members SET left_at=now() WHERE chat_id=$1::uuid AND user_id=$2::uuid`,chatID,targetID);err!=nil{return err}
	return tx.Commit(ctx)
}

func (s *PostgresStore) Leave(ctx context.Context,userID,chatID string) error{
	var role string
	if err:=s.pool.QueryRow(ctx,`SELECT role FROM chat_members WHERE chat_id=$1::uuid AND user_id=$2::uuid AND left_at IS NULL`,chatID,userID).Scan(&role);err!=nil{if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};return err}
	if role=="owner"{return ErrForbidden}
	res,err:=s.pool.Exec(ctx,`UPDATE chat_members SET left_at=now() WHERE chat_id=$1::uuid AND user_id=$2::uuid AND left_at IS NULL`,chatID,userID);if err!=nil{return err};if res.RowsAffected()!=1{return ErrNotFound};return nil
}

func (s *PostgresStore) CreateInvite(ctx context.Context,actorID,chatID string,tokenHash []byte,expiresAt *time.Time,maxUses *int) error{
	var role string
	if err:=s.pool.QueryRow(ctx,`SELECT role FROM chat_members WHERE chat_id=$1::uuid AND user_id=$2::uuid AND left_at IS NULL`,chatID,actorID).Scan(&role);err!=nil{if errors.Is(err,pgx.ErrNoRows){return ErrForbidden};return err}
	if role!="owner"&&role!="admin"{return ErrForbidden}
	_,err:=s.pool.Exec(ctx,`INSERT INTO group_invites(chat_id,token_hash,created_by,expires_at,max_uses) VALUES($1::uuid,$2,$3::uuid,$4,$5)`,chatID,tokenHash,actorID,expiresAt,maxUses);return err
}

func (s *PostgresStore) JoinByInvite(ctx context.Context,userID string,tokenHash []byte,now time.Time)(Group,error){
	tx,err:=s.pool.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return Group{},err};defer func(){_=tx.Rollback(ctx)}()
	var chatID string;var expires *time.Time;var maxUses *int;var uses int
	if err:=tx.QueryRow(ctx,`SELECT chat_id::text,expires_at,max_uses,uses_count FROM group_invites WHERE token_hash=$1 AND revoked_at IS NULL FOR UPDATE`,tokenHash).Scan(&chatID,&expires,&maxUses,&uses);err!=nil{if errors.Is(err,pgx.ErrNoRows){return Group{},ErrInviteInvalid};return Group{},err}
	if expires!=nil&&!now.Before(*expires){return Group{},ErrInviteInvalid};if maxUses!=nil&&uses>=*maxUses{return Group{},ErrInviteInvalid}
	var kind string;if err:=tx.QueryRow(ctx,`SELECT kind FROM chats WHERE id=$1::uuid FOR UPDATE`,chatID).Scan(&kind);err!=nil{return Group{},err};if kind!="group"{return Group{},ErrInviteInvalid}
	if _,err:=tx.Exec(ctx,`INSERT INTO chat_members(chat_id,user_id,role,left_at) VALUES($1::uuid,$2::uuid,'member',NULL) ON CONFLICT(chat_id,user_id) DO UPDATE SET left_at=NULL,role='member'`,chatID,userID);err!=nil{return Group{},err}
	if _,err:=tx.Exec(ctx,`UPDATE group_invites SET uses_count=uses_count+1 WHERE token_hash=$1`,tokenHash);err!=nil{return Group{},err}
	var g Group;if err:=tx.QueryRow(ctx,`SELECT c.id::text,c.title,COALESCE(c.description,''),(SELECT count(*) FROM chat_members WHERE chat_id=c.id AND left_at IS NULL) FROM chats c WHERE c.id=$1::uuid`,chatID).Scan(&g.ChatID,&g.Title,&g.Description,&g.MembersCount);err!=nil{return Group{},err};g.Role="member"
	if _,err:=tx.Exec(ctx,`
		INSERT INTO growth_events(event_name,user_id,object_type,object_id)
		VALUES('invite_join',$1::uuid,'group_invite',$2)`,userID,chatID);err!=nil{return Group{},err}
	if err:=tx.Commit(ctx);err!=nil{return Group{},err};return g,nil
}

func (s *PostgresStore) RevokeInvite(ctx context.Context,actorID,chatID string,tokenHash []byte) error{
	var role string;if err:=s.pool.QueryRow(ctx,`SELECT role FROM chat_members WHERE chat_id=$1::uuid AND user_id=$2::uuid AND left_at IS NULL`,chatID,actorID).Scan(&role);err!=nil{if errors.Is(err,pgx.ErrNoRows){return ErrForbidden};return err};if role!="owner"&&role!="admin"{return ErrForbidden}
	res,err:=s.pool.Exec(ctx,`UPDATE group_invites SET revoked_at=now() WHERE chat_id=$1::uuid AND token_hash=$2 AND revoked_at IS NULL`,chatID,tokenHash);if err!=nil{return err};if res.RowsAffected()!=1{return ErrInviteInvalid};return nil
}
