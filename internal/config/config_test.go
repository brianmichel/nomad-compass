package config

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestDecodeHexKey(t *testing.T) {
	input := make([]byte, 32)
	for i := range input {
		input[i] = byte(i)
	}
	hexKey := hex.EncodeToString(input)
	decoded, err := decodeHexKey(hexKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(decoded) != string(input) {
		t.Fatalf("expected %x but got %x", input, decoded)
	}
}

func TestDecodeHexKeyInvalidLength(t *testing.T) {
	if _, err := decodeHexKey("abcd"); err == nil {
		t.Fatalf("expected error for invalid length")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("COMPASS_CREDENTIAL_KEY", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Address != ":8080" {
		t.Fatalf("expected default server address, got %q", cfg.Server.Address)
	}
	if cfg.Database.Path != "data/nomad-compass.sqlite" {
		t.Fatalf("expected default database path, got %q", cfg.Database.Path)
	}
	if cfg.Nomad.Address != "http://127.0.0.1:4646" {
		t.Fatalf("expected default nomad address, got %q", cfg.Nomad.Address)
	}
	if cfg.Repo.BaseDir != "data/repos" {
		t.Fatalf("expected default repo base dir, got %q", cfg.Repo.BaseDir)
	}
	if cfg.Repo.PollInterval != 30*time.Second {
		t.Fatalf("expected default poll interval, got %s", cfg.Repo.PollInterval)
	}
	if len(cfg.Crypto.CredentialKey) != 32 {
		t.Fatalf("expected 32 byte key, got %d", len(cfg.Crypto.CredentialKey))
	}
}

func TestLoadRequiresCredentialKey(t *testing.T) {
	t.Setenv("COMPASS_CREDENTIAL_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatalf("expected error when credential key is missing")
	}
}
