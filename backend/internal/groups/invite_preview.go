package groups

import (
	"context"
	"strings"
	"time"
)

type InvitePreview struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	MembersCount int64  `json:"members_count"`
}

type InvitePreviewStore interface {
	PreviewInvite(ctx context.Context, tokenHash []byte, now time.Time) (InvitePreview, error)
}

type InvitePreviewService struct {
	store InvitePreviewStore
	now   func() time.Time
}

func NewInvitePreviewService(store InvitePreviewStore) *InvitePreviewService {
	return &InvitePreviewService{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s *InvitePreviewService) Preview(ctx context.Context, rawToken string) (InvitePreview, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || len(rawToken) > 512 {
		return InvitePreview{}, ErrInviteInvalid
	}
	return s.store.PreviewInvite(ctx, hashToken(rawToken), s.now())
}
