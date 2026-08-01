package reconcile

import "testing"

func TestManagedResourceAdaptersCoverSupportedNonJobKinds(t *testing.T) {
	for _, kind := range []string{"volume", "acl_policy", "namespace", "quota", "variable", "sentinel_policy"} {
		adapter, ok := adapterFor(kind)
		if !ok || adapter.Kind() != kind {
			t.Fatalf("missing adapter for %q: %#v", kind, adapter)
		}
	}
}
