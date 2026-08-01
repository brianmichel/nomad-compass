package reconcile

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/brianmichel/nomad-compass/internal/auth"
	"github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestDeleteCredentialClearsOrDeletesLinkedRepositories(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "credentials.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	encryptor, err := auth.NewEncryptor([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	creds := storage.NewCredentialStore(db, encryptor)
	repos := storage.NewRepoStore(db)
	files := storage.NewRepoFileStore(db)
	managed := storage.NewManagedResourceStore(db)
	manager := New(repos, files, managed, creds, repo.NewManager(filepath.Join(t.TempDir(), "clones")), &fakeNomad{}, 0, slog.New(slog.NewTextHandler(io.Discard, nil)))

	credential, err := creds.Create(ctx, "shared", storage.CredentialTypeHTTPToken, storage.CredentialPayload{Token: "token"})
	if err != nil {
		t.Fatal(err)
	}
	linked, err := repos.Create(ctx, storage.RepositoryInput{Name: "linked", RepoURL: "file:///linked", Branch: "main", CredentialID: sql.NullInt64{Int64: credential.ID, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.DeleteCredential(ctx, credential.ID, false, false); err != nil {
		t.Fatalf("clear linked credential: %v", err)
	}
	if remaining, err := creds.Get(ctx, credential.ID); err != nil || remaining != nil {
		t.Fatalf("credential remains after clear: %#v, %v", remaining, err)
	}
	linked, err = repos.Get(ctx, linked.ID)
	if err != nil || linked == nil || linked.CredentialID.Valid {
		t.Fatalf("repository credential was not cleared: %#v, %v", linked, err)
	}

	credential, err = creds.Create(ctx, "owned", storage.CredentialTypeHTTPToken, storage.CredentialPayload{Token: "token"})
	if err != nil {
		t.Fatal(err)
	}
	owned, err := repos.Create(ctx, storage.RepositoryInput{Name: "owned", RepoURL: "file:///owned", Branch: "main", CredentialID: sql.NullInt64{Int64: credential.ID, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.DeleteCredential(ctx, credential.ID, true, false); err != nil {
		t.Fatalf("delete linked repository: %v", err)
	}
	if remaining, err := repos.Get(ctx, owned.ID); err != nil || remaining != nil {
		t.Fatalf("linked repository remains after delete: %#v, %v", remaining, err)
	}
}
