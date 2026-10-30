package store

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The cell's inbox read and the claim pack read by index (C3). Ways they
// can fail, written before the code:
//
//  1. The sequence range drops a record the whole-inbox read returned for
//     the same window, or adds one (another owner, computer, inbox).
//  2. Records come back in another order.
//  3. The claim pack loses or gains a record, or a record of another
//     computer or a directive slips in.
//  4. The store uses the index before the migration made it live.
//
// "Before" is the same store read through an object-graph handle that never
// learned the generated columns: exactly the old SQL.
func TestIndexedInboxAndClaimReadsMatchUnindexed(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	rng := rand.New(rand.NewSource(1014))
	management := "management:" + inboxTestOwner
	inboxes := []string{management, "texture:d1", "texture:d2", "research:d1"}
	var batch []types.CommitmentRecord
	for i := 0; i < 160; i++ {
		from, to := inboxes[rng.Intn(len(inboxes))], inboxes[rng.Intn(len(inboxes))]
		if from == to {
			continue
		}
		id := fmt.Sprintf("idx-%03d", i)
		switch rng.Intn(3) {
		case 0:
			batch = append(batch, inboxNote(id, from, to))
		case 1:
			batch = append(batch, inboxAsk(id, from, to))
		default:
			batch = append(batch, inboxReport(id, from, to, false, types.CommitmentReport{Claims: []types.ReportClaim{{Text: "claim " + id}}}))
		}
		if len(batch) == 40 {
			mustCommitInbox(t, s, fmt.Sprintf("idx-cmd-%d", i), nil, batch...)
			batch = nil
		}
	}
	if len(batch) > 0 {
		mustCommitInbox(t, s, "idx-cmd-last", nil, batch...)
	}
	// Records of the older writer (no inbox metadata) and of another
	// computer must not leak into either read.
	for i := 0; i < 5; i++ {
		rec := inboxAsk(fmt.Sprintf("legacy-%d", i), "research:d1", management)
		if _, err := s.AppendCommitmentRecord(ctx, inboxTestOwner, inboxTestComputer, rec); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AppendCommitmentRecord(ctx, inboxTestOwner, "computer-other", rec); err != nil {
			t.Fatal(err)
		}
	}

	live := s.ogReadStore
	unmigrated := objectgraph.NewDoltStore(s.readDB)
	unmigrated.ShareEngineMutex(live)
	read := func(graph *objectgraph.SQLStore, fn func() (any, error)) string {
		s.ogReadStore = graph
		defer func() { s.ogReadStore = live }()
		v, err := fn()
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%+v", v)
	}
	ids := func(recs []types.CommitmentRecord) []string {
		out := make([]string, len(recs))
		for i, r := range recs {
			out[i] = fmt.Sprintf("%s#%d", r.RecordID, r.InboxSeq)
		}
		return out
	}
	compared := 0
	for _, inbox := range inboxes {
		head, err := s.InboxHead(ctx, inboxTestOwner, inboxTestComputer, inbox)
		if err != nil {
			t.Fatal(err)
		}
		for _, after := range []int64{0, 1, head / 2, head - 1, head} {
			for _, through := range []int64{0, head / 2, head, head + 5} {
				list := func() (any, error) {
					recs, err := s.ListInboxRecords(ctx, inboxTestOwner, inboxTestComputer, inbox, after, through)
					return ids(recs), err
				}
				want, got := read(unmigrated, list), read(live, list)
				if got != want {
					t.Fatalf("inbox %s (%d, %d]:\n got %s\nwant %s", inbox, after, through, got, want)
				}
				compared++
			}
		}
	}
	claims := func() (any, error) {
		recs, err := s.ListCommitmentClaimRecords(ctx, inboxTestOwner, inboxTestComputer, 1000)
		got := make([]string, len(recs))
		for i, r := range recs {
			got[i] = r.RecordID
		}
		// Records committed in one command share their second; the old
		// read left their order to the engine, so compare the set here (the
		// object-graph test checks the order on distinct times).
		sort.Strings(got)
		return got, err
	}
	want, got := read(unmigrated, claims), read(live, claims)
	if got != want {
		t.Fatalf("claim pack:\n got %s\nwant %s", got, want)
	}
	if compared == 0 || len(want) < 50 {
		t.Fatalf("fixture too thin: %d windows, claims %s", compared, want)
	}
}
