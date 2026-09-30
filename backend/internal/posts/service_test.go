package posts

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	created CreateInput
	edited  string
	deleted string
	reacted bool
	saved   bool
	err     error
}

func (f *fakeStore) Create(_ context.Context, input CreateInput) (Post, error) {
	if f.err != nil {
		return Post{}, f.err
	}
	f.created = input
	return Post{ID: "00000000-0000-4000-8000-000000000001", AuthorID: input.AuthorID, Kind: input.Kind, Body: input.Body, Visibility: input.Visibility}, nil
}
func (f *fakeStore) Get(context.Context, string, string) (Post, error) {
	if f.err != nil {
		return Post{}, f.err
	}
	return Post{}, nil
}
func (f *fakeStore) ListByAuthor(context.Context, string, string, time.Time, int) ([]Post, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}
func (f *fakeStore) Edit(_ context.Context, _ string, postID, body string) (Post, error) {
	if f.err != nil {
		return Post{}, f.err
	}
	f.edited = body
	return Post{ID: postID, Body: body}, nil
}
func (f *fakeStore) Delete(_ context.Context, _ string, postID string) error {
	f.deleted = postID
	return f.err
}
func (f *fakeStore) AddReply(_ context.Context, _ string, postID, body string) (Reply, error) {
	if f.err != nil {
		return Reply{}, f.err
	}
	return Reply{PostID: postID, Body: body}, nil
}
func (f *fakeStore) ListReplies(context.Context, string, string, int) ([]Reply, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}
func (f *fakeStore) SetReaction(context.Context, string, string, string, bool) error {
	f.reacted = true
	return f.err
}
func (f *fakeStore) SetSaved(context.Context, string, string, bool) error {
	f.saved = true
	return f.err
}

func TestCreateNormalizesAndValidates(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	post, err := service.Create(context.Background(), CreateInput{
		AuthorID: " user ", Kind: " THOUGHT ", Body: " hello ", Visibility: "",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if store.created.AuthorID != "user" || store.created.Kind != "thought" || store.created.Body != "hello" || store.created.Visibility != "public" {
		t.Fatalf("unexpected normalized input: %+v", store.created)
	}
	if post.Kind != "thought" {
		t.Fatalf("unexpected post: %+v", post)
	}
}

func TestThoughtLengthLimit(t *testing.T) {
	service := NewService(&fakeStore{})
	body := make([]rune, 701)
	for i := range body {
		body[i] = 'я'
	}
	_, err := service.Create(context.Background(), CreateInput{AuthorID: "user", Kind: "thought", Body: string(body)})
	if !errors.Is(err, ErrInvalidPost) {
		t.Fatalf("expected invalid post, got %v", err)
	}
}

func TestEditRequiresValidPostID(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	_, err := service.Edit(context.Background(), "user", "bad-id", "text")
	if !errors.Is(err, ErrInvalidPost) {
		t.Fatalf("expected invalid post, got %v", err)
	}
	if store.edited != "" {
		t.Fatal("store called for invalid post id")
	}
}

func TestReplyLengthLimit(t *testing.T) {
	service := NewService(&fakeStore{})
	body := make([]rune, 2001)
	for i := range body {
		body[i] = 'x'
	}
	_, err := service.AddReply(context.Background(), "user", "00000000-0000-4000-8000-000000000002", string(body))
	if !errors.Is(err, ErrInvalidPost) {
		t.Fatalf("expected invalid post, got %v", err)
	}
}

func TestReactionRequiresValidValue(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	postID := "00000000-0000-4000-8000-000000000003"
	if err := service.SetReaction(context.Background(), "user", postID, "👍", true); err != nil {
		t.Fatalf("reaction: %v", err)
	}
	if !store.reacted {
		t.Fatal("reaction was not delegated")
	}
	if err := service.SetReaction(context.Background(), "user", postID, "", true); !errors.Is(err, ErrInvalidPost) {
		t.Fatalf("expected invalid reaction, got %v", err)
	}
}
