package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Listener struct {
	pool   *pgxpool.Pool
	hub    *Hub
	logger *slog.Logger
}

func NewListener(pool *pgxpool.Pool, hub *Hub, logger *slog.Logger) *Listener {
	return &Listener{pool: pool, hub: hub, logger: logger}
}

func (l *Listener) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		if err := l.listenOnce(ctx); err != nil && ctx.Err() == nil {
			l.logger.Warn("realtime listener disconnected", "error", err, "retry_in", backoff)
			select {
			case <-ctx.Done(): return
			case <-time.After(backoff):
			}
			if backoff < 15*time.Second { backoff *= 2 }
			continue
		}
		backoff = time.Second
	}
}

func (l *Listener) listenOnce(ctx context.Context) error {
	conn, err := l.pool.Acquire(ctx)
	if err != nil { return err }
	defer conn.Release()
	if _, err := conn.Exec(ctx, `LISTEN chat_events`); err != nil { return err }
	defer func(){ _, _ = conn.Exec(context.Background(), `UNLISTEN chat_events`) }()

	for ctx.Err() == nil {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil { return err }
		var event Event
		if err := json.Unmarshal([]byte(notification.Payload), &event); err != nil {
			l.logger.Warn("invalid realtime payload", "error", err)
			continue
		}
		if event.ChatID == "" || event.Type == "" { continue }
		recipients, err := l.chatRecipients(ctx, event.ChatID)
		if err != nil {
			l.logger.Warn("realtime recipient lookup failed", "chat_id", event.ChatID, "error", err)
			continue
		}
		for _, userID := range recipients { l.hub.Publish(userID,event) }
	}
	return ctx.Err()
}

func (l *Listener) chatRecipients(ctx context.Context, chatID string) ([]string,error) {
	rows, err := l.pool.Query(ctx, `SELECT user_id::text FROM chat_members WHERE chat_id=$1::uuid AND left_at IS NULL`, chatID)
	if err != nil { return nil,err }
	defer rows.Close()
	items := make([]string,0,8)
	for rows.Next(){ var userID string; if err:=rows.Scan(&userID);err!=nil{return nil,err};items=append(items,userID) }
	return items,rows.Err()
}
