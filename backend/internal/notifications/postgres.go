package notifications

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) List(ctx context.Context, userID string, limit int) ([]Notification, int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text,kind,actor_id::text,entity_type,entity_id,title,body,read_at,created_at
		FROM notifications
		WHERE user_id=$1::uuid
		ORDER BY created_at DESC,id DESC
		LIMIT $2`, userID, limit)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	items := make([]Notification, 0, limit)
	for rows.Next() {
		var item Notification
		if err := rows.Scan(&item.ID,&item.Kind,&item.ActorID,&item.EntityType,&item.EntityID,&item.Title,&item.Body,&item.ReadAt,&item.CreatedAt); err != nil { return nil,0,err }
		items = append(items,item)
	}
	if err := rows.Err(); err != nil { return nil,0,err }
	var unread int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*)::bigint FROM notifications WHERE user_id=$1::uuid AND read_at IS NULL`, userID).Scan(&unread); err != nil { return nil,0,err }
	return items,unread,nil
}

func (s *PostgresStore) MarkRead(ctx context.Context, userID, notificationID string) error {
	result,err:=s.pool.Exec(ctx,`UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE id=$1::uuid AND user_id=$2::uuid`,notificationID,userID)
	if err!=nil{return err}
	if result.RowsAffected()!=1{return ErrNotFound}
	return nil
}

func (s *PostgresStore) MarkAllRead(ctx context.Context, userID string) error {
	_,err:=s.pool.Exec(ctx,`UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE user_id=$1::uuid AND read_at IS NULL`,userID)
	return err
}

func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
