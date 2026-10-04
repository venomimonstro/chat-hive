package config

import "testing"

func TestDevelopmentDefaultsRemainUsable(t *testing.T) {
	t.Setenv("CHAT_ENV", "development")
	t.Setenv("CHAT_DATABASE_URL", "postgres://chat:chat@localhost:5432/chat?sslmode=disable")
	t.Setenv("CHAT_WEB_ORIGIN", "http://localhost:3000")
	t.Setenv("CHAT_MAGIC_LINK_BASE_URL", "http://localhost:3000/auth/callback")
	t.Setenv("CHAT_SMTP_ADDR", "")
	t.Setenv("CHAT_SMTP_FROM", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("development config should load: %v", err)
	}
	if cfg.CookieSecure {
		t.Fatal("development cookies must not require HTTPS")
	}
}

func TestProductionRejectsDevelopmentDatabaseCredentials(t *testing.T) {
	setProductionBase(t)
	t.Setenv("CHAT_DATABASE_URL", "postgres://chat:chat@postgres:5432/chat?sslmode=disable")

	if _, err := Load(); err == nil {
		t.Fatal("expected development database credentials to be rejected")
	}
}

func TestProductionRequiresHTTPSOrigin(t *testing.T) {
	setProductionBase(t)
	t.Setenv("CHAT_WEB_ORIGIN", "http://chat.example.test")

	if _, err := Load(); err == nil {
		t.Fatal("expected insecure production web origin to be rejected")
	}
}

func TestProductionRejectsPlaceholderSMTPFrom(t *testing.T) {
	setProductionBase(t)
	t.Setenv("CHAT_SMTP_FROM", "CHAT <no-reply@example.com>")

	if _, err := Load(); err == nil {
		t.Fatal("expected placeholder SMTP sender to be rejected")
	}
}

func TestProductionRequiresTrustedProxyCIDR(t *testing.T) {
	setProductionBase(t)
	t.Setenv("CHAT_TRUSTED_PROXY_CIDRS", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected trusted proxy CIDR to be required in production")
	}
}

func TestProductionAcceptsExplicitSecureConfiguration(t *testing.T) {
	setProductionBase(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("production config should load: %v", err)
	}
	if !cfg.CookieSecure {
		t.Fatal("production cookies must require secure transport")
	}
}

func setProductionBase(t *testing.T) {
	t.Helper()
	t.Setenv("CHAT_ENV", "production")
	t.Setenv("CHAT_DATABASE_URL", "postgres://chat_app:strong-password@postgres:5432/chat?sslmode=disable")
	t.Setenv("CHAT_WEB_ORIGIN", "https://chat.example.test")
	t.Setenv("CHAT_MAGIC_LINK_BASE_URL", "https://chat.example.test/auth/callback")
	t.Setenv("CHAT_SMTP_ADDR", "smtp.example.test:587")
	t.Setenv("CHAT_SMTP_USERNAME", "mailer")
	t.Setenv("CHAT_SMTP_PASSWORD", "smtp-password")
	t.Setenv("CHAT_SMTP_FROM", "CHAT <no-reply@chat.example.test>")
	t.Setenv("CHAT_YANDEX_CLIENT_ID", "")
	t.Setenv("CHAT_TRUSTED_PROXY_CIDRS", "172.31.238.2/32")
}
