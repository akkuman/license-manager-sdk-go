package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store persists license envelopes.
type Store interface {
	Save(ctx context.Context, data []byte) error
	Load(ctx context.Context) ([]byte, error)
	Delete(ctx context.Context) error
}

// FileStore writes the license to a file, optionally encrypted.
type FileStore struct {
	path   string
	secret []byte
}

// NewFileStore returns a file-backed store.
func NewFileStore(path string, secret []byte) *FileStore {
	return &FileStore{path: path, secret: secret}
}

// Save writes the license data atomically.
func (s *FileStore) Save(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	payload, err := s.maybeEncrypt(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("storage: mkdir: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return fmt.Errorf("storage: write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("storage: rename: %w", err)
	}
	return nil
}

// Load reads the stored license if present.
func (s *FileStore) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: read: %w", err)
	}
	return s.maybeDecrypt(content)
}

// Delete removes the stored license file.
func (s *FileStore) Delete(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("storage: delete: %w", err)
	}
	return nil
}

func (s *FileStore) maybeEncrypt(data []byte) ([]byte, error) {
	if len(s.secret) == 0 {
		return data, nil
	}
	key := sha256.Sum256(s.secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("storage: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("storage: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("storage: nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	buf := []byte("enc:")
	buf = append(buf, []byte(base64.StdEncoding.EncodeToString(ciphertext))...)
	return buf, nil
}

func (s *FileStore) maybeDecrypt(data []byte) ([]byte, error) {
	if len(s.secret) == 0 {
		return data, nil
	}
	const prefix = "enc:"
	if len(data) > len(prefix) && string(data[:len(prefix)]) == prefix {
		data = data[len(prefix):]
	} else {
		// stored without prefix; treat as plain.
		return data, nil
	}
	raw, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("storage: decode: %w", err)
	}
	key := sha256.Sum256(s.secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("storage: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("storage: gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, fmt.Errorf("storage: invalid payload")
	}
	nonce := raw[:nonceSize]
	payload := raw[nonceSize:]
	plain, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return nil, fmt.Errorf("storage: decrypt: %w", err)
	}
	return plain, nil
}
