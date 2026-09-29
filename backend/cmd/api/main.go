package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/chat-hive/backend/internal/config"
	"github.com/venomimonstro/chat-hive/backend/internal/httpserver"
	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}
	if cfg.Environment == "production" {
		// Sprint 03 intentionally fails closed until a real production mail transport is configured.
		logger.Error("production magic-link transport is not configured")
		os.Exit(1)
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	pool, err := pgxpool.New(startupCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database pool creation failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(startupCtx); err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}

	sender, err := identity.NewLogMagicLinkSender(logger, cfg.MagicLinkBaseURL)
	if err != nil {
		logger.Error("magic link sender configuration failed", "error", err)
		os.Exit(1)
	}
	identityService := identity.NewService(identity.NewPostgresStore(pool), sender)
	identityHTTP := identity.NewHTTPHandler(identityService, logger, cfg.CookieSecure)

	app := httpserver.New(logger)
	app.Register(identityHTTP.Register)
	app.SetReadiness(pool.Ping)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("CHAT API starting", "addr", cfg.HTTPAddr, "environment", cfg.Environment)
		serverErr <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	case <-signalCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		logger.Info("CHAT API stopped")
	}
}
