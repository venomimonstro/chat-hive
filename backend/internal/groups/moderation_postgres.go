package groups

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) DeleteMessageAsModerator(ctx context.Context, actorID, chatID, messageID string) error {
	tx,err:=s.pool.BeginTx(ctx,pgx.TxOptions{})
	if err!=nil{return err}
	defer func(){_=tx.Rollback(ctx)}()
	var role string
	if err:=tx.QueryRow(ctx,`SELECT role FROM chat_members WHERE chat_id=$1::uuid AND user_id=$2::uuid AND left_at IS NULL FOR UPDATE`,chatID,actorID).Scan(&role);err!=nil{
		if err==pgx.ErrNoRows{return ErrForbidden}
		return err
	}
	if role!="owner"&&role!="admin"{return ErrForbidden}
	result,err:=tx.Exec(ctx,`
		UPDATE messages m
		SET body='',deleted_at=COALESCE(deleted_at,now()),edited_at=COALESCE(edited_at,now())
		FROM chats c
		WHERE m.id=$1::uuid AND m.chat_id=$2::uuid AND c.id=m.chat_id AND c.kind='group' AND m.deleted_at IS NULL`,messageID,chatID)
	if err!=nil{return err}
	if result.RowsAffected()!=1{return ErrNotFound}
	return tx.Commit(ctx)
}
