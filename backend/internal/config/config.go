package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Environment       string
	HTTPAddr          string
	DatabaseURL       string
	RedisAddr         string
	NATSURL           string
	MagicLinkBaseURL  string
	WebOrigin         string
	YandexClientID    string
	YandexRedirectURL string
	CookieSecure      bool
}

func Load() (Config, error) {
	environment := strings.ToLower(strings.TrimSpace(env("CHAT_ENV", "development")))
	cfg := Config{
		Environment:       environment,
		HTTPAddr:          env("CHAT_HTTP_ADDR", ":8080"),
		DatabaseURL:       env("CHAT_DATABASE_URL", "postgres://chat:chat@localhost:5432/chat?sslmode=disable"),
		RedisAddr:         env("CHAT_REDIS_ADDR", "localhost:6379"),
		NATSURL:           env("CHAT_NATS_URL", "nats://localhost:4222"),
		MagicLinkBaseURL:  env("CHAT_MAGIC_LINK_BASE_URL", "http://localhost:3000/auth/callback"),
		WebOrigin:         strings.TrimRight(env("CHAT_WEB_ORIGIN", "http://localhost:3000"), "/"),
		YandexClientID:    strings.TrimSpace(env("CHAT_YANDEX_CLIENT_ID", "")),
		YandexRedirectURL: env("CHAT_YANDEX_REDIRECT_URL", "http://localhost:8080/api/v1/auth/yandex/callback"),
		CookieSecure:      environment != "development" && environment != "test",
	}

	if strings.TrimSpace(cfg.HTTPAddr) == "" {
		return Config{}, fmt.Errorf("CHAT_HTTP_ADDR must not be empty")
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return Config{}, fmt.Errorf("CHAT_DATABASE_URL must not be empty")
	}
	if err := validateHTTPURL(cfg.WebOrigin, cfg.Environment == "production"); err != nil {
		return Config{}, fmt.Errorf("CHAT_WEB_ORIGIN: %w", err)
	}
	if err := validateHTTPURL(cfg.MagicLinkBaseURL, cfg.Environment == "production"); err != nil {
		return Config{}, fmt.Errorf("CHAT_MAGIC_LINK_BASE_URL: %w", err)
	}
	if cfg.YandexClientID != "" {
		if err := validateHTTPURL(cfg.YandexRedirectURL, cfg.Environment == "production"); err != nil {
			return Config{}, fmt.Errorf("CHAT_YANDEX_REDIRECT_URL: %w", err)
		}
	}
	return cfg, nil
}

func validateHTTPURL(value string, requireHTTPS bool) error {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("must be an absolute http(s) URL")
	}
	if requireHTTPS && u.Scheme != "https" {
		return fmt.Errorf("must use HTTPS in production")
	}
	return nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
