package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidImage = errors.New("invalid image")
	ErrNotFound     = errors.New("media not found")
	ErrForbidden    = errors.New("media access denied")
)

const (
	MaxUploadBytes = 8 << 20
	MaxDimension   = 12000
	MaxPixels      = 40_000_000
)

type Object struct {
	ID        string `json:"id"`
	MimeType  string `json:"mime_type"`
	ByteSize  int64  `json:"byte_size"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	URL       string `json:"url"`
}

type StoredObject struct {
	ID         string
	OwnerID    string
	StorageKey string
	MimeType   string
	ByteSize   int64
	Width      int
	Height     int
	SHA256     []byte
}

type Store interface {
	Create(ctx context.Context, object StoredObject) (Object, error)
	Resolve(ctx context.Context, viewerID, mediaID string) (StoredObject, bool, error)
}

type FileStorage struct{ root string }

func NewFileStorageFromEnv() (*FileStorage, error) {
	root := strings.TrimSpace(os.Getenv("CHAT_MEDIA_ROOT"))
	if root == "" {
		root = "./var/media"
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("media root: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, fmt.Errorf("create media root: %w", err)
	}
	return &FileStorage{root: absolute}, nil
}

func (s *FileStorage) Write(key string, data []byte) error {
	clean := filepath.Clean(key)
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return ErrInvalidImage
	}
	path := filepath.Join(s.root, clean)
	if !strings.HasPrefix(path, s.root+string(os.PathSeparator)) {
		return ErrInvalidImage
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o640); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (s *FileStorage) Open(key string) (*os.File, error) {
	clean := filepath.Clean(key)
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return nil, ErrNotFound
	}
	path := filepath.Join(s.root, clean)
	if !strings.HasPrefix(path, s.root+string(os.PathSeparator)) {
		return nil, ErrNotFound
	}
	return os.Open(path)
}

type Service struct {
	store Store
	files *FileStorage
}

func NewService(store Store, files *FileStorage) *Service {
	return &Service{store: store, files: files}
}

func (s *Service) UploadImage(ctx context.Context, ownerID string, input io.Reader) (Object, error) {
	if strings.TrimSpace(ownerID) == "" {
		return Object{}, ErrForbidden
	}
	data, err := io.ReadAll(io.LimitReader(input, MaxUploadBytes+1))
	if err != nil {
		return Object{}, err
	}
	if len(data) == 0 || len(data) > MaxUploadBytes {
		return Object{}, ErrInvalidImage
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > MaxDimension || config.Height > MaxDimension || int64(config.Width)*int64(config.Height) > MaxPixels {
		return Object{}, ErrInvalidImage
	}
	if format != "jpeg" && format != "png" {
		return Object{}, ErrInvalidImage
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Object{}, ErrInvalidImage
	}

	var output bytes.Buffer
	var mimeType, extension string
	switch format {
	case "jpeg":
		mimeType, extension = "image/jpeg", ".jpg"
		err = jpeg.Encode(&output, decoded, &jpeg.Options{Quality: 88})
	case "png":
		mimeType, extension = "image/png", ".png"
		encoder := png.Encoder{CompressionLevel: png.BestSpeed}
		err = encoder.Encode(&output, decoded)
	default:
		return Object{}, ErrInvalidImage
	}
	if err != nil || output.Len() == 0 {
		return Object{}, ErrInvalidImage
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return Object{}, err
	}
	name := hex.EncodeToString(random) + extension
	key := filepath.Join(name[:2], name)
	if err := s.files.Write(key, output.Bytes()); err != nil {
		return Object{}, err
	}
	hash := sha256.Sum256(output.Bytes())
	object, err := s.store.Create(ctx, StoredObject{
		OwnerID: ownerID, StorageKey: key, MimeType: mimeType, ByteSize: int64(output.Len()),
		Width: config.Width, Height: config.Height, SHA256: hash[:],
	})
	if err != nil {
		return Object{}, err
	}
	object.URL = "/api/v1/media/" + object.ID + "/content"
	return object, nil
}

func (s *Service) Open(ctx context.Context, viewerID, mediaID string) (*os.File, StoredObject, error) {
	object, allowed, err := s.store.Resolve(ctx, viewerID, strings.TrimSpace(mediaID))
	if err != nil {
		return nil, StoredObject{}, err
	}
	if !allowed {
		return nil, StoredObject{}, ErrForbidden
	}
	file, err := s.files.Open(object.StorageKey)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, StoredObject{}, ErrNotFound
		}
		return nil, StoredObject{}, err
	}
	return file, object, nil
}
