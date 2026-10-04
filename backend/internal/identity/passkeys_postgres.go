package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) LoadPasskeyUser(ctx context.Context, userID string) (passkeyUser, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return passkeyUser{}, ErrPasskeyUnavailable
	}
	var name, displayName string
	err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(NULLIF(p.username,''), NULLIF(ui.email_normalized,''), u.id::text),
			COALESCE(NULLIF(p.display_name,''), NULLIF(p.username,''), NULLIF(ui.email_normalized,''), 'CHAT user')
		FROM users u
		LEFT JOIN profiles p ON p.user_id=u.id
		LEFT JOIN LATERAL (
			SELECT email_normalized
			FROM user_identities
			WHERE user_id=u.id AND email_normalized IS NOT NULL
			ORDER BY verified_at DESC NULLS LAST,created_at ASC
			LIMIT 1
		) ui ON true
		WHERE u.id=$1::uuid AND u.status='active'`, userID).Scan(&name, &displayName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return passkeyUser{}, ErrPasskeyUnavailable }
		return passkeyUser{}, err
	}
	credentials, err := s.ListPasskeyCredentials(ctx, userID)
	if err != nil { return passkeyUser{}, err }
	return passkeyUser{ID:userID,Name:name,DisplayName:displayName,Credentials:credentials}, nil
}

func (s *PostgresStore) LoadPasskeyUserByHandle(ctx context.Context, handle []byte) (passkeyUser, error) {
	value := string(handle)
	if _, err := uuid.Parse(value); err != nil {
		return passkeyUser{}, ErrPasskeyUnavailable
	}
	return s.LoadPasskeyUser(ctx, value)
}

func (s *PostgresStore) ListPasskeyCredentials(ctx context.Context, userID string) ([]webauthn.Credential, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT credential
		FROM passkey_credentials
		WHERE user_id=$1::uuid
		ORDER BY created_at`, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]webauthn.Credential, 0, 4)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil { return nil, err }
		var credential webauthn.Credential
		if err := json.Unmarshal(raw, &credential); err != nil { return nil, fmt.Errorf("decode passkey credential: %w", err) }
		items = append(items, credential)
	}
	return items, rows.Err()
}

func (s *PostgresStore) SavePasskeyCredential(ctx context.Context, userID string, credential webauthn.Credential) error {
	raw, err := json.Marshal(credential)
	if err != nil { return err }
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return err }
	defer func(){ _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, `
		INSERT INTO passkey_credentials(credential_id,user_id,credential)
		VALUES($1,$2::uuid,$3::jsonb)
		ON CONFLICT (credential_id) DO NOTHING`, credential.ID,userID,raw)
	if err != nil { return err }
	if result.RowsAffected()!=1 { return ErrPasskeyUnavailable }

	if _, err := tx.Exec(ctx, `
		INSERT INTO security_events(event_type,severity,user_id,subject_type,subject_id)
		VALUES('passkey_registered','info',$1::uuid,'user',$1)`, userID); err != nil { return err }

	return tx.Commit(ctx)
}

func (s *PostgresStore) UpdatePasskeyCredential(ctx context.Context, userID string, credential webauthn.Credential) error {
	raw, err := json.Marshal(credential)
	if err != nil { return err }
	result, err := s.pool.Exec(ctx, `
		UPDATE passkey_credentials
		SET credential=$3::jsonb,last_used_at=now(),updated_at=now()
		WHERE credential_id=$1 AND user_id=$2::uuid`, credential.ID,userID,raw)
	if err != nil { return err }
	if result.RowsAffected()!=1 { return ErrPasskeyUnavailable }
	return nil
}

