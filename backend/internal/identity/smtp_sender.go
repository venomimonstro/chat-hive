package identity

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

type SMTPConfig struct {
	Addr     string
	Username string
	Password string
	From     string
	BaseURL  string
}

type SMTPMagicLinkSender struct {
	config   SMTPConfig
	host     string
	fromAddr string
	fromName string
}

func NewSMTPMagicLinkSender(config SMTPConfig) (*SMTPMagicLinkSender, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(config.Addr))
	if err != nil || host == "" || port == "" {
		return nil, fmt.Errorf("invalid SMTP address")
	}
	from, err := mail.ParseAddress(strings.TrimSpace(config.From))
	if err != nil || from.Address == "" {
		return nil, fmt.Errorf("invalid SMTP from address")
	}
	parsed, err := url.Parse(config.BaseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("production magic-link URL must be HTTPS")
	}
	return &SMTPMagicLinkSender{
		config: config,
		host: host,
		fromAddr: from.Address,
		fromName: from.Name,
	}, nil
}

func (s *SMTPMagicLinkSender) SendMagicLink(ctx context.Context, email, rawToken string) error {
	link := strings.TrimRight(s.config.BaseURL, "/") + "?token=" + url.QueryEscape(rawToken)
	dialer := &net.Dialer{Timeout: 8 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("SMTP connect: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("SMTP client: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("SMTP server does not support STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("SMTP STARTTLS: %w", err)
	}
	if s.config.Username != "" {
		auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}
	if err := client.Mail(s.fromAddr); err != nil {
		return fmt.Errorf("SMTP MAIL FROM: %w", err)
	}
	if err := client.Rcpt(email); err != nil {
		return fmt.Errorf("SMTP RCPT TO: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	if err := writeMagicLinkMessage(writer, s.fromName, s.fromAddr, email, link); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("SMTP message close: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("SMTP quit: %w", err)
	}
	return nil
}

func writeMagicLinkMessage(w io.Writer, fromName, fromAddr, to, link string) error {
	fromHeader := fromAddr
	if strings.TrimSpace(fromName) != "" {
		fromHeader = mime.QEncoding.Encode("UTF-8", fromName) + " <" + fromAddr + ">"
	}
	subject := mime.QEncoding.Encode("UTF-8", "Вход в CHAT")
	body := "Войдите в CHAT по одноразовой ссылке:\r\n\r\n" + link + "\r\n\r\nСсылка действует ограниченное время и может быть использована только один раз.\r\n"
	message := "From: " + fromHeader + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n\r\n" + body
	if _, err := io.WriteString(w, message); err != nil {
		return fmt.Errorf("SMTP write: %w", err)
	}
	return nil
}
