package messaging

import (
	"context"
	"strings"
)

type ReactionBatch map[string][]ReactionSummary

type reactionBatchStore interface {
	ListReactionBatch(ctx context.Context, userID string, messageIDs []string) (ReactionBatch, error)
}

type ReactionBatchService struct{ store reactionBatchStore }

func NewReactionBatchService(store reactionBatchStore) *ReactionBatchService {
	return &ReactionBatchService{store: store}
}

func (s *ReactionBatchService) List(ctx context.Context, userID string, messageIDs []string) (ReactionBatch, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" || len(messageIDs) == 0 || len(messageIDs) > 100 {
		return nil, ErrInvalidMessage
	}
	seen := make(map[string]struct{}, len(messageIDs))
	normalized := make([]string, 0, len(messageIDs))
	for _, raw := range messageIDs {
		id := strings.TrimSpace(raw)
		if !looksLikeUUID(id) { return nil, ErrInvalidMessage }
		if _, exists := seen[id]; exists { continue }
		seen[id] = struct{}{}
		normalized = append(normalized,id)
	}
	return s.store.ListReactionBatch(ctx,userID,normalized)
}
