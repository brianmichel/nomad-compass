package server

import (
	"database/sql"
	"time"

	"github.com/brianmichel/nomad-compass/internal/storage"
)

type credentialResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newCredentialResponse(c storage.Credential) credentialResponse {
	return credentialResponse{
		ID:        c.ID,
		Name:      c.Name,
		Type:      string(c.Type),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

type repositoryResponse struct {
	ID               int64                   `json:"id"`
	Name             string                  `json:"name"`
	RepoURL          string                  `json:"repo_url"`
	Branch           string                  `json:"branch"`
	CredentialID     *int64                  `json:"credential_id,omitempty"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
	LastCommit       *string                 `json:"last_commit,omitempty"`
	LastCommitAuthor *string                 `json:"last_commit_author,omitempty"`
	LastCommitTitle  *string                 `json:"last_commit_title,omitempty"`
	LastPolledAt     *time.Time              `json:"last_polled_at,omitempty"`
	Jobs             []repositoryJobResponse `json:"jobs"`
}

func newRepositoryResponse(repo storage.Repository) repositoryResponse {
	return repositoryResponse{
		ID:               repo.ID,
		Name:             repo.Name,
		RepoURL:          repo.RepoURL,
		Branch:           repo.Branch,
		CredentialID:     nullableInt64(repo.CredentialID),
		CreatedAt:        repo.CreatedAt,
		UpdatedAt:        repo.UpdatedAt,
		LastCommit:       nullableString(repo.LastCommit),
		LastCommitAuthor: nullableString(repo.LastCommitAuthor),
		LastCommitTitle:  nullableString(repo.LastCommitTitle),
		LastPolledAt:     nullableTime(repo.LastPolledAt),
		Jobs:             []repositoryJobResponse{},
	}
}

type repositoryJobResponse struct {
	Path              string    `json:"path"`
	JobID             string    `json:"job_id,omitempty"`
	JobName           string    `json:"job_name,omitempty"`
	LastCommit        *string   `json:"last_commit,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
	Status            string    `json:"status,omitempty"`
	StatusDescription string    `json:"status_description,omitempty"`
	StatusError       string    `json:"status_error,omitempty"`
}

func newRepositoryJobResponse(file storage.RepoFile) repositoryJobResponse {
	return repositoryJobResponse{
		Path:       file.Path,
		LastCommit: nullableString(file.LastCommit),
		UpdatedAt:  file.UpdatedAt,
	}
}

type statusResponse struct {
	NomadConnected bool   `json:"nomad_connected"`
	NomadMessage   string `json:"nomad_message,omitempty"`
}

func nullableInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	val := v.Int64
	return &val
}

func nullableString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	val := v.String
	return &val
}

func nullableTime(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	val := v.Time
	return &val
}
