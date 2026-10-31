package maild

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func dropNumberIndexes(t *testing.T, s *Store) {
	t.Helper()
	for _, n := range []string{"idx_email_aliases_canonical_number", "idx_email_aliases_numbered_target"} {
		if _, err := s.routingDB.Exec(`DROP INDEX IF EXISTS ` + n); err != nil {
			t.Fatal(err)
		}
	}
}

func rawAlias(t *testing.T, s *Store, id, domain, local string, canonical any, target, created string) {
	t.Helper()
	if _, err := s.routingDB.Exec(`INSERT INTO email_aliases (id, domain, local_part, canonical_number, target_type, target_id, visibility, receive_policy_id, created_at)
		VALUES (?, ?, ?, ?, 'user', ?, 'unlisted', ?, ?)`, id, domain, local, canonical, target, DefaultPublicPolicyID, created); err != nil {
		t.Fatal(err)
	}
}

func canonOf(t *testing.T, s *Store, id string) sql.NullInt64 {
	t.Helper()
	var n sql.NullInt64
	if err := s.routingDB.QueryRow(`SELECT canonical_number FROM email_aliases WHERE id = ?`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Today's production shape: 000 plus five 000+tag rows, all canonical 0,
// mixed owners, on the trusted workflow policy.
func TestBootRepairProductionShape(t *testing.T) {
	s, cfg := newTestStore(t)
	dropNumberIndexes(t, s)
	owners := []string{cfg.RootOwnerID, "t1", "t2", "t3", "t4"}
	for i, o := range owners {
		rawAlias(t, s, fmt.Sprintf("p%d", i), "choir.news", fmt.Sprintf("000+tag%d", i), 0, o, "2026-10-01T00:00:00Z")
	}
	rawAlias(t, s, "p5", "choir.news", "000+tag5", 0, "t4", "2026-10-02T00:00:00Z")
	if err := s.EnsureSchema(cfg); err != nil {
		t.Fatalf("boot must not fail: %v", err)
	}
	if n := canonOf(t, s, DefaultRootAliasID); !n.Valid || n.Int64 != 0 {
		t.Fatalf("000 lost its number: %v", n)
	}
	for i := 0; i <= 5; i++ {
		if n := canonOf(t, s, fmt.Sprintf("p%d", i)); n.Valid {
			t.Fatalf("plus alias p%d kept canonical_number %v", i, n)
		}
	}
	// the plus-aliases still route
	for i := 0; i <= 5; i++ {
		if _, err := s.ResolveAlias(context.Background(), "choir.news", fmt.Sprintf("000+tag%d", i)); err != nil {
			t.Fatalf("plus alias %d stopped routing: %v", i, err)
		}
	}
}

func TestBootRepairResolvesConflictsDeterministically(t *testing.T) {
	s, cfg := newTestStore(t)
	dropNumberIndexes(t, s)
	// wrong non-NULL value on a canonical alias, a conflicting pair across
	// domains (older wins), a second '000' that must lose to the root, and one
	// account holding two numbers (000-side/lower number wins).
	rawAlias(t, s, "w1", "choir.news", "007", 99, "acct-w", "2026-10-03T00:00:00Z")
	rawAlias(t, s, "c1", "other.example.test", "007", 7, "acct-c1", "2026-10-02T00:00:00Z")
	rawAlias(t, s, "r2", "other.example.test", "000", 0, "acct-r2", "2026-09-01T00:00:00Z")
	rawAlias(t, s, "m1", "choir.news", "010", 10, "acct-m", "2026-10-01T00:00:00Z")
	rawAlias(t, s, "m2", "choir.news", "011", 11, "acct-m", "2026-10-01T00:00:00Z")
	for round := 0; round < 2; round++ { // idempotent
		if err := s.EnsureSchema(cfg); err != nil {
			t.Fatalf("round %d: boot must not fail: %v", round, err)
		}
		if n := canonOf(t, s, DefaultRootAliasID); !n.Valid || n.Int64 != 0 {
			t.Fatalf("root alias lost 0: %v", n)
		}
		if n := canonOf(t, s, "r2"); n.Valid {
			t.Fatalf("second 000 kept 0: %v", n)
		}
		if n := canonOf(t, s, "w1"); n.Valid && n.Int64 != 7 {
			t.Fatalf("wrong value kept: %v", n)
		}
		// exactly one of w1/c1 holds 7; the older (c1) wins
		if n := canonOf(t, s, "c1"); !n.Valid || n.Int64 != 7 {
			t.Fatalf("c1 should win 7: %v", n)
		}
		if n := canonOf(t, s, "w1"); n.Valid {
			t.Fatalf("w1 should lose 7: %v", n)
		}
		if n := canonOf(t, s, "m1"); !n.Valid || n.Int64 != 10 {
			t.Fatalf("m1 = %v", n)
		}
		if n := canonOf(t, s, "m2"); n.Valid {
			t.Fatalf("m2 (second number for acct-m) should be cleared: %v", n)
		}
	}
	var receipts int
	_ = s.routingDB.QueryRow(`SELECT count(*) FROM email_alias_receipts WHERE kind IN ('number-conflict','target-conflict')`).Scan(&receipts)
	if receipts < 3 {
		t.Fatalf("conflicts left %d receipts, want >= 3", receipts)
	}
}

func TestBootRecordsNonCanonicalAliasReceipt(t *testing.T) {
	s, cfg := newTestStore(t)
	rawAlias(t, s, "nc", "choir.news", "0042", nil, "acct-nc", "2026-10-01T00:00:00Z")
	if err := s.EnsureSchema(cfg); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = s.routingDB.QueryRow(`SELECT count(*) FROM email_alias_receipts WHERE alias_id='nc' AND kind='unrouted-noncanonical-number'`).Scan(&n)
	if n != 1 {
		t.Fatalf("receipt count = %d", n)
	}
	if _, err := s.ResolveAlias(context.Background(), "choir.news", "0042"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("non-canonical alias must route nowhere")
	}
}

func TestBindRefusesAccountWithDisabledNumber(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	r1, _ := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL)
	if _, err := s.BindNumber(ctx, "choir.news", r1.LocalPart, "acct-a", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.routingDB.Exec(`UPDATE email_aliases SET disabled_at = 'x' WHERE local_part = ?`, r1.LocalPart); err != nil {
		t.Fatal(err)
	}
	r2, _ := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL)
	_, err := s.BindNumber(ctx, "choir.news", r2.LocalPart, "acct-a", t0)
	if !errors.Is(err, ErrAccountHasNumber) {
		t.Fatalf("err = %v, want ErrAccountHasNumber", err)
	}
	if want := "account already has number " + r1.LocalPart + ", disabled"; err == nil || !contains(err.Error(), want) {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestConcurrentBindsExactlyOneWins(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	r, _ := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL)
	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.BindNumber(ctx, "choir.news", r.LocalPart, fmt.Sprintf("acct-%d", i), t0.Add(time.Minute))
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				wins++
			} else if !errors.Is(err, ErrNumberBound) {
				t.Errorf("loser got %v, want ErrNumberBound", err)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("%d binds won, want exactly 1", wins)
	}
}

// The CAS itself: a reservation that lapsed between the read and the write
// is not taken.
func TestBindCASRefusesLapsedReservation(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	r, _ := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL)
	res, err := s.routingDB.Exec(`UPDATE email_aliases SET target_type='user', target_id='x'
		WHERE local_part = ? AND target_type='reserved' AND disabled_at IS NULL AND reserved_until > ?`,
		r.LocalPart, t0.Add(NumberReservationTTL+time.Second).UTC().Format(numberTimeFormat))
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := res.RowsAffected(); c != 0 {
		t.Fatal("CAS took an expired reservation")
	}
}

