package identity

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

type LogMagicLinkSender struct {
	logger  *slog.Logger
	baseURL string
}

func NewLogMagicLinkSender(logger *slog.Logger, baseURL string) (*LogMagicLinkSender, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid magic link base URL")
	}
	return &LogMagicLinkSender{logger: logger, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func (s *LogMagicLinkSender) SendMagicLink(_ context.Context, email, rawToken string) error {
	// Development-only transport. Production startup must not wire this sender.
	link := s.baseURL + "?token=" + url.QueryEscape(rawToken)
	s.logger.Info("development magic link", "email", email, "url", link)
	return nil
}
