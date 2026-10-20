package maild

// Failure modes this file guards (design sections 4.5 and 8). Written before
// digest_store.go.
//
//	M1b The table is the backstop for a double fire: two scheduled rows for
//	    one owner and UTC date cannot exist even if the claim logic is wrong.
//	    Manual (send-now) rows are not limited by the same index, so an hourly
//	    send-now is possible.
//	M1c Two simultaneous runs (a timer double-fire, an operator rerun racing
//	    the timer) send once.
//	S7  The no-repeat ledger keeps the highest revision sent and never
//	    lowers it.

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

func TestDigestTableRefusesASecondScheduledRowForTheDay(t *testing.T) {
	store, _ := newTestStore(t)
	insert := func(id, kind string) error {
		_, err := store.routingDB.Exec(`INSERT INTO digest_runs
			(id, owner_id, run_date, kind, status, reason, provider_message_id, story_ids, ranked_by, created_at, updated_at)
			VALUES (?, 'o1', '2026-10-20', ?, 'started', '', '', '[]', '', 'x', 'x')`, id, kind)
		return err
	}
	if err := insert("r1", "scheduled"); err != nil {
		t.Fatalf("first scheduled row: %v", err)
	}
	if err := insert("r2", "scheduled"); err == nil {
		t.Fatalf("a second scheduled row for one owner and date was accepted")
	}
	if err := insert("r3", "manual"); err != nil {
		t.Fatalf("manual row: %v", err)
	}
	if err := insert("r4", "manual"); err != nil {
		t.Fatalf("second manual row: %v", err)
	}
}

func TestDigestSimultaneousRunsSendOnce(t *testing.T) {
	d := newDigestHarness(t)
	d.subscribe()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if w := d.run(); w.Code != http.StatusOK {
				t.Errorf("run = %d %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	if d.resend.calls() != 1 {
		t.Fatalf("resend calls = %d after 4 simultaneous runs, want 1", d.resend.calls())
	}
}

func TestDigestSentLedgerKeepsTheHighestRevision(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	if err := store.RecordDigestSent(ctx, "o1", map[string]int{"p1": 3, "p2": 1}, digestTestNow); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordDigestSent(ctx, "o1", map[string]int{"p1": 2, "p2": 4}, digestTestNow); err != nil {
		t.Fatal(err)
	}
	got, err := store.DigestSentVersions(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if got["p1"] != 3 || got["p2"] != 4 {
		t.Fatalf("ledger = %v, want p1 3 and p2 4", got)
	}
	other, _ := store.DigestSentVersions(ctx, "o2")
	if len(other) != 0 {
		t.Fatalf("another owner sees %v", other)
	}
}
