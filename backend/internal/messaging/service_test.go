package messaging

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	chat          ChatSummary
	messages      []Message
	sendCalls     int
	markRead      int64
	editedBody    string
	deletedID     string
	reaction      string
	reactionState bool
	err           error
}

func (f *fakeStore) EnsureDirectChat(context.Context, string, string) (ChatSummary, error) {
	if f.err != nil {
		return ChatSummary{}, f.err
	}
	return f.chat, nil
}

func (f *fakeStore) ListChats(context.Context, string, int, string) ([]ChatSummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []ChatSummary{f.chat}, nil
}

func (f *fakeStore) ListMessages(context.Context, string, string, int64, int) ([]Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.messages, nil
}

func (f *fakeStore) SendText(_ context.Context, input SendInput) (Message, bool, error) {
	f.sendCalls++
	if f.err != nil {
		return Message{}, false, f.err
	}
	return Message{
		ChatID: input.ChatID, ClientMessageID: input.ClientMessageID,
		Body: input.Body, Sequence: 1,
	}, false, nil
}

func (f *fakeStore) EditMessage(_ context.Context, _ string, messageID, body string) (Message, error) {
	if f.err != nil {
		return Message{}, f.err
	}
	f.editedBody = body
	return Message{ID: messageID, Body: body}, nil
}

func (f *fakeStore) DeleteMessage(_ context.Context, _ string, messageID string) error {
	if f.err != nil {
		return f.err
	}
	f.deletedID = messageID
	return nil
}

func (f *fakeStore) SetReaction(_ context.Context, _ string, _ string, reaction string, enabled bool) error {
	if f.err != nil {
		return f.err
	}
	f.reaction = reaction
	f.reactionState = enabled
	return nil
}

func (f *fakeStore) ListReactions(context.Context, string, string) ([]ReactionSummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []ReactionSummary{{Reaction: "👍", Count: 2, Mine: true}}, nil
}

func (f *fakeStore) MarkRead(_ context.Context, _ string, _ string, sequence int64) error {
	f.markRead = sequence
	return f.err
}

func TestSendTextValidation(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	_, _, err := service.SendText(context.Background(), SendInput{
		UserID: "user", ChatID: "chat", ClientMessageID: "not-a-uuid", Body: "hello",
	})
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected invalid message, got %v", err)
	}
	if store.sendCalls != 0 {
		t.Fatal("store must not be called for invalid client message id")
	}
}

func TestSendTextTrimsAndDelegates(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	message, duplicate, err := service.SendText(context.Background(), SendInput{
		UserID: " user ", ChatID: " chat ",
		ClientMessageID: "00000000-0000-4000-8000-000000000001", Body: " hello ",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if duplicate {
		t.Fatal("unexpected duplicate")
	}
	if message.Body != "hello" || message.ChatID != "chat" {
		t.Fatalf("unexpected message: %+v", message)
	}
}

func TestMessageLengthLimit(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	tooLong := make([]rune, 4097)
	for i := range tooLong {
		tooLong[i] = 'я'
	}
	_, _, err := service.SendText(context.Background(), SendInput{
		UserID: "user", ChatID: "chat",
		ClientMessageID: "00000000-0000-4000-8000-000000000002", Body: string(tooLong),
	})
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected invalid message, got %v", err)
	}
}

func TestEditMessageValidationAndTrim(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	message, err := service.EditMessage(
		context.Background(), "user", "00000000-0000-4000-8000-000000000003", " edited ",
	)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if store.editedBody != "edited" || message.Body != "edited" {
		t.Fatalf("message was not normalized: store=%q message=%q", store.editedBody, message.Body)
	}
}

func TestDeleteMessageRejectsBadID(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	if err := service.DeleteMessage(context.Background(), "user", "bad-id"); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected invalid message, got %v", err)
	}
	if store.deletedID != "" {
		t.Fatal("store called for invalid id")
	}
}

func TestReactionValidation(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	messageID := "00000000-0000-4000-8000-000000000004"
	if err := service.SetReaction(context.Background(), "user", messageID, "👍", true); err != nil {
		t.Fatalf("set reaction: %v", err)
	}
	if store.reaction != "👍" || !store.reactionState {
		t.Fatalf("unexpected reaction state: %q %v", store.reaction, store.reactionState)
	}
	if err := service.SetReaction(context.Background(), "user", messageID, "bad reaction with spaces", true); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected invalid reaction, got %v", err)
	}
}

func TestListMessagesRejectsBadCursor(t *testing.T) {
	service := NewService(&fakeStore{})
	_, err := service.ListMessages(context.Background(), "user", "chat", "abc", 50)
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected invalid cursor, got %v", err)
	}
}

func TestMarkReadRejectsNegativeSequence(t *testing.T) {
	service := NewService(&fakeStore{})
	if err := service.MarkRead(context.Background(), "user", "chat", -1); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected invalid sequence, got %v", err)
	}
}
