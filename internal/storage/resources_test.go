package storage

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestManagedResourceDependenciesRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "resources.sqlite"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	if err := Migrate(ctx, db); err != nil { t.Fatal(err) }
	store := NewManagedResourceStore(db)
	want := []string{"volume.data", "acl_policy.apps"}
	if err := store.Upsert(ctx, ManagedResourceInput{RepoID: 1, Address: "job.app", Kind: "job", Status: "applied", DependsOn: want}); err != nil { t.Fatal(err) }
	resources, err := store.ListByRepo(ctx, 1)
	if err != nil { t.Fatal(err) }
	if len(resources) != 1 || !reflect.DeepEqual(resources[0].DependsOn, want) { t.Fatalf("dependencies = %#v, want %#v", resources, want) }
}
