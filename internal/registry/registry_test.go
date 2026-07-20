package registry

import "testing"

func TestRegisterAndDiscover(t *testing.T) {
	r := New("node-1")

	r.Register("svc-a", "10.0.0.1:8080", nil)

	entry, ok := r.Discover("svc-a")
	if !ok {
		t.Fatalf("expected svc-a to be discoverable")
	}
	if entry.Endpoint != "10.0.0.1:8080" {
		t.Errorf("unexpected endpoint: %s", entry.Endpoint)
	}
	if entry.Version.Counter != 1 || entry.Version.OwnerNodeID != "node-1" {
		t.Errorf("unexpected version: %+v", entry.Version)
	}
}

func TestRegisterIncrementsCounterOnUpdate(t *testing.T) {
	r := New("node-1")

	r.Register("svc-a", "10.0.0.1:8080", nil)
	entry := r.Register("svc-a", "10.0.0.1:9090", nil)

	if entry.Version.Counter != 2 {
		t.Errorf("expected counter 2 after second register, got %d", entry.Version.Counter)
	}
	if entry.Endpoint != "10.0.0.1:9090" {
		t.Errorf("expected updated endpoint, got %s", entry.Endpoint)
	}
}

func TestDeregisterMarksRemovedAndHidesFromDiscoverAndList(t *testing.T) {
	r := New("node-1")
	r.Register("svc-a", "10.0.0.1:8080", nil)

	entry, ok := r.Deregister("svc-a")
	if !ok {
		t.Fatalf("expected deregister to succeed")
	}
	if entry.Status != StatusRemoved {
		t.Errorf("expected status REMOVED, got %s", entry.Status)
	}

	if _, ok := r.Discover("svc-a"); ok {
		t.Errorf("expected svc-a to not be discoverable after deregister")
	}
	if len(r.List()) != 0 {
		t.Errorf("expected List to exclude removed entries")
	}
	// Il tombstone deve comunque restare nello Snapshot per la propagazione via gossip.
	if len(r.Snapshot()) != 1 {
		t.Errorf("expected Snapshot to still contain the tombstone")
	}
}

func TestDeregisterUnknownServiceIsNoop(t *testing.T) {
	r := New("node-1")

	if _, ok := r.Deregister("does-not-exist"); ok {
		t.Errorf("expected deregister of unknown service to report ok=false")
	}
}

func TestMergeAppliesNewerVersion(t *testing.T) {
	r := New("node-1")
	older := ServiceEntry{
		ServiceID: "svc-a",
		Endpoint:  "10.0.0.1:8080",
		Version:   Version{Counter: 1, OwnerNodeID: "node-2"},
		Status:    StatusActive,
	}
	r.Merge(older)

	newer := ServiceEntry{
		ServiceID: "svc-a",
		Endpoint:  "10.0.0.1:9999",
		Version:   Version{Counter: 2, OwnerNodeID: "node-2"},
		Status:    StatusActive,
	}
	applied := r.Merge(newer)

	if !applied {
		t.Errorf("expected newer version to be applied")
	}
	entry, _ := r.Discover("svc-a")
	if entry.Endpoint != "10.0.0.1:9999" {
		t.Errorf("expected merged endpoint, got %s", entry.Endpoint)
	}
}

func TestMergeRejectsOlderOrEqualVersion(t *testing.T) {
	r := New("node-1")
	current := ServiceEntry{
		ServiceID: "svc-a",
		Endpoint:  "10.0.0.1:8080",
		Version:   Version{Counter: 5, OwnerNodeID: "node-2"},
		Status:    StatusActive,
	}
	r.Merge(current)

	stale := ServiceEntry{
		ServiceID: "svc-a",
		Endpoint:  "10.0.0.1:1111",
		Version:   Version{Counter: 3, OwnerNodeID: "node-2"},
		Status:    StatusActive,
	}
	if applied := r.Merge(stale); applied {
		t.Errorf("expected stale version to be rejected")
	}

	entry, _ := r.Discover("svc-a")
	if entry.Endpoint != "10.0.0.1:8080" {
		t.Errorf("expected original endpoint to be retained, got %s", entry.Endpoint)
	}
}

func TestVersionTieBreakByOwnerNodeID(t *testing.T) {
	a := Version{Counter: 1, OwnerNodeID: "node-a"}
	b := Version{Counter: 1, OwnerNodeID: "node-b"}

	if a.IsNewerThan(b) {
		t.Errorf("expected node-a version to lose the tie-break against node-b")
	}
	if !b.IsNewerThan(a) {
		t.Errorf("expected node-b version to win the tie-break against node-a")
	}
}

func TestRemoveEntriesOwnedByMarksThemRemoved(t *testing.T) {
	r := New("node-1")
	r.Merge(ServiceEntry{
		ServiceID: "svc-a",
		Endpoint:  "10.0.0.1:8080",
		Version:   Version{Counter: 1, OwnerNodeID: "node-2"},
		Status:    StatusActive,
	})
	r.Merge(ServiceEntry{
		ServiceID: "svc-b",
		Endpoint:  "10.0.0.1:9090",
		Version:   Version{Counter: 1, OwnerNodeID: "node-3"},
		Status:    StatusActive,
	})

	removed := r.RemoveEntriesOwnedBy("node-2")

	if len(removed) != 1 || removed[0].ServiceID != "svc-a" {
		t.Fatalf("expected only svc-a to be removed, got %+v", removed)
	}
	if _, ok := r.Discover("svc-a"); ok {
		t.Errorf("expected svc-a to no longer be discoverable")
	}
	if _, ok := r.Discover("svc-b"); !ok {
		t.Errorf("expected svc-b (owned by a different node) to remain discoverable")
	}
}
