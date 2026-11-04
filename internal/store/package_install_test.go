package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// The installed set is a fold of the whole install history (S12), so the
// listing must return every record, however many: a window would drop a
// package that is still installed and repeat an attempt id.
func TestListPackageInstallsReturnsTheWholeHistory(t *testing.T) {
	s := openTestStore(t)
	old := packageInstallPage
	packageInstallPage = 2
	t.Cleanup(func() { packageInstallPage = old })
	ctx := context.Background()
	base := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 11; i++ {
		if err := s.PutPackageInstall(ctx, PackageInstall{
			OwnerID: "owner-a", ComputerID: "computer-a", InstallID: fmt.Sprintf("install-%02d", i), ReleaseID: fmt.Sprintf("release-%02d", i),
			RequestedAt: base.Add(time.Duration(i) * time.Minute), Kind: "package", Action: "install", Mode: PackageInstallModeRuntime,
		}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.ListPackageInstalls(ctx, "owner-a", "computer-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 11 || list[0].InstallID != "install-10" || list[10].InstallID != "install-00" {
		t.Fatalf("listed %d, newest %q, oldest %q", len(list), list[0].InstallID, list[len(list)-1].InstallID)
	}
}
