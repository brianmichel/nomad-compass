package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/brianmichel/nomad-compass/internal/auth"
)

// CredentialPayload holds clear-text values before encryption.
type CredentialPayload struct {
	Token      string `json:"token,omitempty"`
	Username   string `json:"username,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// CredentialStore manages credential persistence.
type CredentialStore struct {
	db        *sql.DB
	encryptor *auth.Encryptor
}

// NewCredentialStore constructs a credential store.
func NewCredentialStore(db *sql.DB, encryptor *auth.Encryptor) *CredentialStore {
	return &CredentialStore{db: db, encryptor: encryptor}
}

// Create stores a new credential entry.
func (s *CredentialStore) Create(ctx context.Context, name string, ctype CredentialType, payload CredentialPayload) (*Credential, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	cipher, err := s.encryptor.Encrypt(raw)
	if err != nil {
		return nil, fmt.Errorf("encrypt payload: %w", err)
	}

	now := Now()
	res, err := s.db.ExecContext(ctx, `INSERT INTO credentials (name, type, data, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, name, string(ctype), cipher, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Credential{
		ID:        id,
		Name:      name,
		Type:      ctype,
		Data:      cipher,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// List returns all credentials without decrypting the payloads.
func (s *CredentialStore) List(ctx context.Context) ([]Credential, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, type, data, created_at, updated_at FROM credentials ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Credential
	for rows.Next() {
		var c Credential
		var typ string
		if err := rows.Scan(&c.ID, &c.Name, &typ, &c.Data, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Type = CredentialType(typ)
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get fetches a credential by ID without decrypting data.
func (s *CredentialStore) Get(ctx context.Context, id int64) (*Credential, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, type, data, created_at, updated_at FROM credentials WHERE id = ?`, id)
	var c Credential
	var typ string
	if err := row.Scan(&c.ID, &c.Name, &typ, &c.Data, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	c.Type = CredentialType(typ)
	return &c, nil
}

// DecryptPayload returns the decrypted payload for a credential.
func (s *CredentialStore) DecryptPayload(c *Credential) (*CredentialPayload, error) {
	raw, err := s.encryptor.Decrypt(c.Data)
	if err != nil {
		return nil, err
	}
	var payload CredentialPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// Delete removes a credential by ID.
func (s *CredentialStore) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM credentials WHERE id = ?`, id)
	return err
}
