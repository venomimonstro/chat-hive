package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var allowedRoles = map[string]struct{}{
	"support": {}, "moderator": {}, "senior_moderator": {}, "security": {},
	"legal": {}, "system_admin": {}, "owner": {},
}

func main() {
	var email string
	var role string
	var reason string
	flag.StringVar(&email, "email", "", "verified CHAT account email")
	flag.StringVar(&role, "role", "", "admin role to grant")
	flag.StringVar(&reason, "reason", "", "audit reason for this grant")
	flag.Parse()

	email = strings.ToLower(strings.TrimSpace(email))
	role = strings.ToLower(strings.TrimSpace(role))
	reason = strings.TrimSpace(reason)

	if email == "" || !strings.Contains(email, "@") {
		fatal("valid -email is required")
	}
	if _, ok := allowedRoles[role]; !ok {
		fatal("invalid -role")
	}
	if len([]rune(reason)) < 3 || len([]rune(reason)) > 500 {
		fatal("-reason must be 3..500 characters")
	}

	env := strings.ToLower(strings.TrimSpace(os.Getenv("CHAT_ENV")))
	if env == "production" && os.Getenv("CHAT_ADMIN_BOOTSTRAP_CONFIRM") != "I_UNDERSTAND" {
		fatal("production grant requires CHAT_ADMIN_BOOTSTRAP_CONFIRM=I_UNDERSTAND")
	}

	databaseURL := strings.TrimSpace(os.Getenv("CHAT_DATABASE_URL"))
	if databaseURL == "" {
		fatal("CHAT_DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fatal("database pool: " + err.Error())
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatal("database unavailable: " + err.Error())
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		fatal("begin transaction: " + err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID string
	err = tx.QueryRow(ctx, `
		SELECT ui.user_id::text
		FROM user_identities ui
		WHERE ui.provider='email'
		  AND ui.email_normalized=$1
		  AND ui.verified_at IS NOT NULL
		LIMIT 1
		FOR SHARE`, email).Scan(&userID)
	if err == pgx.ErrNoRows {
		fatal("verified CHAT account not found")
	}
	if err != nil {
		fatal("lookup account: " + err.Error())
	}

	if role == "owner" || role == "security" {
		var passkeys int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM passkey_credentials WHERE user_id=$1::uuid`, userID).Scan(&passkeys); err != nil {
			fatal("check passkey coverage: " + err.Error())
		}
		if passkeys == 0 {
			fatal("owner/security role requires at least one existing passkey")
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_users(user_id,status)
		VALUES($1::uuid,'active')
		ON CONFLICT (user_id) DO UPDATE SET status='active',updated_at=now()`, userID); err != nil {
		fatal("activate admin: " + err.Error())
	}

	result, err := tx.Exec(ctx, `
		INSERT INTO admin_user_roles(user_id,role)
		VALUES($1::uuid,$2)
		ON CONFLICT (user_id,role) DO NOTHING`, userID, role)
	if err != nil {
		fatal("grant role: " + err.Error())
	}

	action := "admin_role_granted"
	if result.RowsAffected() == 0 {
		action = "admin_role_grant_confirmed"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_events(actor_user_id,actor_role,action,target_type,target_id,reason)
		VALUES(NULL,'system_bootstrap',$1,'user',$2,$3)`, action, userID, reason); err != nil {
		fatal("write audit event: " + err.Error())
	}

	if err := tx.Commit(ctx); err != nil {
		fatal("commit: " + err.Error())
	}

	slog.Info("admin role grant complete", "user_id", userID, "role", role, "changed", result.RowsAffected() == 1)
	fmt.Printf("granted role %s to verified account %s (user %s)\n", role, email, userID)
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "adminctl:", message)
	os.Exit(1)
}
