package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Environment string
	HTTPAddr    string
	DatabaseURL string
	RedisAddr   string
	NATSURL     string
}

func Load() (Config, error) {
	cfg := Config{
		Environment: env("CHAT_ENV", "development"),
		HTTPAddr:    env("CHAT_HTTP_ADDR", ":8080"),
		DatabaseURL: env("CHAT_DATABASE_URL", "postgres://chat:chat@localhost:5432/chat?sslmode=disable"),
		RedisAddr:   env("CHAT_REDIS_ADDR", "localhost:6379"),
		NATSURL:     env("CHAT_NATS_URL", "nats://localhost:4222"),
	}

	if strings.TrimSpace(cfg.HTTPAddr) == "" {
		return Config{}, fmt.Errorf("CHAT_HTTP_ADDR must not be empty")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
