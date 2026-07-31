package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestManagedResourceDependenciesRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "resources.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	store := NewManagedResourceStore(db)
	if err := store.Upsert(ctx, ManagedResourceInput{
		RepoID:     1,
		Address:    "job.web",
		Kind:       "job",
		Status:     "applied",
		DeleteMode: "allow",
		DependsOn:  `["volume.data","acl_policy.web"]`,
	}); err != nil {
		t.Fatalf("upsert resource: %v", err)
	}
	resources, err := store.ListByRepo(ctx, 1)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if len(resources) != 1 || !resources[0].DependsOn.Valid || resources[0].DependsOn.String != `["volume.data","acl_policy.web"]` {
		t.Fatalf("dependencies did not round trip: %#v", resources)
	}
	if resources[0].SourcePath != "" {
		t.Fatalf("expected nullable source path to remain empty, got %q", resources[0].SourcePath)
	}
}

func TestManagedResourceDependenciesMigrateExistingSchema(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "legacy.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE managed_resources (
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
        UNIQUE(repo_id, address)
    )`)
	if err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate legacy schema: %v", err)
	}

	store := NewManagedResourceStore(db)
	if err := store.Upsert(ctx, ManagedResourceInput{RepoID: 2, Address: "job.legacy", Kind: "job", Status: "applied", DeleteMode: "allow", DependsOn: `[]`}); err != nil {
		t.Fatalf("upsert migrated resource: %v", err)
	}
	resources, err := store.ListByRepo(ctx, 2)
	if err != nil || len(resources) != 1 || !resources[0].DependsOn.Valid || resources[0].DependsOn.String != `[]` {
		t.Fatalf("migrated dependency column unavailable: %v %#v", err, resources)
	}
}
