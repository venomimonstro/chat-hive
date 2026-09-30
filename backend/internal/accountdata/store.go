package accountdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type profileExport struct { UserID string `json:"user_id"`; Status string `json:"status"`; Username string `json:"username"`; DisplayName string `json:"display_name"`; Bio string `json:"bio"`; Interests []string `json:"interests"`; CreatedAt string `json:"created_at"` }
type identityExport struct { Provider string `json:"provider"`; Subject string `json:"subject"`; Email string `json:"email,omitempty"`; Verified string `json:"verified_at,omitempty"` }
type relationExport struct { Username string `json:"username"`; Name string `json:"display_name"` }
type postExport struct { ID string `json:"id"`; Kind string `json:"kind"`; Body string `json:"body"`; Visibility string `json:"visibility"`; CreatedAt string `json:"created_at"`; UpdatedAt string `json:"updated_at"` }
type replyExport struct { ID string `json:"id"`; PostID string `json:"post_id"`; Body string `json:"body"`; CreatedAt string `json:"created_at"` }
type messageExport struct { ID string `json:"id"`; ChatID string `json:"chat_id"`; Sequence int64 `json:"sequence"`; Type string `json:"type"`; Body string `json:"body"`; CreatedAt string `json:"created_at"`; EditedAt string `json:"edited_at,omitempty"`; DeletedAt string `json:"deleted_at,omitempty"` }
type membershipExport struct { Kind string `json:"kind"`; ID string `json:"id"`; Slug string `json:"slug,omitempty"`; Title string `json:"title"`; Role string `json:"role,omitempty"` }
type sessionExport struct { ID string `json:"id"`; UserAgent string `json:"user_agent"`; LastIP string `json:"last_ip,omitempty"`; CreatedAt string `json:"created_at"`; LastSeen string `json:"last_seen_at"`; ExpiresAt string `json:"expires_at"`; RevokedAt string `json:"revoked_at,omitempty"` }

func (s *Store) WriteExport(ctx context.Context, userID string, w io.Writer) error {
	userID = strings.TrimSpace(userID)
	if userID == "" { return fmt.Errorf("empty user id") }
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)

	var profile profileExport
	var createdAt time.Time
	if err := s.pool.QueryRow(ctx, `SELECT u.id::text,u.status,COALESCE(p.username,''),COALESCE(p.display_name,''),COALESCE(p.bio,''),u.created_at FROM users u LEFT JOIN profiles p ON p.user_id=u.id WHERE u.id=$1::uuid`, userID).Scan(&profile.UserID,&profile.Status,&profile.Username,&profile.DisplayName,&profile.Bio,&createdAt); err != nil { return err }
	profile.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	rows, err := s.pool.Query(ctx, `SELECT interest_slug FROM user_interests WHERE user_id=$1::uuid ORDER BY interest_slug`, userID)
	if err != nil { return err }
	for rows.Next() { var value string; if err:=rows.Scan(&value);err!=nil{rows.Close();return err};profile.Interests=append(profile.Interests,value) }
	if err:=rows.Err();err!=nil{rows.Close();return err};rows.Close()

	if _,err:=io.WriteString(w,"{\"version\":1,\"generated_at\":");err!=nil{return err}
	if err:=enc.Encode(time.Now().UTC().Format(time.RFC3339Nano));err!=nil{return err}
	if _,err:=io.WriteString(w,",\"profile\":");err!=nil{return err}
	if err:=enc.Encode(profile);err!=nil{return err}
	if err:=s.writeIdentities(ctx,userID,w,enc);err!=nil{return err}
	if err:=s.writeRelations(ctx,userID,"followers",w,enc);err!=nil{return err}
	if err:=s.writeRelations(ctx,userID,"following",w,enc);err!=nil{return err}
	if err:=s.writePosts(ctx,userID,w,enc);err!=nil{return err}
	if err:=s.writeReplies(ctx,userID,w,enc);err!=nil{return err}
	if err:=s.writeMessages(ctx,userID,w,enc);err!=nil{return err}
	if err:=s.writeMemberships(ctx,userID,w,enc);err!=nil{return err}
	if err:=s.writeSessions(ctx,userID,w,enc);err!=nil{return err}
	_,err=io.WriteString(w,"}\n")
	return err
}

func writeArray[T any](w io.Writer, enc *json.Encoder, name string, items []T) error {
	if _,err:=io.WriteString(w,",\""+name+"\":[");err!=nil{return err}
	for i,item:=range items { if i>0{if _,err:=io.WriteString(w,",");err!=nil{return err}};if err:=enc.Encode(item);err!=nil{return err} }
	_,err:=io.WriteString(w,"]")
	return err
}

