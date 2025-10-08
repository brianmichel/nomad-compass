package storage

import (
	"context"
	"database/sql"
	"errors"
)

// RepositoryInput is used when creating a repository record.
type RepositoryInput struct {
	Name         string
	RepoURL      string
	Branch       string
	CredentialID sql.NullInt64
}

// RepoStore manages repository persistence.
type RepoStore struct {
	db *sql.DB
}

// NewRepoStore constructs a repository store.
func NewRepoStore(db *sql.DB) *RepoStore {
	return &RepoStore{db: db}
}

// Create inserts a new repository entry.
func (s *RepoStore) Create(ctx context.Context, input RepositoryInput) (*Repository, error) {
	now := Now()
	res, err := s.db.ExecContext(ctx, `INSERT INTO repos (name, repo_url, branch, credential_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		input.Name, input.RepoURL, input.Branch, nullable(input.CredentialID), now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	repo := &Repository{
		ID:           id,
		Name:         input.Name,
		RepoURL:      input.RepoURL,
		Branch:       input.Branch,
		CredentialID: input.CredentialID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return repo, nil
}

// List returns all repositories.
func (s *RepoStore) List(ctx context.Context) ([]Repository, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, repo_url, branch, credential_id, created_at, updated_at, last_commit, last_commit_author, last_commit_title, last_polled_at FROM repos ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var repo Repository
		if err := rows.Scan(
			&repo.ID,
			&repo.Name,
			&repo.RepoURL,
			&repo.Branch,
			&repo.CredentialID,
			&repo.CreatedAt,
			&repo.UpdatedAt,
			&repo.LastCommit,
			&repo.LastCommitAuthor,
			&repo.LastCommitTitle,
			&repo.LastPolledAt,
		); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

// Get fetches a repository by ID.
func (s *RepoStore) Get(ctx context.Context, id int64) (*Repository, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, repo_url, branch, credential_id, created_at, updated_at, last_commit, last_commit_author, last_commit_title, last_polled_at FROM repos WHERE id = ?`, id)
	var repo Repository
	if err := row.Scan(
		&repo.ID,
		&repo.Name,
		&repo.RepoURL,
		&repo.Branch,
		&repo.CredentialID,
		&repo.CreatedAt,
		&repo.UpdatedAt,
		&repo.LastCommit,
		&repo.LastCommitAuthor,
		&repo.LastCommitTitle,
		&repo.LastPolledAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &repo, nil
}

// UpdateCommitMetadata stores the latest reconciliation data.
func (s *RepoStore) UpdateCommitMetadata(ctx context.Context, id int64, commit, author, title string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE repos SET last_commit = ?, last_commit_author = ?, last_commit_title = ?, last_polled_at = ?, updated_at = ? WHERE id = ?`,
		commitOrNull(commit), commitOrNull(author), commitOrNull(title), Now(), Now(), id)
	return err
}

func nullable(val sql.NullInt64) interface{} {
	if val.Valid {
		return val.Int64
	}
	return nil
}

func commitOrNull(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

// UpdatePollTimestamp updates only the poll timestamp for scenarios where no change occurred.
func (s *RepoStore) UpdatePollTimestamp(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE repos SET last_polled_at = ?, updated_at = ? WHERE id = ?`, Now(), Now(), id)
	return err
}

// RepoFileStore manages job file metadata.
type RepoFileStore struct {
	db *sql.DB
}

// NewRepoFileStore constructs a repo file store.
func NewRepoFileStore(db *sql.DB) *RepoFileStore {
	return &RepoFileStore{db: db}
}

// Upsert stores or updates repo file metadata.
func (s *RepoFileStore) Upsert(ctx context.Context, repoID int64, path string, commit string) error {
	now := Now()
	_, err := s.db.ExecContext(ctx, `INSERT INTO repo_files (repo_id, path, last_commit, updated_at) VALUES (?, ?, ?, ?)
        ON CONFLICT(repo_id, path) DO UPDATE SET last_commit = excluded.last_commit, updated_at = excluded.updated_at`, repoID, path, commitOrNull(commit), now)
	return err
}

// ListByRepo returns tracked files for a repo.
func (s *RepoFileStore) ListByRepo(ctx context.Context, repoID int64) ([]RepoFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, repo_id, path, last_commit, updated_at FROM repo_files WHERE repo_id = ?`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []RepoFile
	for rows.Next() {
		var file RepoFile
		if err := rows.Scan(&file.ID, &file.RepoID, &file.Path, &file.LastCommit, &file.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}
