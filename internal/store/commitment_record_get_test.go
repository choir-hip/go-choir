package store

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6: the
// paper's engine mutex was held 2.3 s per ListObjectsByOwnerAndBody call, a
// JSON scan of every commitment record body. GetCommitmentRecord asked it for
// one record by $.record_id, although every writer keys the record's object
// on that id. Failure modes of reading by the key instead:
//  1. a record committed by a writer is not found (a writer keys it some
//     other way);
//  2. a record of another computer of the same owner is returned;
//  3. an absent id errors instead of reading nil;
//  4. the read still scans record bodies.

func TestGetCommitmentRecordReadsByKeyWithoutABodyScan(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	appended := types.CommitmentRecord{RecordID: "get-by-key-1", SchemaID: "test", Provenance: types.CommitmentProvenance{AgentID: "management:" + storyOwner}}
	if _, err := s.AppendCommitmentRecord(ctx, storyOwner, storyComputer, appended); err != nil {
		t.Fatal(err)
	}
	other := types.CommitmentRecord{RecordID: "get-by-key-other", SchemaID: "test", Provenance: types.CommitmentProvenance{AgentID: "management:" + storyOwner}}
	if _, err := s.AppendCommitmentRecord(ctx, storyOwner, "another-computer", other); err != nil {
		t.Fatal(err)
	}
	receipt := mustCommitInbox(t, s, "cell-records:get-by-key", nil, openingCast("get-by-key:cast:c1"))
	inboxRecord := receipt.Records[0].RecordID

	scansBefore := bodyScanCalls(s)
	for _, id := range []string{appended.RecordID, inboxRecord} {
		rec, err := s.GetCommitmentRecord(ctx, storyOwner, storyComputer, id)
		if err != nil || rec == nil || rec.RecordID != id {
			t.Fatalf("GetCommitmentRecord(%s) = %+v, %v; want the record", id, rec, err)
		}
	}
	if rec, err := s.GetCommitmentRecord(ctx, storyOwner, storyComputer, other.RecordID); err != nil || rec != nil {
		t.Fatalf("another computer's record = %+v, %v; want nil", rec, err)
	}
	if rec, err := s.GetCommitmentRecord(ctx, storyOwner, storyComputer, "no-such-record"); err != nil || rec != nil {
		t.Fatalf("absent record = %+v, %v; want nil, nil", rec, err)
	}
	if scans := bodyScanCalls(s) - scansBefore; scans != 0 {
		t.Fatalf("GetCommitmentRecord made %d body scans; want reads by key", scans)
	}
}

// bodyScanCalls counts ListObjectsByOwnerAndBody calls over every path name.
func bodyScanCalls(s *Store) uint64 {
	var n uint64
	for op, st := range s.EngineMutexStats() {
		if strings.HasPrefix(op, "(*DoltStore).ListObjectsByOwnerAndBody") {
			n += st.Calls
		}
	}
	return n
}