func (s *PostgresStore) CreatePasskeyCeremony(ctx context.Context, kind, userID string, session webauthn.SessionData) (string, error) {
	raw, err := json.Marshal(session)
	if err != nil { return "", err }
	expires := session.Expires
	if expires.IsZero() { expires = time.Now().UTC().Add(2*time.Minute) }
	var id string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO passkey_ceremonies(kind,user_id,session_data,expires_at)
		VALUES($1,NULLIF($2,'')::uuid,$3::jsonb,$4)
		RETURNING id::text`,kind,userID,raw,expires).Scan(&id)
	return id, err
}

func (s *PostgresStore) ConsumePasskeyCeremony(ctx context.Context, ceremonyID, kind string, now time.Time) (string, webauthn.SessionData, error) {
	if _, err := uuid.Parse(ceremonyID); err != nil {
		return "", webauthn.SessionData{}, ErrPasskeyInvalidCeremony
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return "", webauthn.SessionData{}, err }
	defer func(){ _ = tx.Rollback(ctx) }()
	var userID string
	var raw []byte
	var expires time.Time
	var consumed *time.Time
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(user_id::text,''),session_data,expires_at,consumed_at
		FROM passkey_ceremonies
		WHERE id=$1::uuid AND kind=$2
		FOR UPDATE`,ceremonyID,kind).Scan(&userID,&raw,&expires,&consumed)
	if err != nil {
		if errors.Is(err,pgx.ErrNoRows) { return "",webauthn.SessionData{},ErrPasskeyInvalidCeremony }
		return "",webauthn.SessionData{},err
	}
	if consumed != nil || !now.Before(expires) {
		return "",webauthn.SessionData{},ErrPasskeyInvalidCeremony
	}
	if _,err:=tx.Exec(ctx,`UPDATE passkey_ceremonies SET consumed_at=$2 WHERE id=$1::uuid`,ceremonyID,now);err!=nil{
		return "",webauthn.SessionData{},err
	}
	var session webauthn.SessionData
	if err:=json.Unmarshal(raw,&session);err!=nil{return "",webauthn.SessionData{},err}
	if err:=tx.Commit(ctx);err!=nil{return "",webauthn.SessionData{},err}
	return userID,session,nil
}

func (s *PostgresStore) RecordPasskeyCloneWarning(ctx context.Context, userID string, credentialID []byte, sourceIP string) error {
	_,err:=s.pool.Exec(ctx,`
		INSERT INTO security_events(event_type,severity,user_id,source_ip,subject_type,subject_id,metadata)
		VALUES('passkey_clone_warning','high',$1::uuid,NULLIF($2,'')::inet,'user',$1,
		       jsonb_build_object('credential_id',$3))`,
		userID,sourceIP,fmt.Sprintf("%x",credentialID))
	return err
}


func (s *PostgresStore) ListPasskeyInfo(ctx context.Context, userID string) ([]PasskeyInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT credential_id,label,created_at,last_used_at
		FROM passkey_credentials
		WHERE user_id=$1::uuid
		ORDER BY created_at DESC`, userID)
	if err != nil { return nil, err }
	defer rows.Close()

	items := make([]PasskeyInfo, 0, 4)
	for rows.Next() {
		var credentialID []byte
		var item PasskeyInfo
		if err := rows.Scan(&credentialID,&item.Label,&item.CreatedAt,&item.LastUsedAt); err != nil { return nil, err }
		item.CredentialID = base64.RawURLEncoding.EncodeToString(credentialID)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) DeletePasskeyCredential(ctx context.Context, userID, encodedCredentialID string) error {
	encodedCredentialID = strings.TrimSpace(encodedCredentialID)
	if encodedCredentialID == "" || len(encodedCredentialID) > 2048 { return ErrPasskeyUnavailable }
	credentialID, err := base64.RawURLEncoding.DecodeString(encodedCredentialID)
	if err != nil || len(credentialID) == 0 { return ErrPasskeyUnavailable }
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return err }
	defer func(){ _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, `
		DELETE FROM passkey_credentials
		WHERE credential_id=$1 AND user_id=$2::uuid`, credentialID,userID)
	if err != nil { return err }
	if result.RowsAffected()!=1 { return ErrPasskeyUnavailable }

	if _, err := tx.Exec(ctx, `
		INSERT INTO security_events(event_type,severity,user_id,subject_type,subject_id)
		VALUES('passkey_deleted','medium',$1::uuid,'user',$1)`, userID); err != nil { return err }

	return tx.Commit(ctx)
}
