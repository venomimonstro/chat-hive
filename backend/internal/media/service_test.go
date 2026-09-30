package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

type fakeStore struct {
	created StoredObject
	err     error
}

func (f *fakeStore) Create(_ context.Context, object StoredObject) (Object, error) {
	f.created = object
	if f.err != nil {
		return Object{}, f.err
	}
	return Object{ID: "00000000-0000-4000-8000-000000000001", MimeType: object.MimeType, ByteSize: object.ByteSize, Width: object.Width, Height: object.Height}, nil
}
func (f *fakeStore) Resolve(context.Context, string, string) (StoredObject, bool, error) {
	return StoredObject{}, false, ErrNotFound
}

func jpegBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := jpeg.Encode(&output, image.NewRGBA(image.Rect(0, 0, width, height)), &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestUploadImageReencodesAndPersistsMetadata(t *testing.T) {
	root := t.TempDir()
	store := &fakeStore{}
	service := NewService(store, &FileStorage{root: root})
	object, err := service.UploadImage(context.Background(), "00000000-0000-4000-8000-000000000010", bytes.NewReader(jpegBytes(t, 32, 24)))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if object.ID == "" || object.Width != 32 || object.Height != 24 || store.created.StorageKey == "" {
		t.Fatalf("unexpected object: %+v stored=%+v", object, store.created)
	}
	if _, err := os.Stat(filepath.Join(root, store.created.StorageKey)); err != nil {
		t.Fatalf("stored file: %v", err)
	}
}

func TestUploadRejectsUnknownFormat(t *testing.T) {
	service := NewService(&fakeStore{}, &FileStorage{root: t.TempDir()})
	_, err := service.UploadImage(context.Background(), "00000000-0000-4000-8000-000000000010", bytes.NewBufferString("not-an-image"))
	if !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected invalid image, got %v", err)
	}
}

func TestUploadRejectsOversizeInput(t *testing.T) {
	service := NewService(&fakeStore{}, &FileStorage{root: t.TempDir()})
	_, err := service.UploadImage(context.Background(), "00000000-0000-4000-8000-000000000010", bytes.NewReader(make([]byte, MaxUploadBytes+1)))
	if !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected invalid image, got %v", err)
	}
}

func TestUploadRemovesFileWhenMetadataFails(t *testing.T) {
	root := t.TempDir()
	store := &fakeStore{err: errors.New("db down")}
	service := NewService(store, &FileStorage{root: root})
	_, err := service.UploadImage(context.Background(), "00000000-0000-4000-8000-000000000010", bytes.NewReader(jpegBytes(t, 10, 10)))
	if err == nil {
		t.Fatal("expected failure")
	}
	entries, walkErr := os.ReadDir(filepath.Join(root, store.created.StorageKey[:2]))
	if walkErr != nil && !errors.Is(walkErr, os.ErrNotExist) {
		t.Fatal(walkErr)
	}
	if len(entries) != 0 {
		t.Fatalf("orphan file remains: %d", len(entries))
	}
}
