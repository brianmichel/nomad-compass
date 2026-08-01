package storage

import (
	"database/sql"
	"time"
)

// CredentialType enumerates supported authentication mechanisms.
type CredentialType string

const (
	CredentialTypeHTTPToken CredentialType = "https-token"
	CredentialTypeSSHKey    CredentialType = "ssh-key"
)

// Credential stores encrypted authentication materials.
type Credential struct {
	ID        int64
	Name      string
	Type      CredentialType
	Data      []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Repository describes a tracked git repository.
type Repository struct {
	ID               int64
	Name             string
	RepoURL          string
	Branch           string
	JobPath          string
	CredentialID     sql.NullInt64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastCommit       sql.NullString
	LastCommitAuthor sql.NullString
	LastCommitTitle  sql.NullString
	LastPolledAt     sql.NullTime
}

// RepoFile tracks metadata for job files inside a repository.
type RepoFile struct {
	ID         int64
	RepoID     int64
	Path       string
	LastCommit sql.NullString
	UpdatedAt  time.Time
	JobID      sql.NullString
	DeleteMode sql.NullString
}

// ManagedResource tracks a non-file-backed or embedded resource in a bundle.
type ManagedResource struct {
	ID           int64
	RepoID       int64
	Address      string
	Kind         string
	SourcePath   string
	NomadID      sql.NullString
	Namespace    sql.NullString
	ContentHash  sql.NullString
	ManifestHash sql.NullString
	LastCommit   sql.NullString
	Status       string
	LastError    sql.NullString
	DeleteMode   string
	Subtype      sql.NullString
	DependsOn    sql.NullString
	UpdatedAt    time.Time
}
