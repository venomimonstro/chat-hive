package messaging

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	chat      ChatSummary
	messages  []Message
	sendCalls int
	markRead  int64
	err       error
}

func (f *fakeStore) EnsureDirectChat(context.Context, string, string) (ChatSummary, error) {
	if f.err != nil { return ChatSummary{}, f.err }
	return f.chat, nil
}
func (f *fakeStore) ListChats(context.Context, string, int, string) ([]ChatSummary, error) {
	if f.err != nil { return nil, f.err }
	return []ChatSummary{f.chat}, nil
}
func (f *fakeStore) ListMessages(context.Context, string, string, int64, int) ([]Message, error) {
	if f.err != nil { return nil, f.err }
	return f.messages, nil
}
func (f *fakeStore) SendText(_ context.Context, input SendInput) (Message, bool, error) {
	f.sendCalls++
	if f.err != nil { return Message{}, false, f.err }
	return Message{ChatID: input.ChatID, ClientMessageID: input.ClientMessageID, Body: input.Body, Sequence: 1}, false, nil
}
func (f *fakeStore) MarkRead(_ context.Context, _ string, _ string, sequence int64) error {
	f.markRead = sequence
	return f.err
}

func TestSendTextValidation(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	_, _, err := service.SendText(context.Background(), SendInput{
		UserID: "user",
		ChatID: "chat",
		ClientMessageID: "not-a-uuid",
		Body: "hello",
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
		UserID: " user ",
		ChatID: " chat ",
		ClientMessageID: "00000000-0000-4000-8000-000000000001",
		Body: " hello ",
	})
	if err != nil { t.Fatalf("send: %v", err) }
	if duplicate { t.Fatal("unexpected duplicate") }
	if message.Body != "hello" || message.ChatID != "chat" {
		t.Fatalf("unexpected message: %+v", message)
	}
}

func TestMessageLengthLimit(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	tooLong := make([]rune, 4097)
	for i := range tooLong { tooLong[i] = 'я' }
	_, _, err := service.SendText(context.Background(), SendInput{
		UserID: "user",
		ChatID: "chat",
		ClientMessageID: "00000000-0000-4000-8000-000000000002",
		Body: string(tooLong),
	})
	if !errors.Is(err, ErrInvalidMessage) { t.Fatalf("expected invalid message, got %v", err) }
}

func TestListMessagesRejectsBadCursor(t *testing.T) {
	service := NewService(&fakeStore{})
	_, err := service.ListMessages(context.Background(), "user", "chat", "abc", 50)
	if !errors.Is(err, ErrInvalidMessage) { t.Fatalf("expected invalid cursor, got %v", err) }
}

func TestMarkReadRejectsNegativeSequence(t *testing.T) {
	service := NewService(&fakeStore{})
	if err := service.MarkRead(context.Background(), "user", "chat", -1); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("expected invalid sequence, got %v", err)
	}
}
