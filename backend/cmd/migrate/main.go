package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type migration struct {
	version  string
	filename string
	path     string
	checksum string
	body     string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	databaseURL := strings.TrimSpace(os.Getenv("CHAT_DATABASE_URL"))
	if databaseURL == "" {
		logger.Error("CHAT_DATABASE_URL is required")
		os.Exit(2)
	}
	dir := strings.TrimSpace(os.Getenv("CHAT_MIGRATIONS_DIR"))
	if dir == "" {
		dir = "./migrations"
		if _, err := os.Stat(dir); err != nil {
			dir = "/migrations"
		}
	}

	migrations, err := loadMigrations(dir)
	if err != nil {
		logger.Error("load migrations failed", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.Error("database pool failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		logger.Error("acquire database connection failed", "error", err)
		os.Exit(1)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext('chat_schema_migrations'))`); err != nil {
		logger.Error("migration lock failed", "error", err)
		os.Exit(1)
	}
	defer func() { _, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext('chat_schema_migrations'))`) }()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			filename TEXT NOT NULL,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		logger.Error("create migration ledger failed", "error", err)
		os.Exit(1)
	}

	for _, item := range migrations {
		var existingChecksum string
		err := conn.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE version=$1`, item.version).Scan(&existingChecksum)
		switch {
		case err == nil:
			if existingChecksum != item.checksum {
				logger.Error("migration checksum drift detected", "version", item.version, "file", item.filename)
				os.Exit(1)
			}
			logger.Info("migration already applied", "version", item.version, "file", item.filename)
			continue
		case err != pgx.ErrNoRows:
			logger.Error("read migration ledger failed", "error", err, "version", item.version)
			os.Exit(1)
		}

		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			logger.Error("begin migration transaction failed", "error", err, "version", item.version)
			os.Exit(1)
		}
		if _, err := tx.Exec(ctx, item.body); err != nil {
			_ = tx.Rollback(ctx)
			logger.Error("migration failed", "error", err, "version", item.version, "file", item.filename)
			os.Exit(1)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version,filename,checksum) VALUES($1,$2,$3)`, item.version, item.filename, item.checksum); err != nil {
			_ = tx.Rollback(ctx)
			logger.Error("migration ledger write failed", "error", err, "version", item.version)
			os.Exit(1)
		}
		if err := tx.Commit(ctx); err != nil {
			logger.Error("migration commit failed", "error", err, "version", item.version)
			os.Exit(1)
		}
		logger.Info("migration applied", "version", item.version, "file", item.filename)
	}

	logger.Info("database migrations complete", "count", len(migrations))
}

func loadMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory %s: %w", dir, err)
	}
	seen := map[string]string{}
	items := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) != 2 || len(parts[0]) != 6 {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		for _, char := range parts[0] {
			if char < '0' || char > '9' {
				return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
			}
		}
		if previous, exists := seen[parts[0]]; exists {
			return nil, fmt.Errorf("duplicate migration version %s: %s and %s", parts[0], previous, entry.Name())
		}
		seen[parts[0]] = entry.Name()
		path := filepath.Join(dir, entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		hash := sha256.Sum256(body)
		items = append(items, migration{
			version: parts[0], filename: entry.Name(), path: path,
			checksum: hex.EncodeToString(hash[:]), body: string(body),
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no .up.sql migrations found in %s", dir)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].version < items[j].version })
	return items, nil
}
