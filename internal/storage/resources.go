package storage

import (
	"context"
	"database/sql"
	"encoding/json"
)

// ManagedResourceInput contains the latest observed state for a bundle
// resource.
type ManagedResourceInput struct {
	RepoID       int64
	Address      string
	Kind         string
	SourcePath   string
	NomadID      string
	Namespace    string
	ContentHash  string
	ManifestHash string
	LastCommit   string
	Status       string
	LastError    string
	DeleteMode   string
	Subtype      string
	DependsOn    []string
}

// ManagedResourceStore persists bundle resource ownership and observations.
type ManagedResourceStore struct {
	db *sql.DB
}

// NewManagedResourceStore constructs a managed resource store.
func NewManagedResourceStore(db *sql.DB) *ManagedResourceStore {
	return &ManagedResourceStore{db: db}
}

// Upsert records the latest state for a repository resource address.
func (s *ManagedResourceStore) Upsert(ctx context.Context, input ManagedResourceInput) error {
	dependsOn, err := json.Marshal(input.DependsOn)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO managed_resources
        (repo_id, address, kind, source_path, nomad_id, namespace, content_hash, manifest_hash, last_commit, status, last_error, delete_mode, subtype, depends_on, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(repo_id, address) DO UPDATE SET
          kind = excluded.kind,
          source_path = excluded.source_path,
          nomad_id = excluded.nomad_id,
          namespace = excluded.namespace,
          content_hash = excluded.content_hash,
          manifest_hash = excluded.manifest_hash,
          last_commit = excluded.last_commit,
          status = excluded.status,
          last_error = excluded.last_error,
          delete_mode = excluded.delete_mode,
          subtype = excluded.subtype,
          depends_on = excluded.depends_on,
          updated_at = excluded.updated_at`,
		input.RepoID,
		input.Address,
		input.Kind,
		stringOrNull(input.SourcePath),
		stringOrNull(input.NomadID),
		stringOrNull(input.Namespace),
		stringOrNull(input.ContentHash),
		stringOrNull(input.ManifestHash),
		stringOrNull(input.LastCommit),
		input.Status,
		stringOrNull(input.LastError),
		input.DeleteMode,
		stringOrNull(input.Subtype),
		stringOrNull(string(dependsOn)),
		Now(),
	)
	return err
}

// ListByRepo returns all resources previously observed for a repository.
func (s *ManagedResourceStore) ListByRepo(ctx context.Context, repoID int64) ([]ManagedResource, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, repo_id, address, kind, source_path, nomad_id, namespace, content_hash, manifest_hash, last_commit, status, last_error, delete_mode, subtype, depends_on, updated_at
        FROM managed_resources WHERE repo_id = ? ORDER BY address`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resources []ManagedResource
	for rows.Next() {
		var resource ManagedResource
		var sourcePath sql.NullString
		var resourceDependsOn sql.NullString
		var sourcePath sql.NullString
		if err := rows.Scan(
			&resource.ID,
			&resource.RepoID,
			&resource.Address,
			&resource.Kind,
			&sourcePath,
			&resource.NomadID,
			&resource.Namespace,
			&resource.ContentHash,
			&resource.ManifestHash,
			&resource.LastCommit,
			&resource.Status,
			&resource.LastError,
			&resource.DeleteMode,
			&resource.Subtype,
			&resourceDependsOn,
			&resource.UpdatedAt,
		); err != nil {
			return nil, err
		}
		resource.SourcePath = sourcePath.String
		if resourceDependsOn.Valid && resourceDependsOn.String != "" {
			if err := json.Unmarshal([]byte(resourceDependsOn.String), &resource.DependsOn); err != nil {
				return nil, err
			}
		}
		resources = append(resources, resource)
	}
	return resources, rows.Err()
}

// Delete removes one tracked resource record.
func (s *ManagedResourceStore) Delete(ctx context.Context, repoID int64, address string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM managed_resources WHERE repo_id = ? AND address = ?`, repoID, address)
	return err
}

// DeleteByRepo removes all tracked resources for a repository.
func (s *ManagedResourceStore) DeleteByRepo(ctx context.Context, repoID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM managed_resources WHERE repo_id = ?`, repoID)
	return err
}

func stringOrNull(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
