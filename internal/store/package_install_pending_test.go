package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func pendingRow(subject string, attempt int) PackageInstallPending {
	return PackageInstallPending{
		OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, SubjectDigest: subject, Attempt: attempt, ReleaseID: strings.Repeat("a", 64),
		Package: "demo", Action: "install", Record: []byte(`{"r":1}`), TreePatch: []byte("patch"), Change: []byte(`{}`), OwedRecordID: "owed-" + subject,
	}
}

// FM11: the pending table grows only until rows close; closed rows older than
// 30 days are pruned, open rows never.
func TestPackageInstallPendingClosesAndPrunes(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if err := s.PutPackageInstallPending(ctx, pendingRow("s1", 1)); err != nil {
		t.Fatal(err)
	}
	if err := s.PutPackageInstallPending(ctx, pendingRow("s1", 1)); err == nil {
		t.Fatal("a row was written twice")
	}
	big := pendingRow("big", 1)
	big.TreePatch = make([]byte, pendingMaxBlob+1)
	if err := s.PutPackageInstallPending(ctx, big); err == nil {
		t.Fatal("a patch over the blob bound was stored")
	}
	if err := s.PutPackageInstallPending(ctx, pendingRow("s2", 1)); err != nil {
		t.Fatal(err)
	}
	// The close is one compare-and-set: the second closer loses.
	won, err := s.ClosePackageInstallPending(ctx, inboxTestOwner, inboxTestComputer, "s1", 1, PackageInstallPendingOpen, PackageInstallPendingTimedOut, "late")
	if err != nil || !won {
		t.Fatalf("first close = %v, %v", won, err)
	}
	if won, _ := s.ClosePackageInstallPending(ctx, inboxTestOwner, inboxTestComputer, "s1", 1, PackageInstallPendingOpen, PackageInstallPendingRejected, "x"); won {
		t.Fatal("a closed row closed twice")
	}
	row, err := s.GetPackageInstallPending(ctx, inboxTestOwner, inboxTestComputer, "s1", 1)
	if err != nil || row.State != PackageInstallPendingTimedOut || row.Reason != "late" || row.ClosedAt.IsZero() {
		t.Fatalf("row = %+v, %v", row, err)
	}
	// Nothing prunes before 30 days; everything closed prunes after; the open
	// row never does.
	if n, _ := s.PrunePackageInstallPending(ctx, time.Now().Add(-PackageInstallPendingRetention)); n != 0 {
		t.Fatalf("pruned %d fresh rows", n)
	}
	if n, err := s.PrunePackageInstallPending(ctx, time.Now().Add(PackageInstallPendingRetention)); err != nil || n != 1 {
		t.Fatalf("pruned %d, %v; want the one closed row", n, err)
	}
	if _, err := s.GetPackageInstallPending(ctx, inboxTestOwner, inboxTestComputer, "s1", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pruned row = %v", err)
	}
	if open, _ := s.ListPackageInstallPending(ctx, inboxTestOwner, inboxTestComputer, true); len(open) != 1 || open[0].SubjectDigest != "s2" {
		t.Fatalf("open rows = %+v", open)
	}
	// The crash boot closes the open row (and an accepted one).
	if ok, _ := s.ClosePackageInstallPending(ctx, inboxTestOwner, inboxTestComputer, "s2", 1, PackageInstallPendingOpen, PackageInstallPendingAccepted, ""); !ok {
		t.Fatal("accept")
	}
	closed, err := s.ClosePackageInstallPendingAtBoot(ctx, "interrupted by a restart")
	if err != nil || len(closed) != 1 || closed[0].State != PackageInstallPendingInterrupted {
		t.Fatalf("boot close = %+v, %v", closed, err)
	}
}

// The owed record's own commit mints its re-entry wakes: the record owes a
// tick and the deadline, notice 1 the next tick, and the notice at the cap
// none; an operation-path record owes none (the reconcile pass is its clock).
func TestInstallReviewOwedRecordMintsItsOwnClock(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	facts := types.InstallReviewOwedFacts{ReleaseID: strings.Repeat("a", 64), SubjectDigest: "subj", Package: "demo", Path: types.InstallReviewPathInstall, Attempt: 1}
	mgmt := "management:" + inboxTestOwner
	commit := func(id string, rec types.CommitmentRecord) {
		t.Helper()
		mustCommitInbox(t, s, id, nil, rec)
	}
	commit("owed-1", types.NewInstallReviewOwedRecord(mgmt, inboxTestComputer, facts, "owed"))
	dues := func() map[string]types.InstallReviewDueWake {
		wakes, err := s.ListUnprojectedActorWakes(ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]types.InstallReviewDueWake{}
		for _, w := range wakes {
			if w.Kind != types.InstallReviewDueWakeKind {
				continue
			}
			if w.TargetAgentID != mgmt || w.NotBefore.IsZero() {
				t.Fatalf("wake = %+v", w)
			}
			var due types.InstallReviewDueWake
			if err := json.Unmarshal([]byte(w.Content), &due); err != nil {
				t.Fatal(err)
			}
			out[due.Stage+string(rune('0'+due.Notice))] = due
		}
		return out
	}
	got := dues()
	if len(got) != 2 || got["tick0"].ReleaseID != facts.ReleaseID || got["deadline0"].SubjectDigest != "subj" {
		t.Fatalf("record wakes = %+v", got)
	}
	f1, f2 := facts, facts
	f1.Notice, f2.Notice = 1, 2
	commit("owed-n1", types.NewInstallReviewOwedNotice(mgmt, inboxTestComputer, f1, "n1"))
	if got = dues(); len(got) != 3 || got["tick1"].Notice != 1 {
		t.Fatalf("after notice 1 = %+v", got)
	}
	commit("owed-n2", types.NewInstallReviewOwedNotice(mgmt, inboxTestComputer, f2, "n2"))
	if got = dues(); len(got) != 3 {
		t.Fatalf("the notice at the cap minted a wake: %+v", got)
	}
	op := facts
	op.Path, op.OperationID, op.SubjectDigest = types.InstallReviewPathOperation, "op-1", "subj-op"
	commit("owed-op", types.NewInstallReviewOwedRecord(mgmt, inboxTestComputer, op, "owed"))
	if got = dues(); len(got) != 3 {
		t.Fatalf("an operation-path record minted a wake: %+v", got)
	}
	// The owed records wake management like any system Note.
	if !RecordWakesReader(types.NewInstallReviewOwedRecord(mgmt, inboxTestComputer, facts, "x")) {
		t.Fatal("an owed record does not wake management")
	}
}
