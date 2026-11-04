package store

import (
	"context"
	"testing"
	"time"
)

// legacyInstallFixture writes a record the way a tape from before MA2.4 holds
// it: the old kind and the old id field in the body.
func legacyInstallFixture(t *testing.T, s *Store, id string, at time.Time) {
	t.Helper()
	body := map[string]any{
		"adoption_id": id, "owner_id": "owner-a", "computer_id": "computer-a", "release_id": "release-" + id,
		"adopter_base_commit": "", "adopter_tree_hash": "", "trajectory_id": "",
		"requested_at": at.Format(time.RFC3339Nano), "kind": "package", "package": "files-next", "action": "install",
		"attempt": 1, "mode": "runtime", "module_sha256": "abc",
	}
	meta := map[string]any{"computer_id": "computer-a", "adoption_id": id, "release_id": "release-" + id, "requested_at": at.Format(time.RFC3339Nano)}
	if _, err := s.ogPut(context.Background(), legacyInstallKind, "owner-a", packageInstallKey("computer-a", id), body, meta, at); err != nil {
		t.Fatal(err)
	}
}

// An old record reads as an install (get and list) and a mixed history lists
// each record once; nothing old is rewritten.
func TestLegacyInstallRecordsReadThroughTheFold(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	legacyInstallFixture(t, s, "install-old-1", base)
	legacyInstallFixture(t, s, "install-old-2", base.Add(time.Minute))
	if err := s.PutPackageInstall(ctx, PackageInstall{
		OwnerID: "owner-a", ComputerID: "computer-a", InstallID: "install-new-3", ReleaseID: "release-3",
		RequestedAt: base.Add(2 * time.Minute), Kind: "package", Action: "install", Mode: PackageInstallModeRuntime,
	}); err != nil {
		t.Fatal(err)
	}

	ids := func() []string {
		list, err := s.ListPackageInstalls(ctx, "owner-a", "computer-a")
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, item := range list {
			out = append(out, item.InstallID)
		}
		return out
	}
	want := []string{"install-new-3", "install-old-2", "install-old-1"}
	check := func(when string) {
		got := ids()
		if len(got) != len(want) {
			t.Fatalf("%s: listed %v, want %v", when, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: listed %v, want %v", when, got, want)
			}
		}
	}
	check("before")
	one, err := s.GetPackageInstall(ctx, "owner-a", "computer-a", "install-old-1")
	if err != nil || one.InstallID != "install-old-1" || one.ReleaseID != "release-install-old-1" || one.ModuleSHA256 != "abc" {
		t.Fatalf("get old record = %+v, %v", one, err)
	}
	// A new record cannot reuse an old id.
	if err := s.PutPackageInstall(ctx, PackageInstall{OwnerID: "owner-a", ComputerID: "computer-a", InstallID: "install-old-1", ReleaseID: "r", Mode: PackageInstallModeRuntime}); err == nil {
		t.Fatal("an old id was reused")
	}

	// Writes use the new kind only; nothing old is moved or tombstoned.
	left, err := s.ogListAllObjectsByKind(ctx, legacyInstallKind)
	if err != nil || len(left) != 2 {
		t.Fatalf("legacy records = %d, %v; want 2 untouched", len(left), err)
	}
	for _, obj := range left {
		if obj.Tombstone {
			t.Fatalf("legacy record tombstoned: %s", obj.CanonicalID)
		}
	}
	// A record present under both kinds lists once, as the new kind.
	old, _ := s.GetPackageInstall(ctx, "owner-a", "computer-a", "install-old-1")
	if _, err := s.ogPut(ctx, ogKindPackageInstall, "owner-a", packageInstallKey("computer-a", "install-old-1"), old, packageInstallMetadata(old), old.RequestedAt); err != nil {
		t.Fatal(err)
	}
	check("with a duplicate under both kinds")
}
