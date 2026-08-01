package storage

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/brianmichel/nomad-compass/internal/auth"
)

func testStorageDB(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "storage.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func TestRepoStoreUpdatesMetadataAndHandlesMissingRecords(t *testing.T) {
	db, ctx := testStorageDB(t)
	store := NewRepoStore(db)
	repo, err := store.Create(ctx, RepositoryInput{Name: "repo", RepoURL: "file:///repo", Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCommitMetadata(ctx, repo.ID, "commit-1", "Author <author@example.com>", "Initial"); err != nil {
		t.Fatal(err)
	}
	updated, err := store.Get(ctx, repo.ID)
	if err != nil || updated == nil || !updated.LastCommit.Valid || updated.LastCommit.String != "commit-1" || updated.LastCommitTitle.String != "Initial" {
		t.Fatalf("updated repository = %#v, err = %v", updated, err)
	}
	if err := store.UpdatePollTimestamp(ctx, repo.ID); err != nil {
		t.Fatal(err)
	}
	updated, err = store.Get(ctx, repo.ID)
	if err != nil || updated == nil || !updated.LastPolledAt.Valid {
		t.Fatalf("poll timestamp = %#v, err = %v", updated, err)
	}
	if err := store.UpdateCommitMetadata(ctx, repo.ID, "", "", ""); err != nil {
		t.Fatal(err)
	}
	updated, err = store.Get(ctx, repo.ID)
	if err != nil || updated == nil || updated.LastCommit.Valid || updated.LastCommitAuthor.Valid || updated.LastCommitTitle.Valid {
		t.Fatalf("empty metadata was not cleared: %#v, err = %v", updated, err)
	}
	missing, err := store.Get(ctx, 9999)
	if err != nil || missing != nil {
		t.Fatalf("missing repository = %#v, err = %v", missing, err)
	}
}

func TestRepoFileStoreUpsertAndDeletesRows(t *testing.T) {
	db, ctx := testStorageDB(t)
	store := NewRepoFileStore(db)
	if err := store.UpsertWithDeleteMode(ctx, 1, "jobs/api.nomad", "commit-1", "api", "protect"); err != nil {
		t.Fatal(err)
	}
	if err := store.Upsert(ctx, 1, "jobs/api.nomad", "commit-2", "api-v2"); err != nil {
		t.Fatal(err)
	}
	files, err := store.ListByRepo(ctx, 1)
	if err != nil || len(files) != 1 || files[0].LastCommit.String != "commit-2" || files[0].JobID.String != "api-v2" || files[0].DeleteMode.String != "allow" {
		t.Fatalf("upserted files = %#v, err = %v", files, err)
	}
	if err := store.Delete(ctx, 1, "jobs/api.nomad"); err != nil {
		t.Fatal(err)
	}
	files, err = store.ListByRepo(ctx, 1)
	if err != nil || len(files) != 0 {
		t.Fatalf("files after delete = %#v, err = %v", files, err)
	}
	if err := store.Upsert(ctx, 1, "jobs/a.nomad", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Upsert(ctx, 1, "jobs/b.nomad", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteByRepo(ctx, 1); err != nil {
		t.Fatal(err)
	}
	files, err = store.ListByRepo(ctx, 1)
	if err != nil || len(files) != 0 {
		t.Fatalf("files after repo delete = %#v, err = %v", files, err)
	}
}

func TestManagedResourceStoreDeletesByAddressAndRepository(t *testing.T) {
	db, ctx := testStorageDB(t)
	store := NewManagedResourceStore(db)
	for _, input := range []ManagedResourceInput{
		{RepoID: 1, Address: "job.api", Kind: "job", NomadID: "job-api", Status: "applied", DeleteMode: "allow"},
		{RepoID: 1, Address: "volume.data", Kind: "volume", NomadID: "volume-data", Status: "protected", DeleteMode: "protect"},
		{RepoID: 2, Address: "job.other", Kind: "job", Status: "applied", DeleteMode: "allow"},
	} {
		if err := store.Upsert(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	owners, err := store.ListByNomadID(ctx, "volume", "volume-data")
	if err != nil || len(owners) != 1 || owners[0].Address != "volume.data" {
		t.Fatalf("identity owners = %#v, err = %v", owners, err)
	}
	if err := store.Delete(ctx, 1, "job.api"); err != nil {
		t.Fatal(err)
	}
	resources, err := store.ListByRepo(ctx, 1)
	if err != nil || len(resources) != 1 || resources[0].Address != "volume.data" {
		t.Fatalf("resources after address delete = %#v, err = %v", resources, err)
	}
	if err := store.DeleteByRepo(ctx, 1); err != nil {
		t.Fatal(err)
	}
	resources, err = store.ListByRepo(ctx, 1)
	if err != nil || len(resources) != 0 {
		t.Fatalf("resources after repo delete = %#v, err = %v", resources, err)
	}
	other, err := store.ListByRepo(ctx, 2)
	if err != nil || len(other) != 1 {
		t.Fatalf("other repository resources changed: %#v, err = %v", other, err)
	}
}

func TestCredentialStoreListsWithoutDecryptingAndValidatesTypes(t *testing.T) {
	db, ctx := testStorageDB(t)
	key := []byte("0123456789abcdef0123456789abcdef")
	encryptor, err := auth.NewEncryptor(key)
	if err != nil {
		t.Fatal(err)
	}
	store := NewCredentialStore(db, encryptor)
	if _, err := store.Create(ctx, "z-key", CredentialTypeSSHKey, CredentialPayload{PrivateKey: "key"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, "a-token", CredentialTypeHTTPToken, CredentialPayload{Token: "token"}); err != nil {
		t.Fatal(err)
	}
	credentials, err := store.List(ctx)
	if err != nil || len(credentials) != 2 || credentials[0].Name != "a-token" || credentials[1].Name != "z-key" {
		t.Fatalf("listed credentials = %#v, err = %v", credentials, err)
	}
	if credentials[0].Data == nil || bytes.Equal(credentials[0].Data, []byte("token")) {
		t.Fatal("credential payload was exposed or stored in plaintext")
	}
	for _, tc := range []struct {
		kind    CredentialType
		payload CredentialPayload
	}{
		{CredentialTypeHTTPToken, CredentialPayload{}},
		{CredentialTypeSSHKey, CredentialPayload{}},
		{"unknown", CredentialPayload{}},
	} {
		if err := ValidateCredential(tc.kind, tc.payload); err == nil {
			t.Fatalf("expected validation error for %q", tc.kind)
		}
	}
}
