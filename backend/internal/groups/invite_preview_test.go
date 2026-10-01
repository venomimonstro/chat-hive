package groups

import (
	"context"
	"errors"
	"testing"
	"time"
)

type previewStoreFake struct {
	preview InvitePreview
	err     error
	called  bool
	hashLen int
}

func (f *previewStoreFake) PreviewInvite(_ context.Context, tokenHash []byte, _ time.Time) (InvitePreview, error) {
	f.called = true
	f.hashLen = len(tokenHash)
	return f.preview, f.err
}

func TestInvitePreviewRejectsEmptyToken(t *testing.T) {
	store := &previewStoreFake{}
	service := NewInvitePreviewService(store)
	if _, err := service.Preview(context.Background(), "   "); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("expected ErrInviteInvalid, got %v", err)
	}
	if store.called {
		t.Fatal("store must not be called for an invalid token")
	}
}

func TestInvitePreviewHashesTokenBeforeStore(t *testing.T) {
	store := &previewStoreFake{preview: InvitePreview{Title: "Friends", MembersCount: 12}}
	service := NewInvitePreviewService(store)
	service.now = func() time.Time { return time.Unix(100, 0).UTC() }

	preview, err := service.Preview(context.Background(), "secret-invite-token")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if preview.Title != "Friends" || preview.MembersCount != 12 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if !store.called || store.hashLen != 32 {
		t.Fatalf("expected SHA-256 token hash, called=%v len=%d", store.called, store.hashLen)
	}
}

func TestInvitePreviewPropagatesExpiredInvite(t *testing.T) {
	store := &previewStoreFake{err: ErrInviteInvalid}
	service := NewInvitePreviewService(store)
	if _, err := service.Preview(context.Background(), "valid-looking-token"); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("expected ErrInviteInvalid, got %v", err)
	}
}
