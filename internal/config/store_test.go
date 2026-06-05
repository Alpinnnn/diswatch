package config

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestStoreEncryptsConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISWATCH_SECRET_KEY", "test-secret")
	store, err := Open(dir, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	_, err = store.Update(func(cfg *Config) error {
		cfg.Discord.Token = "super-secret-token"
		return nil
	})
	if err != nil {
		t.Fatalf("update store: %v", err)
	}
	raw, err := os.ReadFile(dir + "/config.enc.json")
	if err != nil {
		t.Fatalf("read encrypted config: %v", err)
	}
	if string(raw) == "" || strings.Contains(string(raw), "super-secret-token") {
		t.Fatal("encrypted config leaked plaintext token")
	}
}
