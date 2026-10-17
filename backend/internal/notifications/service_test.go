package notifications

import (
	"context"
	"errors"
	"testing"
)

type captureStore struct{ subscribed PushSubscriptionInput }

func (s *captureStore) List(context.Context, string, int) ([]Notification, int64, error) { return nil, 0, nil }
func (s *captureStore) MarkRead(context.Context, string, string) error { return nil }
func (s *captureStore) MarkAllRead(context.Context, string) error { return nil }
func (s *captureStore) Emit(context.Context, EmitInput) error { return nil }
func (s *captureStore) UpsertPushSubscription(_ context.Context, in PushSubscriptionInput) error {
	s.subscribed = in
	return nil
}
func (s *captureStore) RevokePushSubscription(context.Context, string, string) error { return nil }

func TestPushSubscriptionRejectsUnsafeDestinations(t *testing.T) {
	store := &captureStore{}
	service := NewService(store)
	for _, endpoint := range []string{
		"http://push.example.com/send/123",
		"https://localhost/push",
		"https://notifications.internal/send/1",
		"https://127.0.0.1/notify",
		"https://[::1]/push",
		"https://192.168.1.1/send",
		"https://user:pass@push.example.com/send",
		"https://push.example.com:8443/send",
		"https://push.example.com/send#fragment",
	} {
		t.Run(endpoint, func(t *testing.T) {
			err := service.SubscribePush(context.Background(), PushSubscriptionInput{
				UserID: "user-id", Endpoint: endpoint, P256DH: "public-key", Auth: "shared-secret",
			})
			if !errors.Is(err, ErrInvalid) { t.Fatalf("expected invalid endpoint %q, got %v", endpoint, err) }
		})
	}
	if store.subscribed.Endpoint != "" { t.Fatal("unsafe subscription reached persistence") }
}

func TestPushSubscriptionAcceptsVendorHTTPS(t *testing.T) {
	store := &captureStore{}
	service := NewService(store)
	endpoint := "https://fcm.googleapis.com/fcm/send/abcdef"
	if err := service.SubscribePush(context.Background(), PushSubscriptionInput{
		UserID: "user-id", Endpoint: endpoint, P256DH: "key", Auth: "auth",
	}); err != nil { t.Fatal(err) }
	if store.subscribed.Endpoint != endpoint { t.Fatalf("expected endpoint saved, got %q", store.subscribed.Endpoint) }
}
