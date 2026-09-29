package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Environment      string
	HTTPAddr         string
	DatabaseURL      string
	RedisAddr        string
	NATSURL          string
	MagicLinkBaseURL string
	CookieSecure     bool
}

func Load() (Config, error) {
	environment := strings.ToLower(strings.TrimSpace(env("CHAT_ENV", "development")))
	cfg := Config{
		Environment:      environment,
		HTTPAddr:         env("CHAT_HTTP_ADDR", ":8080"),
		DatabaseURL:      env("CHAT_DATABASE_URL", "postgres://chat:chat@localhost:5432/chat?sslmode=disable"),
		RedisAddr:        env("CHAT_REDIS_ADDR", "localhost:6379"),
		NATSURL:          env("CHAT_NATS_URL", "nats://localhost:4222"),
		MagicLinkBaseURL: env("CHAT_MAGIC_LINK_BASE_URL", "http://localhost:3000/auth/complete"),
		CookieSecure:     environment != "development" && environment != "test",
	}

	if strings.TrimSpace(cfg.HTTPAddr) == "" {
		return Config{}, fmt.Errorf("CHAT_HTTP_ADDR must not be empty")
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return Config{}, fmt.Errorf("CHAT_DATABASE_URL must not be empty")
	}
	if cfg.Environment == "production" && strings.HasPrefix(cfg.MagicLinkBaseURL, "http://") {
		return Config{}, fmt.Errorf("production magic link base URL must use HTTPS")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
