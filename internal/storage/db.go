package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Open establishes a SQLite connection with sensible defaults.
func Open(path string) (*sql.DB, error) {
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
            UNIQUE(repo_id, path),
            FOREIGN KEY(repo_id) REFERENCES repos(id)
        )`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

// Now returns a UTC timestamp helper.
func Now() time.Time {
	return time.Now().UTC()
}
