package messaging

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrChatNotFound      = errors.New("chat not found")
	ErrProfileNotFound   = errors.New("profile not found")
	ErrInteractionDenied = errors.New("interaction denied")
	ErrInvalidMessage    = errors.New("invalid message")
)

type ChatSummary struct {
	ChatID           string   `json:"chat_id"`
	Kind             string   `json:"kind"`
	Title            string   `json:"title"`
	PeerUsername     string   `json:"peer_username,omitempty"`
	PeerDisplayName  string   `json:"peer_display_name,omitempty"`
	LastMessage      *Message `json:"last_message,omitempty"`
	LastReadSequence int64    `json:"last_read_sequence"`
	UnreadCount      int64    `json:"unread_count"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Message struct {
	ID              string     `json:"id"`
	ChatID          string     `json:"chat_id"`
	SenderID        string     `json:"sender_id,omitempty"`
	ClientMessageID string     `json:"client_message_id"`
	Sequence        int64      `json:"sequence"`
	Type            string     `json:"type"`
	Body            string     `json:"body"`
	ReplyToID       *string    `json:"reply_to_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	EditedAt        *time.Time `json:"edited_at,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

type SendInput struct {
	UserID          string
	ChatID          string
	ClientMessageID string
	Body            string
	ReplyToID       string
}

type Store interface {
	EnsureDirectChat(ctx context.Context, userID, peerUsername string) (ChatSummary, error)
	ListChats(ctx context.Context, userID string, limit int, beforeUpdatedAt string) ([]ChatSummary, error)
	ListMessages(ctx context.Context, userID, chatID string, beforeSequence int64, limit int) ([]Message, error)
	SendText(ctx context.Context, input SendInput) (Message, bool, error)
	MarkRead(ctx context.Context, userID, chatID string, sequence int64) error
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) EnsureDirectChat(ctx context.Context, userID, username string) (ChatSummary, error) {
	username = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(username, "@")))
	if strings.TrimSpace(userID) == "" || username == "" || len(username) > 32 {
		return ChatSummary{}, ErrInteractionDenied
	}
	return s.store.EnsureDirectChat(ctx, userID, username)
}

func (s *Service) ListChats(ctx context.Context, userID, before string, rawLimit int) ([]ChatSummary, error) {
	limit := normalizeLimit(rawLimit, 50, 100)
	return s.store.ListChats(ctx, userID, limit, strings.TrimSpace(before))
}

func (s *Service) ListMessages(ctx context.Context, userID, chatID, before string, rawLimit int) ([]Message, error) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" || len(chatID) > 64 {
		return nil, ErrChatNotFound
	}
	var beforeSequence int64
	if strings.TrimSpace(before) != "" {
		parsed, err := strconv.ParseInt(before, 10, 64)
		if err != nil || parsed < 1 {
			return nil, ErrInvalidMessage
		}
		beforeSequence = parsed
	}
	return s.store.ListMessages(ctx, userID, chatID, beforeSequence, normalizeLimit(rawLimit, 50, 100))
}

func (s *Service) SendText(ctx context.Context, input SendInput) (Message, bool, error) {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ChatID = strings.TrimSpace(input.ChatID)
	input.ClientMessageID = strings.TrimSpace(input.ClientMessageID)
	input.Body = strings.TrimSpace(input.Body)
	input.ReplyToID = strings.TrimSpace(input.ReplyToID)
	if input.UserID == "" || input.ChatID == "" || len(input.ChatID) > 64 || !looksLikeUUID(input.ClientMessageID) {
		return Message{}, false, ErrInvalidMessage
	}
	if len([]rune(input.Body)) < 1 || len([]rune(input.Body)) > 4096 {
		return Message{}, false, ErrInvalidMessage
	}
	if input.ReplyToID != "" && !looksLikeUUID(input.ReplyToID) {
		return Message{}, false, ErrInvalidMessage
	}
	message, duplicate, err := s.store.SendText(ctx, input)
	if err != nil {
		if errors.Is(err, ErrChatNotFound) || errors.Is(err, ErrInteractionDenied) || errors.Is(err, ErrInvalidMessage) {
			return Message{}, false, err
		}
		return Message{}, false, fmt.Errorf("send message: %w", err)
	}
	return message, duplicate, nil
}

func (s *Service) MarkRead(ctx context.Context, userID, chatID string, sequence int64) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(chatID) == "" || sequence < 0 {
		return ErrInvalidMessage
	}
	return s.store.MarkRead(ctx, userID, chatID, sequence)
}

func normalizeLimit(value, fallback, maximum int) int {
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}