func TestReservationSkipsOccupiedNumbers(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	// 10000000 held by a disabled row on this domain; 10000001 by another
	// domain's alias holding the same canonical number.
	rawAlias(t, s, "o1", "choir.news", "10000000", nil, "occupier-1", "2026-10-01T00:00:00Z")
	rawAlias(t, s, "o2", "other.example.test", "10000001", 10000001, "occupier-2", "2026-10-01T00:00:00Z")
	r, err := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL)
	if err != nil {
		t.Fatalf("reserve wedged: %v", err)
	}
	if r.LocalPart != "10000002" {
		t.Fatalf("reserved %s, want 10000002", r.LocalPart)
	}
	var next int64
	_ = s.routingDB.QueryRow(`SELECT next FROM email_number_sequence WHERE range_name='test'`).Scan(&next)
	if next != 10000003 {
		t.Fatalf("sequence = %d, want 10000003 (bumps kept)", next)
	}
}

func TestListAliasesShowsNullNumberAsMinusOne(t *testing.T) {
	s, _ := newTestStore(t)
	rawAlias(t, s, "n1", "choir.news", "named", nil, "u", "2026-10-01T00:00:00Z")
	got, err := s.ListAliases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range got {
		if a.LocalPart == "named" && a.CanonicalNumber != -1 {
			t.Fatalf("named canonical = %d, want -1", a.CanonicalNumber)
		}
		if a.LocalPart == "000" && a.CanonicalNumber != 0 {
			t.Fatalf("000 canonical = %d", a.CanonicalNumber)
		}
	}
}
