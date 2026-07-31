package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Open establishes a SQLite connection with sensible defaults.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("ensure database directory: %w", err)
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout=5000&_pragma=journal_mode=WAL", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(0)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}

// Migrate ensures the database schema is present.
func Migrate(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS credentials (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE,
            type TEXT NOT NULL,
            data BLOB NOT NULL,
            created_at TIMESTAMP NOT NULL,
            updated_at TIMESTAMP NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS repos (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            repo_url TEXT NOT NULL,
            branch TEXT NOT NULL,
            job_path TEXT NOT NULL DEFAULT '.nomad',
            credential_id INTEGER,
            created_at TIMESTAMP NOT NULL,
            updated_at TIMESTAMP NOT NULL,
            last_commit TEXT,
            last_commit_author TEXT,
            last_commit_title TEXT,
            last_polled_at TIMESTAMP,
            FOREIGN KEY (credential_id) REFERENCES credentials(id)
        )`,
		`CREATE TABLE IF NOT EXISTS repo_files (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            repo_id INTEGER NOT NULL,
            path TEXT NOT NULL,
            last_commit TEXT,
            updated_at TIMESTAMP NOT NULL,
            job_id TEXT,
            delete_mode TEXT NOT NULL DEFAULT 'allow',
            UNIQUE(repo_id, path),
            FOREIGN KEY(repo_id) REFERENCES repos(id)
        )`,
		`CREATE TABLE IF NOT EXISTS managed_resources (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            repo_id INTEGER NOT NULL,
            address TEXT NOT NULL,
            kind TEXT NOT NULL,
            source_path TEXT,
            nomad_id TEXT,
            namespace TEXT,
            content_hash TEXT,
            manifest_hash TEXT,
            last_commit TEXT,
            status TEXT NOT NULL,
            last_error TEXT,
            delete_mode TEXT NOT NULL DEFAULT 'protect',
            subtype TEXT,
            updated_at TIMESTAMP NOT NULL,
            UNIQUE(repo_id, address),
            FOREIGN KEY(repo_id) REFERENCES repos(id)
        )`,
		`ALTER TABLE repos ADD COLUMN job_path TEXT NOT NULL DEFAULT '.nomad'`,
		`ALTER TABLE repo_files ADD COLUMN job_id TEXT`,
		`ALTER TABLE repo_files ADD COLUMN delete_mode TEXT NOT NULL DEFAULT 'allow'`,
		`ALTER TABLE managed_resources ADD COLUMN manifest_hash TEXT`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			// Ignore SQLite errors when columns already exist.
			if !isIgnorableMigrationError(err) {
				return err
			}
		}
	}

	return nil
}

func isIgnorableMigrationError(err error) bool {
	if err == nil {
		return false
	}
	// SQLite returns errors containing these substrings when an ALTER has already been applied.
	msg := err.Error()
	return strings.Contains(msg, "duplicate column name") || strings.Contains(msg, "already exists")
}

// Now returns a UTC timestamp helper.
func Now() time.Time {
	return time.Now().UTC()
}
