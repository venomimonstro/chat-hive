package media

import (
	"bytes"
	"context"
	"crypto/sha256"
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

func TestFileStorageDoesNotFollowSymlinkedMediaPrefix(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "ab")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	files := &FileStorage{root: root}
	if err := files.Write("ab/abcdef.jpg", []byte("private")); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("must reject symlinked write prefix, got %v", err)
	}
	if _, err := files.Open("ab/abcdef.jpg"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("must reject symlinked read prefix, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "abcdef.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("media storage wrote outside root: %v", err)
	}
}

func TestFileStorageRejectsSymlinkedFile(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.jpg")
	if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ab"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "ab", "abcdef.jpg")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	files := &FileStorage{root: root}
	if _, err := files.Open("ab/abcdef.jpg"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("must reject symlinked media object, got %v", err)
	}
}

type resolvingStore struct {
	*fakeStore
	object StoredObject
	allowed bool
}
func (f *resolvingStore) Resolve(_ context.Context, _, _ string) (StoredObject, bool, error) {
	return f.object, f.allowed, nil
}

func TestOpenMissingMediaReturnsNotFound(t *testing.T) {
	files := &FileStorage{root: t.TempDir()}
	store := &resolvingStore{fakeStore: &fakeStore{}, allowed: true, object: StoredObject{
		StorageKey: "ab/missing.jpg", ByteSize: 12,
	}}
	service := NewService(store, files)
	file, _, err := service.Open(context.Background(), "viewer", "object-id")
	if file != nil { _ = file.Close(); t.Fatal("missing image unexpectedly opened") }
	if !errors.Is(err, ErrNotFound) { t.Fatalf("expected 404 sentinel, got %v", err) }
}

func TestOpenRejectsTruncatedMedia(t *testing.T) {
	root := t.TempDir()
	key := "ab/abcdef.jpg"
	if err := os.MkdirAll(filepath.Join(root, "ab"), 0o750); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, key), []byte("short"), 0o640); err != nil { t.Fatal(err) }
	digest := sha256.Sum256([]byte("short"))
	store := &resolvingStore{fakeStore: &fakeStore{}, allowed: true, object: StoredObject{
		StorageKey: key, ByteSize: 500, SHA256: digest[:],
	}}
	service := NewService(store, &FileStorage{root: root})
	file, _, err := service.Open(context.Background(), "viewer", "object-id")
	if file != nil { _ = file.Close(); t.Fatal("truncated image unexpectedly opened") }
	if !errors.Is(err, ErrCorruptMedia) { t.Fatalf("expected corrupted image error, got %v", err) }
	store.object.ByteSize = 5
	file, _, err = service.Open(context.Background(), "viewer", "object-id")
	if err != nil { t.Fatalf("intact image rejected: %v", err) }
	_ = file.Close()
}

func TestOpenRejectsSameSizeContentCorruption(t *testing.T) {
	root := t.TempDir()
	key := "ab/abcdef.jpg"
	if err := os.MkdirAll(filepath.Join(root, "ab"), 0o750); err != nil { t.Fatal(err) }
	original := []byte("original-data")
	digest := sha256.Sum256(original)
	if err := os.WriteFile(filepath.Join(root, key), []byte("altered!-data"), 0o640); err != nil { t.Fatal(err) }
	store := &resolvingStore{fakeStore: &fakeStore{}, allowed: true, object: StoredObject{
		StorageKey: key, ByteSize: int64(len(original)), SHA256: digest[:],
	}}
	service := NewService(store, &FileStorage{root: root})
	file, _, err := service.Open(context.Background(), "viewer", "id")
	if file != nil { _ = file.Close(); t.Fatal("corrupted bytes were served") }
	if !errors.Is(err, ErrCorruptMedia) { t.Fatalf("same-size corruption not detected: %v", err) }
	if err := os.WriteFile(filepath.Join(root, key), original, 0o640); err != nil { t.Fatal(err) }
	file, _, err = service.Open(context.Background(), "viewer", "id")
	if err != nil { t.Fatalf("valid digest rejected: %v", err) }
	defer file.Close()
	var contents bytes.Buffer
	if _, err := contents.ReadFrom(file); err != nil { t.Fatal(err) }
	if !bytes.Equal(contents.Bytes(), original) { t.Fatal("digest read did not rewind the file") }
}
