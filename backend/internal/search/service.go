package search

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidQuery = errors.New("invalid search query")

type Person struct {
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Bio             string `json:"bio"`
	FollowersCount  int64  `json:"followers_count"`
	SharedInterests int64  `json:"shared_interests"`
}

type Post struct {
	ID             string `json:"id"`
	AuthorUsername string `json:"author_username"`
	AuthorName     string `json:"author_name"`
	Kind           string `json:"kind"`
	Body           string `json:"body"`
}

type Group struct {
	ChatID       string `json:"chat_id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	MembersCount int64  `json:"members_count"`
	IsMember     bool   `json:"is_member"`
}

type Results struct {
	People []Person `json:"people"`
	Posts  []Post   `json:"posts"`
	Groups []Group  `json:"groups"`
}

type Store interface {
	Search(ctx context.Context, userID, query string, limit int) (Results, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) Search(ctx context.Context, userID, query string, limit int) (Results, error) {
	userID = strings.TrimSpace(userID)
	query = strings.TrimSpace(query)
	if userID == "" || len([]rune(query)) < 2 || len([]rune(query)) > 100 {
		return Results{}, ErrInvalidQuery
	}
	if limit <= 0 {
		limit = 12
	}
	if limit > 30 {
		limit = 30
	}
	return s.store.Search(ctx, userID, query, limit)
}
