package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/brianmichel/nomad-compass/internal/auth"
)

func TestCredentialStoreCreateAndDecrypt(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	key := make([]byte, 32)
	copy(key, []byte("0123456789abcdef0123456789abcdef"))

	enc, err := auth.NewEncryptor(key)
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}

	store := NewCredentialStore(db, enc)

	payload := CredentialPayload{Token: "abc123"}
	cred, err := store.Create(context.Background(), "github", CredentialTypeHTTPToken, payload)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if cred.ID == 0 {
		t.Fatal("expected ID to be set")
	}
	if string(cred.Data) == payload.Token {
		t.Fatal("data stored in plaintext")
	}

	fetched, err := store.Get(context.Background(), cred.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected credential")
	}

	decrypted, err := store.DecryptPayload(fetched)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted.Token != payload.Token {
		t.Fatalf("got token %q want %q", decrypted.Token, payload.Token)
	}

	if err := store.Delete(context.Background(), cred.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	removed, err := store.Get(context.Background(), cred.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if removed != nil {
		t.Fatal("expected credential to be deleted")
	}
}

func TestRepoStoreCreate(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewRepoStore(db)
	repo, err := store.Create(context.Background(), RepositoryInput{
		Name:         "repo",
		RepoURL:      "https://example.com/repo.git",
		Branch:       "main",
		CredentialID: sql.NullInt64{},
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}
	if repo.ID == 0 {
		t.Fatal("expected repo id")
	}

	repos, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	key := make([]byte, 32)
	copy(key, []byte("0123456789abcdef0123456789abcdef"))
	enc, err := auth.NewEncryptor(key)
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	credStore := NewCredentialStore(db, enc)
	cred, err := credStore.Create(context.Background(), "git", CredentialTypeHTTPToken, CredentialPayload{Token: "abc"})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	repoWithCred, err := store.Create(context.Background(), RepositoryInput{
		Name:         "secure",
		RepoURL:      "https://example.com/secure.git",
		Branch:       "main",
		CredentialID: sql.NullInt64{Int64: cred.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create repo with credential: %v", err)
	}

	reposForCred, err := store.ListByCredential(context.Background(), cred.ID)
	if err != nil {
		t.Fatalf("list by credential: %v", err)
	}
	if len(reposForCred) != 1 || reposForCred[0].ID != repoWithCred.ID {
		t.Fatalf("expected repo with credential, got %#v", reposForCred)
	}

	if err := store.ClearCredential(context.Background(), cred.ID); err != nil {
		t.Fatalf("clear credential: %v", err)
	}

	reposForCred, err = store.ListByCredential(context.Background(), cred.ID)
	if err != nil {
		t.Fatalf("list by credential after clear: %v", err)
	}
	if len(reposForCred) != 0 {
		t.Fatalf("expected 0 repos after clearing credential, got %d", len(reposForCred))
	}

	if err := store.Delete(context.Background(), repoWithCred.ID); err != nil {
		t.Fatalf("delete repo: %v", err)
	}
}
