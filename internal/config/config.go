package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config represents the application level configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Nomad    NomadConfig
	Repo     RepoConfig
	Crypto   CryptoConfig
}

// ServerConfig drives the HTTP server.
type ServerConfig struct {
	Address string
}

// DatabaseConfig drives the persistence layer.
type DatabaseConfig struct {
	Path string
}

// NomadConfig holds the fields required to connect to Nomad.
type NomadConfig struct {
	Address   string
	Token     string
	Region    string
	Namespace string
}

// RepoConfig controls how repositories are managed and reconciled.
type RepoConfig struct {
	BaseDir      string
	PollInterval time.Duration
}

// CryptoConfig controls how sensitive fields are secured.
type CryptoConfig struct {
	CredentialKey []byte
}

const (
	defaultServerAddress   = ":8080"
	defaultDatabasePath    = "data/nomad-compass.sqlite"
	defaultNomadAddress    = "http://127.0.0.1:4646"
	defaultRepoBaseDir     = "data/repos"
	defaultRepoPollSeconds = 30
)

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{}

	cfg.Server = ServerConfig{
		Address: getEnv("COMPASS_HTTP_ADDR", defaultServerAddress),
	}

	cfg.Database = DatabaseConfig{
		Path: getEnv("COMPASS_DATABASE_PATH", defaultDatabasePath),
	}

	cfg.Nomad = NomadConfig{
		Address:   getEnv("COMPASS_NOMAD_ADDR", defaultNomadAddress),
		Token:     os.Getenv("COMPASS_NOMAD_TOKEN"),
		Region:    getEnv("COMPASS_NOMAD_REGION", ""),
		Namespace: getEnv("COMPASS_NOMAD_NAMESPACE", ""),
	}

	poll := time.Duration(defaultRepoPollSeconds) * time.Second
	if raw := os.Getenv("COMPASS_REPO_POLL_SECONDS"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			poll = time.Duration(v) * time.Second
		}
	}

	cfg.Repo = RepoConfig{
		BaseDir:      getEnv("COMPASS_REPO_BASE_DIR", defaultRepoBaseDir),
		PollInterval: poll,
	}

	keyHex := os.Getenv("COMPASS_CREDENTIAL_KEY")
	if keyHex == "" {
		return nil, fmt.Errorf("COMPASS_CREDENTIAL_KEY must be provided and be 64 hex characters")
	}
	key, err := decodeHexKey(keyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode COMPASS_CREDENTIAL_KEY: %w", err)
	}

	cfg.Crypto = CryptoConfig{CredentialKey: key}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func decodeHexKey(input string) ([]byte, error) {
	if len(input) != 64 {
		return nil, fmt.Errorf("encryption key must be 32 bytes encoded as 64 hex characters")
	}
	buf := make([]byte, 32)
	for i := 0; i < 32; i++ {
		n, err := strconv.ParseUint(input[i*2:i*2+2], 16, 8)
		if err != nil {
			return nil, err
		}
		buf[i] = byte(n)
	}
	return buf, nil
}