func (s *Store) writeIdentities(ctx context.Context,userID string,w io.Writer,enc *json.Encoder)error{
	rows,err:=s.pool.Query(ctx,`SELECT provider,provider_subject,COALESCE(email_normalized,''),verified_at FROM user_identities WHERE user_id=$1::uuid ORDER BY provider,created_at`,userID);if err!=nil{return err};defer rows.Close();items:=[]identityExport{}
	for rows.Next(){var item identityExport;var verified *time.Time;if err:=rows.Scan(&item.Provider,&item.Subject,&item.Email,&verified);err!=nil{return err};if verified!=nil{item.Verified=verified.UTC().Format(time.RFC3339Nano)};items=append(items,item)}
	if err:=rows.Err();err!=nil{return err};return writeArray(w,enc,"identities",items)
}
func (s *Store) writeRelations(ctx context.Context,userID,kind string,w io.Writer,enc *json.Encoder)error{
	query:=`SELECT p.username,p.display_name FROM follows f JOIN profiles p ON p.user_id=f.follower_id WHERE f.followed_id=$1::uuid ORDER BY f.created_at`;if kind=="following"{query=`SELECT p.username,p.display_name FROM follows f JOIN profiles p ON p.user_id=f.followed_id WHERE f.follower_id=$1::uuid ORDER BY f.created_at`}
	rows,err:=s.pool.Query(ctx,query,userID);if err!=nil{return err};defer rows.Close();items:=[]relationExport{}
	for rows.Next(){var item relationExport;if err:=rows.Scan(&item.Username,&item.Name);err!=nil{return err};items=append(items,item)}
	if err:=rows.Err();err!=nil{return err};return writeArray(w,enc,kind,items)
}
func (s *Store) writePosts(ctx context.Context,userID string,w io.Writer,enc *json.Encoder)error{
	rows,err:=s.pool.Query(ctx,`SELECT id::text,kind,body,visibility,created_at,updated_at FROM posts WHERE author_id=$1::uuid ORDER BY created_at`,userID);if err!=nil{return err};defer rows.Close();items:=[]postExport{}
	for rows.Next(){var item postExport;var created,updated time.Time;if err:=rows.Scan(&item.ID,&item.Kind,&item.Body,&item.Visibility,&created,&updated);err!=nil{return err};item.CreatedAt=created.UTC().Format(time.RFC3339Nano);item.UpdatedAt=updated.UTC().Format(time.RFC3339Nano);items=append(items,item)}
	if err:=rows.Err();err!=nil{return err};return writeArray(w,enc,"posts",items)
}
func (s *Store) writeReplies(ctx context.Context,userID string,w io.Writer,enc *json.Encoder)error{
	rows,err:=s.pool.Query(ctx,`SELECT id::text,post_id::text,body,created_at FROM post_replies WHERE author_id=$1::uuid ORDER BY created_at`,userID);if err!=nil{return err};defer rows.Close();items:=[]replyExport{}
	for rows.Next(){var item replyExport;var created time.Time;if err:=rows.Scan(&item.ID,&item.PostID,&item.Body,&created);err!=nil{return err};item.CreatedAt=created.UTC().Format(time.RFC3339Nano);items=append(items,item)}
	if err:=rows.Err();err!=nil{return err};return writeArray(w,enc,"post_replies",items)
}
func (s *Store) writeMessages(ctx context.Context,userID string,w io.Writer,enc *json.Encoder)error{
	if _,err:=io.WriteString(w,",\"authored_messages\":[");err!=nil{return err}
	rows,err:=s.pool.Query(ctx,`SELECT id::text,chat_id::text,sequence,type,body,created_at,edited_at,deleted_at FROM messages WHERE sender_id=$1::uuid ORDER BY created_at,id`,userID);if err!=nil{return err};defer rows.Close();first:=true
	for rows.Next(){var item messageExport;var created time.Time;var edited,deleted *time.Time;if err:=rows.Scan(&item.ID,&item.ChatID,&item.Sequence,&item.Type,&item.Body,&created,&edited,&deleted);err!=nil{return err};item.CreatedAt=created.UTC().Format(time.RFC3339Nano);if edited!=nil{item.EditedAt=edited.UTC().Format(time.RFC3339Nano)};if deleted!=nil{item.DeletedAt=deleted.UTC().Format(time.RFC3339Nano)};if !first{if _,err:=io.WriteString(w,",");err!=nil{return err}};first=false;if err:=enc.Encode(item);err!=nil{return err}}
	if err:=rows.Err();err!=nil{return err};_,err=io.WriteString(w,"]");return err
}
func (s *Store) writeMemberships(ctx context.Context,userID string,w io.Writer,enc *json.Encoder)error{
	rows,err:=s.pool.Query(ctx,`SELECT 'group',c.id::text,'',c.title,cm.role FROM chat_members cm JOIN chats c ON c.id=cm.chat_id WHERE cm.user_id=$1::uuid AND cm.left_at IS NULL AND c.kind='group' UNION ALL SELECT 'community',c.id::text,c.slug,c.title,cm.role FROM community_members cm JOIN communities c ON c.id=cm.community_id WHERE cm.user_id=$1::uuid AND cm.left_at IS NULL UNION ALL SELECT 'channel',c.id::text,c.slug,c.title,'subscriber' FROM channel_subscribers cs JOIN channels c ON c.id=cs.channel_id WHERE cs.user_id=$1::uuid ORDER BY 1,4`,userID);if err!=nil{return err};defer rows.Close();items:=[]membershipExport{}
	for rows.Next(){var item membershipExport;if err:=rows.Scan(&item.Kind,&item.ID,&item.Slug,&item.Title,&item.Role);err!=nil{return err};items=append(items,item)}
	if err:=rows.Err();err!=nil{return err};return writeArray(w,enc,"memberships",items)
}
func (s *Store) writeSessions(ctx context.Context,userID string,w io.Writer,enc *json.Encoder)error{
	rows,err:=s.pool.Query(ctx,`SELECT id::text,user_agent,COALESCE(last_ip::text,''),created_at,last_seen_at,expires_at,revoked_at FROM sessions WHERE user_id=$1::uuid ORDER BY created_at`,userID);if err!=nil{return err};defer rows.Close();items:=[]sessionExport{}
	for rows.Next(){var item sessionExport;var created,lastSeen,expires time.Time;var revoked *time.Time;if err:=rows.Scan(&item.ID,&item.UserAgent,&item.LastIP,&created,&lastSeen,&expires,&revoked);err!=nil{return err};item.CreatedAt=created.UTC().Format(time.RFC3339Nano);item.LastSeen=lastSeen.UTC().Format(time.RFC3339Nano);item.ExpiresAt=expires.UTC().Format(time.RFC3339Nano);if revoked!=nil{item.RevokedAt=revoked.UTC().Format(time.RFC3339Nano)};items=append(items,item)}
	if err:=rows.Err();err!=nil{return err};return writeArray(w,enc,"sessions",items)
}

