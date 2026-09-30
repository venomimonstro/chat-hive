package groups

import (
	"context"
	"strings"
)

type moderationStore interface {
	DeleteMessageAsModerator(ctx context.Context, actorID, chatID, messageID string) error
}

type ModerationService struct{ store moderationStore }

func NewModerationService(store moderationStore) *ModerationService {
	return &ModerationService{store:store}
}

func (s *ModerationService) DeleteMessage(ctx context.Context, actorID, chatID, messageID string) error {
	actorID=strings.TrimSpace(actorID)
	chatID=strings.TrimSpace(chatID)
	messageID=strings.TrimSpace(messageID)
	if actorID==""||!looksLikeUUID(chatID)||!looksLikeUUID(messageID){return ErrInvalidGroup}
	return s.store.DeleteMessageAsModerator(ctx,actorID,chatID,messageID)
}

func looksLikeUUID(value string) bool {
	if len(value)!=36{return false}
	for i,char:=range value{
		if i==8||i==13||i==18||i==23{if char!='-'{return false};continue}
		if !((char>='0'&&char<='9')||(char>='a'&&char<='f')||(char>='A'&&char<='F')){return false}
	}
	return true
}
