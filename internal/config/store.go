package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const (
	configFileName = "config.enc.json"
	keyFileName    = "master.key"
)

var aad = []byte("diswatch-config-v1")

type Store struct {
	mu     sync.RWMutex
	path   string
	key    []byte
	cfg    Config
	logger *slog.Logger
}

type encryptedFile struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func Open(dataDir string, logger *slog.Logger) (*Store, error) {
	key, err := loadMasterKey(dataDir, logger)
	if err != nil {
		return nil, err
	}
	store := &Store{
		path:   filepath.Join(dataDir, configFileName),
		key:    key,
		cfg:    Default(),
		logger: logger,
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Store) Update(fn func(*Config) error) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.cfg
	if err := fn(&next); err != nil {
		return Config{}, err
	}
	next.Normalize()
	if err := s.saveLocked(next); err != nil {
		return Config{}, err
	}
	s.cfg = next
	return next, nil
}

func (s *Store) load() error {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.cfg = Default()
		return nil
	}
	if err != nil {
		return err
	}

	var wrapped encryptedFile
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return fmt.Errorf("decode encrypted config wrapper: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(wrapped.Nonce)
	if err != nil {
		return fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(wrapped.Ciphertext)
	if err != nil {
		return fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return fmt.Errorf("decrypt config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(plain, &cfg); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	cfg.Normalize()
	s.cfg = cfg
	return nil
}

func (s *Store) saveLocked(cfg Config) error {
	plain, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	wrapped := encryptedFile{
		Version:    1,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(gcm.Seal(nil, nonce, plain, aad)),
	}
	raw, err := json.MarshalIndent(wrapped, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func loadMasterKey(dataDir string, logger *slog.Logger) ([]byte, error) {
	if raw := os.Getenv("DISWATCH_SECRET_KEY"); raw != "" {
		if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
		sum := sha256.Sum256([]byte(raw))
		logger.Warn("DISWATCH_SECRET_KEY is not base64-encoded 32 bytes; deriving a key with SHA-256")
		return sum[:], nil
	}

	keyPath := filepath.Join(dataDir, keyFileName)
	raw, err := os.ReadFile(keyPath)
	if err == nil {
		decoded, err := base64.StdEncoding.DecodeString(string(raw))
		if err != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("invalid master key file %s", keyPath)
		}
		return decoded, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		return nil, err
	}
	logger.Info("generated local encryption key", "path", keyPath)
	return key, nil
}
