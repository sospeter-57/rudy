package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AttachmentStore abstracts image bytes so local disk can be swapped for object
// storage without changing the chat service.
type AttachmentStore interface {
	Save(ctx context.Context, filename string, data []byte) (key string, err error)
	Read(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// Local writes files under one directory. The directory must be backed by a
// persistent volume in production.
type Local struct {
	dir string
}

func NewLocal(dir string) (*Local, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("upload directory is required")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create upload directory: %w", err)
	}
	return &Local{dir: dir}, nil
}

func (l *Local) Save(_ context.Context, filename string, data []byte) (string, error) {
	extension := strings.ToLower(filepath.Ext(filename))
	if len(extension) > 10 {
		extension = ""
	}

	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	key := hex.EncodeToString(buffer) + extension

	path, err := l.path(key)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return "", err
	}
	return key, nil
}

func (l *Local) Read(_ context.Context, key string) ([]byte, error) {
	path, err := l.path(key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (l *Local) Delete(_ context.Context, key string) error {
	path, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// path rejects traversal. Storage keys are generated internally, so anything
// containing a separator is a bug or an attack.
func (l *Local) path(key string) (string, error) {
	if key == "" || filepath.Base(key) != key || strings.ContainsAny(key, `/\`) {
		return "", errors.New("invalid storage key")
	}
	return filepath.Join(l.dir, key), nil
}
