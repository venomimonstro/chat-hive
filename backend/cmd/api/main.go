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
	"github.com/venomimonstro/chat-hive/backend/internal/feed"
	"github.com/venomimonstro/chat-hive/backend/internal/groups"
	"github.com/venomimonstro/chat-hive/backend/internal/httpserver"
	"github.com/venomimonstro/chat-hive/backend/internal/identity"
	"github.com/venomimonstro/chat-hive/backend/internal/messaging"
	"github.com/venomimonstro/chat-hive/backend/internal/onboarding"
	"github.com/venomimonstro/chat-hive/backend/internal/posts"
	"github.com/venomimonstro/chat-hive/backend/internal/search"
	"github.com/venomimonstro/chat-hive/backend/internal/social"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
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

	store := identity.NewPostgresStore(pool)
	var sender identity.MagicLinkSender
	if cfg.Environment == "production" {
		sender, err = identity.NewSMTPMagicLinkSender(identity.SMTPConfig{
			Addr: cfg.SMTPAddr, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
			From: cfg.SMTPFrom, BaseURL: cfg.MagicLinkBaseURL,
		})
	} else {
		sender, err = identity.NewLogMagicLinkSender(logger, cfg.MagicLinkBaseURL)
	}
	if err != nil {
		logger.Error("magic link sender configuration failed", "error", err)
		os.Exit(1)
	}

	identityService := identity.NewService(store, sender)
	identityHTTP := identity.NewHTTPHandler(identityService, logger, cfg.CookieSecure)
	if cfg.YandexClientID != "" {
		identityHTTP.SetYandexOAuth(identity.NewYandexOAuth(store, identity.YandexConfig{
			ClientID: cfg.YandexClientID, RedirectURL: cfg.YandexRedirectURL,
			WebCompleteURL: cfg.WebOrigin + "/auth/yandex-complete",
		}))
	}

	onboardingService := onboarding.NewService(onboarding.NewPostgresStore(pool))
	onboardingHTTP := onboarding.NewHTTPHandler(onboardingService, identityService, logger)

	socialService := social.NewService(social.NewPostgresStore(pool))
	socialHTTP := social.NewHTTPHandler(socialService, identityService, logger)

	messagingService := messaging.NewService(messaging.NewPostgresStore(pool))
	messagingHTTP := messaging.NewHTTPHandler(messagingService, identityService, logger)

	groupsService := groups.NewService(groups.NewPostgresStore(pool))
	groupsHTTP := groups.NewHTTPHandler(groupsService, identityService, logger)

	postsService := posts.NewService(posts.NewPostgresStore(pool))
	postsHTTP := posts.NewHTTPHandler(postsService, identityService, logger)

	feedService := feed.NewService(feed.NewPostgresStore(pool))
	feedHTTP := feed.NewHTTPHandler(feedService, identityService, logger)

	searchService := search.NewService(search.NewPostgresStore(pool))
	searchHTTP := search.NewHTTPHandler(searchService, identityService, logger)

	app := httpserver.New(logger)
	app.Register(identityHTTP.Register)
	app.Register(onboardingHTTP.Register)
	app.Register(socialHTTP.Register)
	app.Register(messagingHTTP.Register)
	app.Register(groupsHTTP.Register)
	app.Register(postsHTTP.Register)
	app.Register(feedHTTP.Register)
	app.Register(searchHTTP.Register)
	app.SetReadiness(pool.Ping)
	app.SetAllowedOrigin(cfg.WebOrigin)

	server := &http.Server{
		Addr: cfg.HTTPAddr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second,
		IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20,
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
