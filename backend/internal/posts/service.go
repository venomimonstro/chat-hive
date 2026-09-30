package posts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidPost = errors.New("invalid post")
	ErrNotFound    = errors.New("post not found")
	ErrForbidden   = errors.New("forbidden")
)

type MediaRef struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	URL      string `json:"url"`
}

type Post struct {
	ID             string     `json:"id"`
	AuthorID       string     `json:"author_id"`
	AuthorUsername string     `json:"author_username"`
	AuthorName     string     `json:"author_name"`
	Kind           string     `json:"kind"`
	Body           string     `json:"body"`
	Visibility     string     `json:"visibility"`
	Media          []MediaRef `json:"media"`
	RepliesCount   int64      `json:"replies_count"`
	ReactionsCount int64      `json:"reactions_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Mine           bool       `json:"mine"`
	Saved          bool       `json:"saved"`
}

type Reply struct {
	ID             string    `json:"id"`
	PostID         string    `json:"post_id"`
	AuthorID       string    `json:"author_id"`
	AuthorUsername string    `json:"author_username"`
	AuthorName     string    `json:"author_name"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
	Mine           bool      `json:"mine"`
}

type CreateInput struct {
	AuthorID   string
	Kind       string
	Body       string
	Visibility string
	MediaIDs   []string
}

type Store interface {
	Create(ctx context.Context, input CreateInput) (Post, error)
	Get(ctx context.Context, viewerID, postID string) (Post, error)
	ListByAuthor(ctx context.Context, viewerID, username string, before time.Time, limit int) ([]Post, error)
	Edit(ctx context.Context, authorID, postID, body string) (Post, error)
	Delete(ctx context.Context, authorID, postID string) error
	AddReply(ctx context.Context, authorID, postID, body string) (Reply, error)
	ListReplies(ctx context.Context, viewerID, postID string, limit int) ([]Reply, error)
	SetReaction(ctx context.Context, userID, postID, reaction string, enabled bool) error
	SetSaved(ctx context.Context, userID, postID string, saved bool) error
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) Create(ctx context.Context, input CreateInput) (Post, error) {
	input.AuthorID = strings.TrimSpace(input.AuthorID)
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	input.Body = strings.TrimSpace(input.Body)
	input.Visibility = strings.ToLower(strings.TrimSpace(input.Visibility))
	if input.Visibility == "" {
		input.Visibility = "public"
	}
	if input.AuthorID == "" || !validKind(input.Kind) || !validVisibility(input.Visibility) || !validBody(input.Kind, input.Body, len(input.MediaIDs)) {
		return Post{}, ErrInvalidPost
	}
	if len(input.MediaIDs) > 10 {
		return Post{}, ErrInvalidPost
	}
	seen := make(map[string]struct{}, len(input.MediaIDs))
	cleanMedia := make([]string, 0, len(input.MediaIDs))
	for _, id := range input.MediaIDs {
		id = strings.TrimSpace(id)
		if !looksLikeUUID(id) {
			return Post{}, ErrInvalidPost
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		cleanMedia = append(cleanMedia, id)
	}
	input.MediaIDs = cleanMedia
	if input.Kind == "photo" && len(input.MediaIDs) == 0 {
		return Post{}, ErrInvalidPost
	}
	post, err := s.store.Create(ctx, input)
	if err != nil {
		if errors.Is(err, ErrForbidden) || errors.Is(err, ErrNotFound) {
			return Post{}, err
		}
		return Post{}, fmt.Errorf("create post: %w", err)
	}
	return post, nil
}

func (s *Service) Get(ctx context.Context, viewerID, postID string) (Post, error) {
	if strings.TrimSpace(viewerID) == "" || !looksLikeUUID(strings.TrimSpace(postID)) {
		return Post{}, ErrNotFound
	}
	return s.store.Get(ctx, viewerID, strings.TrimSpace(postID))
}

func (s *Service) ListByAuthor(ctx context.Context, viewerID, username string, before time.Time, limit int) ([]Post, error) {
	viewerID = strings.TrimSpace(viewerID)
	username = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(username, "@")))
	if viewerID == "" || username == "" || len(username) > 32 {
		return nil, ErrNotFound
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	return s.store.ListByAuthor(ctx, viewerID, username, before, limit)
}

func (s *Service) Edit(ctx context.Context, authorID, postID, body string) (Post, error) {
	authorID = strings.TrimSpace(authorID)
	postID = strings.TrimSpace(postID)
	body = strings.TrimSpace(body)
	if authorID == "" || !looksLikeUUID(postID) || len([]rune(body)) < 1 || len([]rune(body)) > 8000 {
		return Post{}, ErrInvalidPost
	}
	return s.store.Edit(ctx, authorID, postID, body)
}

func (s *Service) Delete(ctx context.Context, authorID, postID string) error {
	if strings.TrimSpace(authorID) == "" || !looksLikeUUID(strings.TrimSpace(postID)) {
		return ErrInvalidPost
	}
	return s.store.Delete(ctx, authorID, strings.TrimSpace(postID))
}

func (s *Service) AddReply(ctx context.Context, authorID, postID, body string) (Reply, error) {
	authorID = strings.TrimSpace(authorID)
	postID = strings.TrimSpace(postID)
	body = strings.TrimSpace(body)
	if authorID == "" || !looksLikeUUID(postID) || len([]rune(body)) < 1 || len([]rune(body)) > 2000 {
		return Reply{}, ErrInvalidPost
	}
	return s.store.AddReply(ctx, authorID, postID, body)
}

func (s *Service) ListReplies(ctx context.Context, viewerID, postID string, limit int) ([]Reply, error) {
	if strings.TrimSpace(viewerID) == "" || !looksLikeUUID(strings.TrimSpace(postID)) {
		return nil, ErrNotFound
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.store.ListReplies(ctx, viewerID, strings.TrimSpace(postID), limit)
}

func (s *Service) SetReaction(ctx context.Context, userID, postID, reaction string, enabled bool) error {
	userID = strings.TrimSpace(userID)
	postID = strings.TrimSpace(postID)
	reaction = strings.TrimSpace(reaction)
	if userID == "" || !looksLikeUUID(postID) || reaction == "" || len([]rune(reaction)) > 8 {
		return ErrInvalidPost
	}
	return s.store.SetReaction(ctx, userID, postID, reaction, enabled)
}

func (s *Service) SetSaved(ctx context.Context, userID, postID string, saved bool) error {
	if strings.TrimSpace(userID) == "" || !looksLikeUUID(strings.TrimSpace(postID)) {
		return ErrInvalidPost
	}
	return s.store.SetSaved(ctx, strings.TrimSpace(userID), strings.TrimSpace(postID), saved)
}

func validKind(kind string) bool {
	return kind == "thought" || kind == "photo" || kind == "post"
}

func validVisibility(value string) bool {
	return value == "public" || value == "followers"
}

func validBody(kind, body string, mediaCount int) bool {
	length := len([]rune(body))
	switch kind {
	case "thought":
		return mediaCount == 0 && length >= 1 && length <= 700
	case "photo":
		return mediaCount > 0 && length <= 2000
	case "post":
		return length >= 1 && length <= 8000
	default:
		return false
	}
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
