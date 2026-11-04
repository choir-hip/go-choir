package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// The installed set is a fold of the whole adoption history (S12), so the
// listing must return every record, however many: a window would drop a
// package that is still installed and repeat an attempt id.
func TestListReleaseAdoptionsReturnsTheWholeHistory(t *testing.T) {
	s := openTestStore(t)
	old := releaseAdoptionPage
	releaseAdoptionPage = 2
	t.Cleanup(func() { releaseAdoptionPage = old })
	ctx := context.Background()
	base := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 11; i++ {
		if err := s.PutReleaseAdoption(ctx, ReleaseAdoption{
			OwnerID: "owner-a", ComputerID: "computer-a", AdoptionID: fmt.Sprintf("install-%02d", i), ReleaseID: fmt.Sprintf("release-%02d", i),
			RequestedAt: base.Add(time.Duration(i) * time.Minute), Kind: "package", Action: "install", Mode: ReleaseAdoptionModeRuntime,
		}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.ListReleaseAdoptions(ctx, "owner-a", "computer-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 11 || list[0].AdoptionID != "install-10" || list[10].AdoptionID != "install-00" {
		t.Fatalf("listed %d, newest %q, oldest %q", len(list), list[0].AdoptionID, list[len(list)-1].AdoptionID)
	}
}