func (s *Store) DeleteAccount(ctx context.Context,userID string)error{
	tx,err:=s.pool.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var status string
	if err:=tx.QueryRow(ctx,`SELECT status FROM users WHERE id=$1::uuid FOR UPDATE`,userID).Scan(&status);err!=nil{return err}
	if status=="deleted"{return nil}
	anonymousUsername:="deleted_"+strings.ReplaceAll(userID,"-","")[:16]
	if _,err:=tx.Exec(ctx,`UPDATE users SET status='deleted',updated_at=now() WHERE id=$1::uuid`,userID);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`UPDATE profiles SET username=$2,display_name='Deleted user',bio='',avatar_media_id=NULL,onboarding_completed_at=NULL,updated_at=now() WHERE user_id=$1::uuid`,userID,anonymousUsername);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`DELETE FROM user_identities WHERE user_id=$1::uuid`,userID);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`DELETE FROM user_interests WHERE user_id=$1::uuid`,userID);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`DELETE FROM follows WHERE follower_id=$1::uuid OR followed_id=$1::uuid`,userID);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`DELETE FROM user_blocks WHERE blocker_id=$1::uuid OR blocked_id=$1::uuid`,userID);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`UPDATE sessions SET revoked_at=COALESCE(revoked_at,now()),access_token_hash=NULL WHERE user_id=$1::uuid`,userID);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`DELETE FROM push_subscriptions WHERE user_id=$1::uuid`,userID);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`INSERT INTO security_events(event_type,severity,user_id,subject_type,subject_id,metadata) VALUES('account_product_deleted','medium',$1::uuid,'user',$1,'{}'::jsonb)`,userID);err!=nil{return err}
	return tx.Commit(ctx)
}
