package media

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Create(ctx context.Context, object StoredObject) (Object, error) {
	var result Object
	const query = `
		INSERT INTO media_objects (owner_id,media_type,storage_key,mime_type,byte_size,width,height,sha256,state)
		VALUES ($1::uuid,'image',$2,$3,$4,$5,$6,$7,'ready')
		RETURNING id::text,mime_type,byte_size,width,height`
	if err := s.pool.QueryRow(ctx, query,
		object.OwnerID, object.StorageKey, object.MimeType, object.ByteSize, object.Width, object.Height, object.SHA256,
	).Scan(&result.ID, &result.MimeType, &result.ByteSize, &result.Width, &result.Height); err != nil {
		return Object{}, err
	}
	return result, nil
}

func (s *PostgresStore) Resolve(ctx context.Context, viewerID, mediaID string) (StoredObject, bool, error) {
	if mediaID == "" {
		return StoredObject{}, false, ErrNotFound
	}
	const query = `
		SELECT m.id::text,m.owner_id::text,m.storage_key,m.mime_type,m.byte_size,m.width,m.height,m.sha256,
		       (
		         m.owner_id = NULLIF($2,'')::uuid
		         OR EXISTS(
		           SELECT 1 FROM post_media pm
		           JOIN posts p ON p.id=pm.post_id
		           WHERE pm.media_id=m.id AND p.deleted_at IS NULL AND p.visibility='public'
		         )
		         OR EXISTS(
		           SELECT 1 FROM message_attachments ma
		           JOIN messages msg ON msg.id=ma.message_id
		           JOIN chat_members cm ON cm.chat_id=msg.chat_id
		           WHERE ma.media_id=m.id AND cm.user_id=NULLIF($2,'')::uuid AND cm.left_at IS NULL
		         )
		       ) AS allowed
		FROM media_objects m
		WHERE m.id=$1::uuid AND m.deleted_at IS NULL AND m.state='ready'`
	var object StoredObject
	var allowed bool
	err := s.pool.QueryRow(ctx, query, mediaID, viewerID).Scan(
		&object.ID, &object.OwnerID, &object.StorageKey, &object.MimeType, &object.ByteSize,
		&object.Width, &object.Height, &object.SHA256, &allowed,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredObject{}, false, ErrNotFound
	}
	if err != nil {
		return StoredObject{}, false, err
	}
	return object, allowed, nil
}
